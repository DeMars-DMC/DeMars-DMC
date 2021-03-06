package node

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	amino "github.com/tendermint/go-amino"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	cmn "github.com/Demars-DMC/Demars-DMC/libs/common"
	dbm "github.com/Demars-DMC/Demars-DMC/libs/db"
	"github.com/Demars-DMC/Demars-DMC/libs/log"

	bc "github.com/Demars-DMC/Demars-DMC/blockchain"
	cfg "github.com/Demars-DMC/Demars-DMC/config"
	cs "github.com/Demars-DMC/Demars-DMC/consensus"
	"github.com/Demars-DMC/Demars-DMC/crypto"
	mempl "github.com/Demars-DMC/Demars-DMC/mempool"
	"github.com/Demars-DMC/Demars-DMC/p2p"
	"github.com/Demars-DMC/Demars-DMC/privval"
	"github.com/Demars-DMC/Demars-DMC/proxy"
	rpccore "github.com/Demars-DMC/Demars-DMC/rpc/core"
	ctypes "github.com/Demars-DMC/Demars-DMC/rpc/core/types"
	grpccore "github.com/Demars-DMC/Demars-DMC/rpc/grpc"
	"github.com/Demars-DMC/Demars-DMC/rpc/lib"
	"github.com/Demars-DMC/Demars-DMC/rpc/lib/server"
	sm "github.com/Demars-DMC/Demars-DMC/state"
	"github.com/Demars-DMC/Demars-DMC/state/txindex"
	"github.com/Demars-DMC/Demars-DMC/state/txindex/null"
	"github.com/Demars-DMC/Demars-DMC/types"
	"github.com/Demars-DMC/Demars-DMC/version"
	"github.com/Demars-DMC/Demars-DMC/p2p/kademlia"
	abci "github.com/Demars-DMC/Demars-DMC/abci/types"

	_ "net/http/pprof"
	"github.com/Demars-DMC/Demars-DMC/state/txindex/kv"
)

//------------------------------------------------------------------------------

// DBContext specifies config information for loading a new DB.
type DBContext struct {
	ID     string
	Config *cfg.Config
}

// DBProvider takes a DBContext and returns an instantiated DB.
type DBProvider func(*DBContext) (dbm.DB, error)

// DefaultDBProvider returns a database using the DBBackend and DBDir
// specified in the ctx.Config.
func DefaultDBProvider(ctx *DBContext) (dbm.DB, error) {
	dbType := dbm.DBBackendType(ctx.Config.DBBackend)
	return dbm.NewDB(ctx.ID, dbType, ctx.Config.DBDir()), nil
}

// GenesisDocProvider returns a GenesisDoc.
// It allows the GenesisDoc to be pulled from sources other than the
// filesystem, for instance from a distributed key-value store cluster.
type GenesisDocProvider func() (*types.GenesisDoc, error)

// DefaultGenesisDocProviderFunc returns a GenesisDocProvider that loads
// the GenesisDoc from the config.GenesisFile() on the filesystem.
func DefaultGenesisDocProviderFunc(config *cfg.Config) GenesisDocProvider {
	return func() (*types.GenesisDoc, error) {
		return types.GenesisDocFromFile(config.GenesisFile())
	}
}

// NodeProvider takes a config and a logger and returns a ready to go Node.
type NodeProvider func(*cfg.Config, log.Logger) (*Node, error)

// DefaultNewNode returns a Demars-DMC node with default settings for the
// PrivValidator, ClientCreator, GenesisDoc, and DBProvider.
// It implements NodeProvider.
func DefaultNewNode(config *cfg.Config, logger log.Logger) (*Node, error) {
	return NewNode(config,
		privval.LoadOrGenFilePV(config.PrivValidatorFile()),
		proxy.DefaultClientCreator(config.ProxyApp, config.ABCI, config.DBDir()),
		DefaultGenesisDocProviderFunc(config),
		DefaultDBProvider,
		DefaultMetricsProvider,
		logger,
	)
}

// MetricsProvider returns a consensus, p2p and mempool Metrics.
type MetricsProvider func() (*cs.Metrics, *p2p.Metrics, *mempl.Metrics)

