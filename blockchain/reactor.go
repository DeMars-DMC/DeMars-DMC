package blockchain

import (
	"strconv"
	"fmt"
	"reflect"
	"time"

	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/p2p"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
	cmn "github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	// BlockchainChannel is a channel for blocks and status updates (`BlockStore` height)
	BlockchainChannel = byte(0x40)

	trySyncIntervalMS = 50
	checkBucketChainsSyncIntervalMS = 50

	// ask for best height every 10s
	statusUpdateIntervalSeconds = 10
	// check if we should switch to consensus reactor
	doneSyncingCheckIntervalSeconds = 1

	// NOTE: keep up to date with bcBlockResponseMessage
	bcBlockResponseMessagePrefixSize   = 4
	bcBlockResponseMessageFieldKeySize = 1
	maxMsgSize                         = types.MaxBlockSizeBytes +
		bcBlockResponseMessagePrefixSize +
		bcBlockResponseMessageFieldKeySize
)

type consensusReactor interface {
	// for when we switch from blockchain reactor and fast sync to
	// the consensus machine
	SwitchToConsensus(sm.State, int)
	IsProposer() bool
}

type peerError struct {
	err    error
	peerID p2p.NodeID
}

func (e peerError) Error() string {
	return fmt.Sprintf("error with peer %v: %s", e.peerID, e.err.Error())
}

// BlockchainReactor handles long-term catchup syncing.
type BlockchainReactor struct {
	p2p.BaseReactor

	// immutable
	initialState sm.State

	blockExec *sm.BlockExecutor
	store     *BlockStore
	pool      *BlockPool
	fastSync  bool
	claimToBeInProposerRegion bool
	claimToBeInProposerRegionBlock *types.Block

	requestsCh <-chan BlockRequest
	errorsCh   <-chan peerError
}

// NewBlockchainReactor returns new reactor instance.
func NewBlockchainReactor(state sm.State, blockExec *sm.BlockExecutor, store *BlockStore,
	fastSync bool) *BlockchainReactor {

	if state.LastBlockHeight != store.Height() {
		panic(fmt.Sprintf("state (%v) and store (%v) height mismatch", state.LastBlockHeight,
			store.Height()))
	}

	const capacity = 1000 // must be bigger than peers count
	requestsCh := make(chan BlockRequest, capacity)
	errorsCh := make(chan peerError, capacity) // so we don't block in #Receive#pool.AddBlock

	pool := NewBlockPool(
		store.Height()+1,
		requestsCh,
		errorsCh,
	)

	bcR := &BlockchainReactor{
		initialState: state,
		blockExec:    blockExec,
		store:        store,
		pool:         pool,
		fastSync:     fastSync,
		requestsCh:   requestsCh,
		errorsCh:     errorsCh,
		claimToBeInProposerRegion: false,
		claimToBeInProposerRegionBlock: nil,

	}
	bcR.BaseReactor = *p2p.NewBaseReactor("BlockchainReactor", bcR)
	return bcR
}

// SetLogger implements cmn.Service by setting the logger on reactor and pool.
func (bcR *BlockchainReactor) SetLogger(l log.Logger) {
	bcR.BaseService.Logger = l
	bcR.pool.Logger = l
}

// OnStart implements cmn.Service.
func (bcR *BlockchainReactor) OnStart() error {
	if err := bcR.BaseReactor.OnStart(); err != nil {
		return err
	}
	if bcR.fastSync {
		err := bcR.pool.Start()
		if err != nil {
			return err
		}
		go bcR.poolRoutine()
	}
	return nil
}

// OnStop implements cmn.Service.
func (bcR *BlockchainReactor) OnStop() {
	bcR.BaseReactor.OnStop()
	bcR.pool.Stop()
}

// GetChannels implements Reactor
func (bcR *BlockchainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  BlockchainChannel,
			Priority:            10,
			SendQueueCapacity:   1000,
			RecvBufferCapacity:  50 * 4096,
			RecvMessageCapacity: maxMsgSize,
		},
	}
}