// DefaultMetricsProvider returns consensus, p2p and mempool Metrics build
// using Prometheus client library.
func DefaultMetricsProvider() (*cs.Metrics, *p2p.Metrics, *mempl.Metrics) {
	return cs.PrometheusMetrics(), p2p.PrometheusMetrics(), mempl.PrometheusMetrics()
}

// NopMetricsProvider returns consensus, p2p and mempool Metrics as no-op.
func NopMetricsProvider() (*cs.Metrics, *p2p.Metrics, *mempl.Metrics) {
	return cs.NopMetrics(), p2p.NopMetrics(), mempl.NopMetrics()
}

//------------------------------------------------------------------------------

// Node is the highest level interface to a full Demars-DMC node.
// It includes all configuration information and running services.
type Node struct {
	cmn.BaseService

	// config
	config        *cfg.Config
	genesisDoc    *types.GenesisDoc   // initial validator set
	privValidator types.PrivValidator // local node's validator key

	// network
	sw       *p2p.Switch  // p2p connections

	// services
	eventBus         *types.EventBus // pub/sub for services
	stateDB          dbm.DB
	blockStore       *bc.BlockStore         // store the blockchain to disk
	bcReactor        *bc.BlockchainReactor  // for fast-syncing
	mempoolReactor   *mempl.MempoolReactor  // for gossipping transactions
	consensusState   *cs.ConsensusState     // latest consensus state
	consensusReactor *cs.ConsensusReactor   // for participating in the consensus
	proxyApp         proxy.AppConns         // connection to the application
	rpcListeners     []net.Listener         // rpc servers
	txIndexer        txindex.TxIndexer
	indexerService   *txindex.IndexerService
	prometheusSrv    *http.Server
}

// NewNode returns a new, ready to go, Demars-DMC Node.
func NewNode(config *cfg.Config,
	privValidator types.PrivValidator,
	clientCreator proxy.ClientCreator,
	genesisDocProvider GenesisDocProvider,
	dbProvider DBProvider,
	metricsProvider MetricsProvider,
	logger log.Logger) (*Node, error) {

	// Get BlockStore
	blockStoreDB, err := dbProvider(&DBContext{"blockstore", config})
	if err != nil {
		return nil, err
	}
	blockStore := bc.NewBlockStore(blockStoreDB)

	// Get State
	stateDB, err := dbProvider(&DBContext{"state", config})
	if err != nil {
		return nil, err
	}

	// Get genesis doc
	// TODO: move to state package?
	genDoc, err := loadGenesisDoc(stateDB)
	if err != nil {
		genDoc, err = genesisDocProvider()
		if err != nil {
			return nil, err
		}
		// save genesis doc to prevent a certain class of user errors (e.g. when it
		// was changed, accidentally or not). Also good for audit trail.
		saveGenesisDoc(stateDB, genDoc)
	}

	state, err := sm.LoadStateFromDBOrGenesisDoc(stateDB, genDoc)
	if err != nil {
		return nil, err
	}

	// Create the proxyApp, which manages connections (consensus, mempool, query)
	// and sync Demars-DMC and the app by performing a handshake
	// and replaying any necessary blocks
	consensusLogger := logger.With("module", "consensus")
	handshaker := cs.NewHandshaker(stateDB, state, blockStore, genDoc)
	handshaker.SetLogger(consensusLogger)
	proxyApp := proxy.NewAppConns(clientCreator, handshaker)
	proxyApp.SetLogger(logger.With("module", "proxy"))
	if err := proxyApp.Start(); err != nil {
		return nil, fmt.Errorf("Error starting proxy app connections: %v", err)
	}

	// reload the state (it may have been updated by the handshake)
	state = sm.LoadState(stateDB)

	// If an address is provided, listen on the socket for a
	// connection from an external signing process.
	if config.PrivValidatorListenAddr != "" {
		var (
			// TODO: persist this key so external signer
			// can actually authenticate us
			privKey = crypto.GenPrivKeyEd25519()
			pvsc    = privval.NewSocketPV(
				logger.With("module", "privval"),
				config.PrivValidatorListenAddr,
				privKey,
			)
		)

		if err := pvsc.Start(); err != nil {
			return nil, fmt.Errorf("Error starting private validator client: %v", err)
		}

		privValidator = pvsc
	}

	// Decide whether to fast-sync or not
	// We don't fast-sync when the only validator is us.
//	fastSync := config.FastSync
//	if state.Validators.Size() == 1 {
//		addr, _ := state.Validators.GetByIndex(0)
//		if bytes.Equal(privValidator.GetAddress(), addr) {
//			fastSync = false
//		}
//	}
	fastSync := false

	// Log whether this node is a validator or an observer
	if state.Validators.HasAddress(privValidator.GetAddress()) {
		consensusLogger.Info("This node is a validator", "addr",
			privValidator.GetAddress(), "pubKey", privValidator.GetPubKey())
	} else {
		consensusLogger.Info("This node is not a validator", "addr",
			privValidator.GetAddress(), "pubKey", privValidator.GetPubKey())
	}

	// metrics
	var (
		csMetrics    *cs.Metrics
		p2pMetrics   *p2p.Metrics
		memplMetrics *mempl.Metrics
	)
	if config.Instrumentation.Prometheus {
		csMetrics, p2pMetrics, memplMetrics = metricsProvider()
	} else {
		csMetrics, p2pMetrics, memplMetrics = NopMetricsProvider()
	}

	// Make MempoolReactor
	mempoolLogger := logger.With("module", "mempool")
	mempool := mempl.NewMempool(
		config.Mempool,
		proxyApp.Mempool(),
		state.LastBlockHeight,
		mempl.WithMetrics(memplMetrics),
	)
	mempool.SetLogger(mempoolLogger)
	mempool.InitWAL() // no need to have the mempool wal during tests
	mempoolReactor := mempl.NewMempoolReactor(config.Mempool, mempool)
	mempoolReactor.SetLogger(mempoolLogger)

	blockExecLogger := logger.With("module", "state")
	// make block executor for consensus and blockchain reactors to execute blocks
	blockExec := sm.NewBlockExecutor(stateDB, blockExecLogger, proxyApp.Consensus(), mempool)

	// Make BlockchainReactor
	bcReactor := bc.NewBlockchainReactor(state.Copy(), blockExec, blockStore, fastSync)
	bcReactor.SetLogger(logger.With("module", "blockchain"))

	mempool.BCReactor = bcReactor
	if config.Consensus.WaitForTxs() {
		mempool.EnableTxsAvailable()
	}

	// Make ConsensusReactor
	consensusState := cs.NewConsensusState(
		config.Consensus,
		state.Copy(),
		blockExec,
		blockStore,
		mempool,
		cs.WithMetrics(csMetrics),
	)
	consensusState.SetLogger(consensusLogger)
	if privValidator != nil {
		consensusState.SetPrivValidator(privValidator)
	}
	consensusReactor := cs.NewConsensusReactor(consensusState, fastSync)
	consensusReactor.SetLogger(consensusLogger)

	p2pLogger := logger.With("module", "p2p")

	sw := p2p.NewSwitch(config.P2P, p2p.WithMetrics(p2pMetrics))
	sw.SetLogger(p2pLogger)
	sw.AddReactor("MEMPOOL", mempoolReactor)
	sw.AddReactor("BLOCKCHAIN", bcReactor)
	sw.AddReactor("CONSENSUS", consensusReactor)

	// Start the KademliaReactor
	seedAddresses := strings.Split(config.P2P.Seeds, ",")
	netAddrs, _ := p2p.NewNetAddressStrings(seedAddresses)
	p2pLogger.Debug("Seed net addresses", "count", len(netAddrs))
	
	kademliaReactor := kademlia.NewKademliaReactor(netAddrs)
	kademliaReactor.SetLogger(p2pLogger)
	sw.AddReactor("KADEMLIA", kademliaReactor)

	// Filter peers by addr or pubkey with an ABCI query.
	// If the query return code is OK, add peer.
	// XXX: Query format subject to change
	if config.FilterPeers {
		// NOTE: addr is ip:port
		sw.SetAddrFilter(func(addr net.Addr) error {
			resQuery, err := proxyApp.Query().QuerySync(abci.RequestQuery{Path: cmn.Fmt("/p2p/filter/addr/%s",
				addr.String())})
			if err != nil {
				return err
			}
			if resQuery.IsErr() {
				return fmt.Errorf("Error querying abci app: %v", resQuery)
			}
			return nil
		})
		sw.SetIDFilter(func(id p2p.NodeID) error {
			resQuery, err := proxyApp.Query().QuerySync(abci.RequestQuery{Path: cmn.Fmt("/p2p/filter/id/%s",
				id)})
			if err != nil {
				return err
			}
			if resQuery.IsErr() {
				return fmt.Errorf("Error querying abci app: %v", resQuery)
			}
			return nil
		})
	}

	eventBus := types.NewEventBus()
	eventBus.SetLogger(logger.With("module", "events"))

	// services which will be publishing and/or subscribing for messages (events)
	// consensusReactor will set it on consensusState and blockExecutor
	consensusReactor.SetEventBus(eventBus)

	// Transaction indexing
	var txIndexer txindex.TxIndexer
	switch config.TxIndex.Indexer {
	case "kv":
		store, err := dbProvider(&DBContext{"tx_index", config})
		if err != nil {
			return nil, err
		}
		if config.TxIndex.IndexTags != "" {
			txIndexer = kv.NewTxIndex(store, kv.IndexTags(cmn.SplitAndTrim(config.TxIndex.IndexTags, ",",
				" ")))
		} else if config.TxIndex.IndexAllTags {
			txIndexer = kv.NewTxIndex(store, kv.IndexAllTags())
		} else {
			txIndexer = kv.NewTxIndex(store)
		}
	default:
		txIndexer = &null.TxIndex{}
	}

	indexerService := txindex.NewIndexerService(txIndexer, eventBus)
	indexerService.SetLogger(logger.With("module", "txindex"))

	// run the profile server
	profileHost := config.ProfListenAddress
	if profileHost != "" {
		go func() {
			logger.Error("Profile server", "err", http.ListenAndServe(profileHost, nil))
		}()
	}

	node := &Node{
		config:        config,
		genesisDoc:    genDoc,
		privValidator: privValidator,

		sw:       sw,

		stateDB:          stateDB,
		blockStore:       blockStore,
		bcReactor:        bcReactor,
		mempoolReactor:   mempoolReactor,
		consensusState:   consensusState,
		consensusReactor: consensusReactor,
		proxyApp:         proxyApp,
		txIndexer:        txIndexer,
		indexerService:   indexerService,
		eventBus:         eventBus,
	}
	node.BaseService = *cmn.NewBaseService(logger, "Node", node)
	return node, nil
}