// AddPeer implements Reactor by sending our state to peer.
func (bcR *BlockchainReactor) AddPeer(peer p2p.Peer) {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponseMessage{bcR.store.Height()})
	if !peer.Send(BlockchainChannel, msgBytes) {
		// doing nothing, will try later in `poolRoutine`
	}
	// peer is added to the pool once we receive the first
	// bcStatusResponseMessage from the peer and call pool.SetPeerHeight
}

// RemovePeer implements Reactor by removing peer from the pool.
func (bcR *BlockchainReactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	bcR.pool.RemovePeer(peer.ID())
}

// respondToPeer loads a block and sends it to the requesting peer,
// if we have it. Otherwise, we'll respond saying we don't have it.
// According to the Tendermint spec, if all nodes are honest,
// no node should be requesting for a block that's non-existent.
func (bcR *BlockchainReactor) respondToPeer(msg *bcBlockRequestMessage,
	src p2p.Peer) (queued bool) {

	block := bcR.store.LoadBlock(msg.Height)
	if block != nil {
		if (msg.Bucket == "") {
			msgBytes := cdc.MustMarshalBinaryBare(&bcBlockResponseMessage{Block: block})
			return src.TrySend(BlockchainChannel, msgBytes)
		} else {
			for _, txBucket := range block.Data.TxBuckets {
				if (txBucket.BucketId == msg.Bucket) {
					b := block.LastBucketHashes.BucketHashes[txBucket.BucketId]
					cdc.MustMarshalBinaryBare(&bcBlockResponseMessage{Bucket: &txBucket, LastBucketHash: &b})
				}
			}
		}
	}

	bcR.Logger.Info("Peer asking for a block we don't have", "src", src, "height", msg.Height)

	msgBytes := cdc.MustMarshalBinaryBare(&bcNoBlockResponseMessage{Height: msg.Height})
	return src.TrySend(BlockchainChannel, msgBytes)
}

// Receive implements Reactor by handling 4 types of messages (look below).
func (bcR *BlockchainReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	msg, err := DecodeMessage(msgBytes)
	if err != nil {
		bcR.Logger.Error("Error decoding message", "src", src, "chId", chID, "msg", msg, "err", err, "bytes", msgBytes)
		bcR.Switch.StopPeerForError(src, err)
		return
	}

	bcR.Logger.Debug("Receive", "src", src, "chID", chID, "msg", msg)

	switch msg := msg.(type) {
	case *bcBlockRequestMessage:
		bcR.Logger.Debug("Received block request message")
		if queued := bcR.respondToPeer(msg, src); !queued {
			// Unfortunately not queued since the queue is full.
		}
	case *bcBlockResponseMessage:
		bcR.Logger.Debug("Received block response message")
		if msg.Bucket != nil && msg.Bucket.BucketId != "" {
			// Got a bucket. The bucket height is in msg.Block.Height
			bcR.pool.AddBucket(src.ID(), msg.Bucket, msg.Block.Height, msg.Block.Commit, bcR.store)
		} else {
			// Got a block.
			segmentID, _ := strconv.ParseInt(bcR.Switch.NodeInfo().SegmentID(), 16, 64)
			bcR.claimToBeInProposerRegion = bcR.pool.AddBlock(src.ID(), msg.Block, len(msgBytes), segmentID)
			if bcR.claimToBeInProposerRegion {
				bcR.claimToBeInProposerRegionBlock = msg.Block
				bcR.fastSync = true
			}
		}

	case *bcStatusRequestMessage:
		bcR.Logger.Debug("Received status request message")
		// Send peer our state.
		msgBytes := cdc.MustMarshalBinaryBare(&bcStatusResponseMessage{bcR.store.Height()})
		queued := src.TrySend(BlockchainChannel, msgBytes)
		if !queued {
			// sorry
		}
	case *bcStatusResponseMessage:
		bcR.Logger.Debug("Received status response message")
		// Got a peer status. Unverified.
		bcR.pool.SetPeerHeight(src.ID(), msg.Height)
	default:
		bcR.Logger.Error(cmn.Fmt("Unknown message type %v", reflect.TypeOf(msg)))
	}
}