// OnStart starts the Node. It implements cmn.Service.
func (n *Node) OnStart() error {
	err := n.eventBus.Start()
	if err != nil {
		return err
	}

	// Create & add listener
	l := p2p.NewDefaultListener(
		n.config.P2P.ListenAddress,
		n.config.P2P.ExternalAddress,
		n.config.P2P.UPNP,
		n.Logger.With("module", "p2p"))
	n.sw.AddListener(l)

	// Generate node PrivKey
	// TODO: pass in like privValidator
	nodeKey, err := p2p.LoadOrGenNodeKey(n.config.NodeKeyFile())
	if err != nil {
		return err
	}
	n.Logger.Info("P2P Node ID", "ID", nodeKey.ID(), "file", n.config.NodeKeyFile())

	nodeInfo := n.makeNodeInfo(nodeKey.ID())
	n.sw.SetNodeInfo(nodeInfo)
	n.sw.SetNodeKey(nodeKey)

	// Start the RPC server before the P2P server
	// so we can eg. receive txs for the first block
	if n.config.RPC.ListenAddress != "" {
		listeners, err := n.startRPC()
		if err != nil {
			return err
		}
		n.rpcListeners = listeners
	}

	if n.config.Instrumentation.Prometheus {
		n.prometheusSrv = n.startPrometheusServer(n.config.Instrumentation.PrometheusListenAddr)
	}

	// Start the switch (the P2P server).
	err = n.sw.Start()
	if err != nil {
		return err
	}

	// start tx indexer
	return n.indexerService.Start()
}