// Handle messages from the poolReactor telling the reactor what to do.
// NOTE: Don't sleep in the FOR_LOOP or otherwise slow it down!
// (Except for the SYNC_LOOP, which is the primary purpose and must be synchronous.)
func (bcR *BlockchainReactor) poolRoutine() {

	trySyncTicker := time.NewTicker(trySyncIntervalMS * time.Millisecond)
	statusUpdateTicker := time.NewTicker(statusUpdateIntervalSeconds * time.Second)
	doneSyncingTicker := time.NewTicker(doneSyncingCheckIntervalSeconds * time.Second)
	checkBucketChainsTicker := time.NewTicker(checkBucketChainsSyncIntervalMS * time.Millisecond)

	blocksSynced := 0

	chainID := bcR.initialState.ChainID
	state := bcR.initialState

	lastHundred := time.Now()
	lastRate := 0.0

FOR_LOOP:
	for {
		select {
		case request := <-bcR.requestsCh:
			peer := bcR.Switch.Peers().Get(request.PeerID)
			if peer == nil {
				continue FOR_LOOP // Peer has since been disconnected.
			}
			bucketID := ""
			if (request.BucketID != nil) {
				bucketID = *request.BucketID
			}
			msgBytes := cdc.MustMarshalBinaryBare(&bcBlockRequestMessage{request.Height, bucketID})
			queued := peer.TrySend(BlockchainChannel, msgBytes)
			if !queued {
				// We couldn't make the request, send-queue full.
				// The pool handles timeouts, just let it go.
				continue FOR_LOOP
			}
		case err := <-bcR.errorsCh:
			peer := bcR.Switch.Peers().Get(err.peerID)
			if peer != nil {
				bcR.Switch.StopPeerForError(peer, err)
			}
		case <-statusUpdateTicker.C:
			// ask for status updates
			go bcR.BroadcastStatusRequest() // nolint: errcheck
		case <-doneSyncingTicker.C:
			height, numPending, lenRequesters := bcR.pool.GetStatus()
			outbound, inbound, _ := bcR.Switch.NumPeers()
			bcR.Logger.Debug("Consensus ticker", "numPending", numPending, "total", lenRequesters,
				"outbound", outbound, "inbound", inbound)
			if bcR.pool.IsCaughtUp() {
				bcR.Logger.Info("Caught up with network", "height", height)
				bcR.fastSync = false

				// Verify if currently in claim to be in proposer region
				if bcR.claimToBeInProposerRegion {
					// Verify whether we have the UTXO bucket for the last block's proposer region
					LatestUTXOBlock := bcR.claimToBeInProposerRegionBlock.Height / 100
					LatestUTXOBlock = (LatestUTXOBlock * 100) + 1

					// FIXME edge cases not dealt with here for example previous block was jump block and block to
					// vote on is UTXO block
					previousBlockSegment := bcR.claimToBeInProposerRegionBlock.Height % 100
					previousBlockSegmentHex := fmt.Sprintf("%x", previousBlockSegment)
					block := bcR.store.LoadBlock(LatestUTXOBlock)
					if block == nil {
						// The block is not available, let's try the bucket
						bucket := bcR.store.LoadBucket(LatestUTXOBlock, previousBlockSegmentHex)
						if (bucket == nil) {
							// Download the bucket
							peer := bcR.pool.PickIncrAvailablePeerInSegment(LatestUTXOBlock, previousBlockSegmentHex)
							bcR.pool.sendRequestBucket(LatestUTXOBlock, peer.id, previousBlockSegmentHex)

							// Recheck later whether we have the required bucket
							continue FOR_LOOP
						}
					}

					// Here we either have the latest UTXO block or relevant bucket
					// TODO Verify proposer block before switching to consensus reactor

					// We have verified the proposer block here
					// If (1) we are a proposer, (2) we are supposed to propose a UTXO block
					// then we need to download all the blocks since the last UTXO before being able to propose
					// the UTXO block
					conR := bcR.Switch.Reactor("CONSENSUS").(consensusReactor)
					needMoreBlocks := false
					if conR.IsProposer() && bcR.claimToBeInProposerRegionBlock.Height % 100 == 0 {
						for i := LatestUTXOBlock; i < bcR.claimToBeInProposerRegionBlock.Height + 1; i++ {
							block := bcR.store.LoadBlock(LatestUTXOBlock)
							if block == nil {
								blockSegmentHex := fmt.Sprintf("%x", i)
								peer := bcR.pool.PickIncrAvailablePeerInSegment(i, blockSegmentHex)
								bcR.pool.sendRequest(i, peer.id)
								// Request block
								needMoreBlocks = true
							}	
						}
					}

					if needMoreBlocks {
						continue FOR_LOOP
					}


					bcR.blockExec.UpdateValidatorsSet(state)

					// Verify commit message on block (will need to read account balances from UTXO)
					// If valid, start consensus reactor
					conR.SwitchToConsensus(state, blocksSynced)
					bcR.claimToBeInProposerRegion = false
				}
			}
		case <-checkBucketChainsTicker.C:
			for bucketId, bucketChainRequester := range bcR.pool.BucketChainRequesters {
				if bucketChainRequester.GotAllBuckets {
					bcR.pool.BucketChainRequesters[bucketId] = nil

					for i := bucketChainRequester.Height; i >= (bucketChainRequester.Height / 100 * 100) + 1; i++ {
						// TODO validate buckets
					}

					for i := (bucketChainRequester.Height / 100 * 100) + 1; i < bucketChainRequester.Height; i++ {
						// Save and apply buckets
						bcR.store.SaveBucket(i, bucketChainRequester.Buckets[i].BucketId, bucketChainRequester.Buckets[i])
						bcR.blockExec.ApplyBucket(i, bucketChainRequester.Buckets[i])
					}
				}
			}
		case <-trySyncTicker.C: // chan time
			// This loop can be slow as long as it's doing syncing work.
			if !bcR.fastSync {
				continue FOR_LOOP
			}
		SYNC_LOOP:
			for i := 0; i < 10; i++ {
				// See if there are any blocks to sync.
				block := bcR.pool.PeekBlock()
				if block == nil {
					// We need both to sync the first block.
					break SYNC_LOOP
				}
				parts := block.MakePartSet(state.ConsensusParams.BlockPartSizeBytes)
				partsHeader := parts.Header()
				blockID := types.BlockID{block.Hash(), partsHeader}
				// Finally, verify the block
				// NOTE: we can probably make this more efficient, but note that calling
				// block.Hash() doesn't verify the tx contents, so MakePartSet() is
				// currently necessary.
				err := state.Validators.VerifyCommit(
					chainID, blockID, block.Height, block.Commit)
				if err != nil {
					bcR.Logger.Error("Error in validation", "err", err)
					peerID := bcR.pool.RedoRequest(block.Height)
					peer := bcR.Switch.Peers().Get(peerID)
					if peer != nil {
						bcR.Switch.StopPeerForError(peer, fmt.Errorf("BlockchainReactor validation error: %v", err))
					}
					break SYNC_LOOP
				} else {
					bcR.pool.PopRequest()

					// TODO: batch saves so we dont persist to disk every block
					bcR.store.SaveBlock(block, parts, block.Commit)

					// TODO: same thing for app - but we would need a way to
					// get the hash without persisting the state
					var err error
					state, err = bcR.blockExec.ApplyBlock(state, blockID, block)
					// TODO This is bad, are we zombie?
					if err != nil {
						cmn.PanicQ(cmn.Fmt("Failed to process committed block (%d:%X): %v",
							block.Height, block.Hash(), err))
					}
					blocksSynced++

					if blocksSynced%100 == 0 {
						lastRate = 0.9*lastRate + 0.1*(100/time.Since(lastHundred).Seconds())
						bcR.Logger.Info("Fast Sync Rate", "height", bcR.pool.height,
							"max_peer_height", bcR.pool.MaxPeerHeight(), "blocks/s", lastRate)
						lastHundred = time.Now()
					}
				}
			}
			continue FOR_LOOP
		case <-bcR.Quit():
			break FOR_LOOP
		}
	}
}