// OnStop stops the Node. It implements cmn.Service.
func (n *Node) OnStop() {
	n.BaseService.OnStop()

	n.Logger.Info("Stopping Node")
	// TODO: gracefully disconnect from peers.
	n.sw.Stop()

	for _, l := range n.rpcListeners {
		n.Logger.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			n.Logger.Error("Error closing listener", "listener", l, "err", err)
		}
	}

	n.eventBus.Stop()
	n.indexerService.Stop()

	if pvsc, ok := n.privValidator.(*privval.SocketPV); ok {
		if err := pvsc.Stop(); err != nil {
			n.Logger.Error("Error stopping priv validator socket client", "err", err)
		}
	}

	if n.prometheusSrv != nil {
		if err := n.prometheusSrv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			n.Logger.Error("Prometheus HTTP server ShutdoRwn", "err", err)
		}
	}
}

// RunForever waits for an interrupt signal and stops the node.
func (n *Node) RunForever() {
	// Sleep forever and then...
	cmn.TrapSignal(func() {
		n.Stop()
	})
}

// AddListener adds a listener to accept inbound peer connections.
// It should be called before starting the Node.
// The first listener is the primary listener (in NodeInfo)
func (n *Node) AddListener(l p2p.Listener) {
	n.sw.AddListener(l)
}

// ConfigureRPC sets all variables in rpccore so they will serve
// rpc calls from this node
func (n *Node) ConfigureRPC() {
	rpccore.SetStateDB(n.stateDB)
	rpccore.SetBlockStore(n.blockStore)
	rpccore.SetConsensusState(n.consensusState)
	rpccore.SetMempool(n.mempoolReactor.Mempool)
	rpccore.SetSwitch(n.sw)
	rpccore.SetPubKey(n.privValidator.GetPubKey())
	rpccore.SetGenesisDoc(n.genesisDoc)
	rpccore.SetProxyAppQuery(n.proxyApp.Query())
	rpccore.SetTxIndexer(n.txIndexer)
	rpccore.SetConsensusReactor(n.consensusReactor)
	rpccore.SetEventBus(n.eventBus)
	rpccore.SetLogger(n.Logger.With("module", "rpc"))
}

func (n *Node) startRPC() ([]net.Listener, error) {
	n.ConfigureRPC()
	listenAddrs := cmn.SplitAndTrim(n.config.RPC.ListenAddress, ",", " ")
	coreCodec := amino.NewCodec()
	ctypes.RegisterAmino(coreCodec)

	if n.config.RPC.Unsafe {
		rpccore.AddUnsafeRoutes()
	}

	// we may expose the rpc over both a unix and tcp socket
	listeners := make([]net.Listener, len(listenAddrs))
	for i, listenAddr := range listenAddrs {
		mux := http.NewServeMux()
		rpcLogger := n.Logger.With("module", "rpc-server")
		wm := rpcserver.NewWebsocketManager(rpccore.Routes, coreCodec, rpcserver.EventSubscriber(n.eventBus))
		wm.SetLogger(rpcLogger.With("protocol", "websocket"))
		mux.HandleFunc("/websocket", wm.WebsocketHandler)
		rpcserver.RegisterRPCFuncs(mux, rpccore.Routes, coreCodec, rpcLogger)
		listener, err := rpcserver.StartHTTPServer(
			listenAddr,
			mux,
			rpcLogger,
			rpcserver.Config{MaxOpenConnections: n.config.RPC.MaxOpenConnections},
		)
		if err != nil {
			return nil, err
		}
		listeners[i] = listener
	}

	// we expose a simplified api over grpc for convenience to app devs
	grpcListenAddr := n.config.RPC.GRPCListenAddress
	if grpcListenAddr != "" {
		listener, err := grpccore.StartGRPCServer(
			grpcListenAddr,
			grpccore.Config{
				MaxOpenConnections: n.config.RPC.GRPCMaxOpenConnections,
			},
		)
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, listener)
	}

	return listeners, nil
}

// startPrometheusServer starts a Prometheus HTTP server, listening for metrics
// collectors on addr.
func (n *Node) startPrometheusServer(addr string) *http.Server {
	srv := &http.Server{
		Addr:    addr,
		Handler: promhttp.Handler(),
	}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// Error starting or closing listener:
			n.Logger.Error("Prometheus HTTP server ListenAndServe", "err", err)
		}
	}()
	return srv
}

// Switch returns the Node's Switch.
func (n *Node) Switch() *p2p.Switch {
	return n.sw
}

// BlockStore returns the Node's BlockStore.
func (n *Node) BlockStore() *bc.BlockStore {
	return n.blockStore
}