// BroadcastStatusRequest broadcasts `BlockStore` height.
func (bcR *BlockchainReactor) BroadcastStatusRequest() error {
	msgBytes := cdc.MustMarshalBinaryBare(&bcStatusRequestMessage{bcR.store.Height()})
	bcR.Switch.Broadcast(BlockchainChannel, msgBytes)
	return nil
}

func (bcR *BlockchainReactor) GetPool() *BlockPool {
	return bcR.pool
}

//-----------------------------------------------------------------------------
// Messages

// BlockchainMessage is a generic message for this reactor.
type BlockchainMessage interface{}

func RegisterBlockchainMessages(cdc *amino.Codec) {
	cdc.RegisterInterface((*BlockchainMessage)(nil), nil)
	cdc.RegisterConcrete(&bcBlockRequestMessage{}, "tendermint/mempool/BlockRequest", nil)
	cdc.RegisterConcrete(&bcBlockResponseMessage{}, "tendermint/mempool/BlockResponse", nil)
	cdc.RegisterConcrete(&bcNoBlockResponseMessage{}, "tendermint/mempool/NoBlockResponse", nil)
	cdc.RegisterConcrete(&bcStatusResponseMessage{}, "tendermint/mempool/StatusResponse", nil)
	cdc.RegisterConcrete(&bcStatusRequestMessage{}, "tendermint/mempool/StatusRequest", nil)
}