// ConsensusState returns the Node's ConsensusState.
func (n *Node) ConsensusState() *cs.ConsensusState {
	return n.consensusState
}

// ConsensusReactor returns the Node's ConsensusReactor.
func (n *Node) ConsensusReactor() *cs.ConsensusReactor {
	return n.consensusReactor
}

// MempoolReactor returns the Node's MempoolReactor.
func (n *Node) MempoolReactor() *mempl.MempoolReactor {
	return n.mempoolReactor
}

// EventBus returns the Node's EventBus.
func (n *Node) EventBus() *types.EventBus {
	return n.eventBus
}

// PrivValidator returns the Node's PrivValidator.
// XXX: for convenience only!
func (n *Node) PrivValidator() types.PrivValidator {
	return n.privValidator
}

// GenesisDoc returns the Node's GenesisDoc.
func (n *Node) GenesisDoc() *types.GenesisDoc {
	return n.genesisDoc
}

// ProxyApp returns the Node's AppConns, representing its connections to the ABCI application.
func (n *Node) ProxyApp() proxy.AppConns {
	return n.proxyApp
}

func (n *Node) makeNodeInfo(nodeID p2p.NodeID) p2p.NodeInfo {
	txIndexerStatus := "on"
	if _, ok := n.txIndexer.(*null.TxIndex); ok {
		txIndexerStatus = "off"
	}
	nodeInfo := p2p.NodeInfo{
		ID:      nodeID,
		Network: n.genesisDoc.ChainID,
		Version: version.Version,
		Channels: []byte{
			bc.BlockchainChannel,
			cs.StateChannel, cs.DataChannel, cs.VoteChannel, cs.VoteSetBitsChannel,
			mempl.MempoolChannel,
		},
		Moniker: n.config.Moniker,
		Other: []string{
			cmn.Fmt("amino_version=%v", amino.Version),
			cmn.Fmt("p2p_version=%v", p2p.Version),
			cmn.Fmt("consensus_version=%v", cs.Version),
			cmn.Fmt("rpc_version=%v/%v", rpc.Version, rpccore.Version),
			cmn.Fmt("tx_index=%v", txIndexerStatus),
		},
	}

	if n.config.P2P.PexReactor {
		nodeInfo.Channels = append(nodeInfo.Channels, kademlia.KademliaChannel)
	}

	rpcListenAddr := n.config.RPC.ListenAddress
	nodeInfo.Other = append(nodeInfo.Other, cmn.Fmt("rpc_addr=%v", rpcListenAddr))

	if !n.sw.IsListening() {
		return nodeInfo
	}

	p2pListener := n.sw.Listeners()[0]
	p2pHost := p2pListener.ExternalAddressHost()
	p2pPort := p2pListener.ExternalAddress().Port
	nodeInfo.ListenAddr = cmn.Fmt("%v:%v", p2pHost, p2pPort)

	return nodeInfo
}

//------------------------------------------------------------------------------

// NodeInfo returns the Node's Info from the Switch.
func (n *Node) NodeInfo() p2p.NodeInfo {
	return n.sw.NodeInfo()
}

//------------------------------------------------------------------------------

var (
	genesisDocKey = []byte("genesisDoc")
)

// panics if failed to unmarshal bytes
func loadGenesisDoc(db dbm.DB) (*types.GenesisDoc, error) {
	bytes := db.Get(genesisDocKey)
	if len(bytes) == 0 {
		return nil, errors.New("Genesis doc not found")
	}
	var genDoc *types.GenesisDoc
	err := cdc.UnmarshalJSON(bytes, &genDoc)
	if err != nil {
		cmn.PanicCrisis(fmt.Sprintf("Failed to load genesis doc due to unmarshaling error: %v (bytes: %X)",
			err, bytes))
	}
	return genDoc, nil
}

// panics if failed to marshal the given genesis document
func saveGenesisDoc(db dbm.DB, genDoc *types.GenesisDoc) {
	bytes, err := cdc.MarshalJSON(genDoc)
	if err != nil {
		cmn.PanicCrisis(fmt.Sprintf("Failed to save genesis doc due to marshaling error: %v", err))
	}
	db.SetSync(genesisDocKey, bytes)
}