// DecodeMessage decodes BlockchainMessage.
// TODO: ensure that bz is completely read.
func DecodeMessage(bz []byte) (msg BlockchainMessage, err error) {
	if len(bz) > maxMsgSize {
		return msg, fmt.Errorf("Msg exceeds max size (%d > %d)",
			len(bz), maxMsgSize)
	}
	err = cdc.UnmarshalBinaryBare(bz, &msg)
	if err != nil {
		err = cmn.ErrorWrap(err, "DecodeMessage() had bytes left over")
	}
	return
}

//-------------------------------------

type bcBlockRequestMessage struct {
	Height int64
	Bucket string
}

func (m *bcBlockRequestMessage) String() string {
	return cmn.Fmt("[bcBlockRequestMessage %v %s %s]", m.Height, m.Bucket)
}

type bcNoBlockResponseMessage struct {
	Height int64
}

func (brm *bcNoBlockResponseMessage) String() string {
	return cmn.Fmt("[bcNoBlockResponseMessage %d]", brm.Height)
}

//-------------------------------------

type bcBlockResponseMessage struct {
	Block *types.Block
	Bucket *types.TxBucket
	LastBucketHash *types.BucketHash
}

func (m *bcBlockResponseMessage) String() string {
	return cmn.Fmt("[bcBlockResponseMessage %v]", m.Block.Height)
}

//-------------------------------------

type bcStatusRequestMessage struct {
	Height int64
}

func (m *bcStatusRequestMessage) String() string {
	return cmn.Fmt("[bcStatusRequestMessage %v]", m.Height)
}

//-------------------------------------

type bcStatusResponseMessage struct {
	Height int64
}

func (m *bcStatusResponseMessage) String() string {
	return cmn.Fmt("[bcStatusResponseMessage %v]", m.Height)
}
