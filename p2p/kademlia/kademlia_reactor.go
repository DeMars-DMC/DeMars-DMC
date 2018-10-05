package kademlia

import (
	"github.com/Demars-DMC/Demars-DMC/p2p"
	p2pconn "github.com/Demars-DMC/Demars-DMC/p2p/conn"
	"fmt"
	"time"
	"bytes"
	"container/list"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/netutil"
	"github.com/ethereum/go-ethereum/rlp"
	"sync"
	"errors"
	"net"
)

// Errors
var (
	errPacketTooSmall   = errors.New("too small")
	errBadHash          = errors.New("bad hash")
	errExpired          = errors.New("expired")
	errUnsolicitedReply = errors.New("unsolicited reply")
	errUnknownNode      = errors.New("unknown node")
	errTimeout          = errors.New("RPC timeout")
	errClockWarp        = errors.New("reply deadline too far in the future")
	errClosed           = errors.New("socket closed")
)

const (
	// KademliaChannel is a channel for Kademlia messages
	KademliaChannel = byte(0x00)

	macSize  = 256 / 8
	sigSize  = 520 / 8
	headSize = macSize + sigSize // space of packet frame data

	maxAttemptsToDial = 16 // ~ 35h in total (last attempt - 18h)

	respTimeout = 500 * time.Millisecond
	expiration  = 20 * time.Second

	ntpFailureThreshold = 32               // Continuous timeouts after which to check NTP
	ntpWarningCooldown  = 10 * time.Minute // Minimum amount of time to pass before repeating NTP warning
	driftThreshold      = 10 * time.Second // Allowed clock drift before warning user
	nodeDBPath 			= "~/.Demars-DMC/db"
)

// RPC packet types
const (
	pingPacket = iota + 1 // zero is 'reserved'
	pongPacket
	findnodePacket
	neighborsPacket
)

var (
	// Neighbors replies are sent across multiple packets to
	// stay below the 1280 byte limit. We compute the maximum number
	// of entries by stuffing a packet until it grows too large.
	maxNeighbors int
)

type Peer p2p.Peer

type packet interface {
	handle(t *KademliaReactor, from Peer, mac []byte) error
	name() string
}

type KademliaReactor struct {
	transport
	p2p.BaseReactor
	nodeTable* Table

	ourEndpoint p2p.NetAddress

	addpending chan *pending
	gotreply   chan reply

	closing chan struct{}

	attemptsToDial sync.Map // address (string) -> {number of attempts (int), last time dialed (time.Time)}

	netrestrict *netutil.Netlist
	bootNodes []*Node
	bootNodesAddresses []*p2p.NetAddress
}

type _attemptsToDial struct {
	number     int
	lastDialed time.Time
}

// RPC request structures
type (
	ping struct {
		Version    uint
		From, To   p2p.NetAddress
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// pong is the reply to ping.
	pong struct {
		// This field should mirror the envelope address
		// of the ping packet, which provides a way to kademlia the
		// the external address (after NAT).
		To p2p.NodeID

		ReplyTok   []byte // This contains the hash of the ping packet.
		Expiration uint64 // Absolute timestamp at which the packet becomes invalid.
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// findnode is a query for nodes close to the given target.
	findnode struct {
		Target     p2p.NodeID // doesn't need to be an actual public key
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}

	// reply to findnode
	neighbors struct {
		Nodes      []Node
		Expiration uint64
		// Ignore additional fields (for forward compatibility).
		Rest []rlp.RawValue `rlp:"tail"`
	}
)

// pending represents a pending reply.
//
// some implementations of the protocol wish to send more than one
// reply packet to findnode. in general, any neighbors packet cannot
// be matched up with a specific findnode packet.
//
// our implementation handles this by storing a callback function for
// each pending reply. incoming packets from a node are dispatched
// to all the callback functions for that node.
type pending struct {
	// these fields must match in the reply.
	from  p2p.NetAddress
	ptype byte

	// time when the request must complete
	deadline time.Time

	// callback is called when a matching reply arrives. if it returns
	// true, the callback is removed from the pending reply queue.
	// if it returns false, the reply is considered incomplete and
	// the callback will be invoked again for the next matching reply.
	callback func(resp interface{}) (done bool)

	// errc receives nil when the callback indicates completion or an
	// error if no further reply is received within the timeout.
	errc chan<- error
}

type reply struct {
	from  Peer
	ptype byte
	data  interface{}
	// loop indicates whether there was
	// a matching request by sending on this channel.
	matched chan<- bool
}


// NewKademliaReactor creates new Kademlia reactor.
func NewKademliaReactor(seedNetAddresses []*p2p.NetAddress) *KademliaReactor {
	r := new(KademliaReactor)
	r.bootNodes = convertToNodes(seedNetAddresses)
	r.bootNodesAddresses = seedNetAddresses
	r.BaseReactor = *p2p.NewBaseReactor("KademliaReactor", r)
	return r
}

func convertToNodes(seedNetAddresses []*p2p.NetAddress) []*Node {
	seeds := make([]*Node, 0)
	for i := 0; i < len(seedNetAddresses); i++ {
		node := Node {
			addedAt: time.Now(),
			IP: seedNetAddresses[i].IP,
			ID: seedNetAddresses[i].ID,
			UDP: seedNetAddresses[i].Port,
			TCP: seedNetAddresses[i].Port,
		}
		seeds = append(seeds, &node)
	}
	return seeds
}

// OnStart implements BaseService
func (r *KademliaReactor) OnStart() error {
	if err := r.BaseReactor.OnStart(); err != nil {
		return err
	}

	r.Logger.Debug("Initializing routing table", "boot_count", r.bootNodes)
	for _, addr := range r.bootNodesAddresses {
		r.Switch.DialPeerWithAddress(addr, true)
	}
	tab, err := newTable(r, r.Switch.NodeInfo(), nodeDBPath, r.bootNodes, r.Logger)
	if err != nil {
		return err
	}
	r.nodeTable = tab

	p := neighbors{Expiration: ^uint64(0)}

	for n := 0; ; n++ {
		//Fixme
		maxSizeNode := Node{IP: make(net.IP, 16), UDP: ^uint16(0), TCP: ^uint16(0)}

		p.Nodes = append(p.Nodes, maxSizeNode)
		size, _, err := rlp.EncodeToReader(p)
		if err != nil {
			// If this ever happens, it will be caught by the unit tests.
			panic("cannot encode: " + err.Error())
		}
		if headSize+size+1 >= 1280 {
			maxNeighbors = n
			break
		}
	}

	go r.loop()

	return nil
}

// OnStop implements BaseService
func (r *KademliaReactor) OnStop() {
	r.BaseReactor.OnStop()
}

// GetChannels implements Reactor
func (r *KademliaReactor) GetChannels() []*p2pconn.ChannelDescriptor {
	return []*p2pconn.ChannelDescriptor{
		{
			ID:                KademliaChannel,
			Priority:          1,
			SendQueueCapacity: 10,
		},
	}
}


// AddPeer implements Reactor by adding peer to the address book (if inbound)
// or by requesting more addresses (if outbound).
func (r *KademliaReactor) AddPeer(p p2p.Peer) {
	r.Logger.Debug("AddPeer invoked on KademliaReactor [ignored]")
}

// RemovePeer implements Reactor.
func (r *KademliaReactor) RemovePeer(p p2p.Peer, reason interface{}) {
	r.Logger.Debug("RemovePeer invoked on KademliaReactor [ignored]")
}


// Receive implements Reactor by handling incoming Kademlia messages.
func (r *KademliaReactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {
	r.Logger.Debug("Received message on KademliaReactor")
	r.handlePacket(src, msgBytes)
}

func (t *KademliaReactor) handlePacket(from Peer, buf []byte) error {
	packet, hash, err := decodePacket(t, buf)
	if err != nil {
		t.Logger.Debug("Bad discv4 packet", "addr", from, "err", err)
		return err
	}
	err = packet.handle(t, from, hash)
	t.Logger.Debug("<< "+packet.name(), "addr", from, "err", err)
	return err
}

func decodePacket(t *KademliaReactor, buf []byte) (packet, []byte, error) {
	if len(buf) < headSize+1 {
		return nil, nil, errPacketTooSmall
	}
	hash, sigdata := buf[:macSize], buf[headSize:]
	shouldhash := crypto.Keccak256(buf[macSize:])
	if !bytes.Equal(hash, shouldhash) {
		return nil, nil, errBadHash
	}
	var req packet
	switch ptype := sigdata[0]; ptype {
	case pingPacket:
		t.Logger.Debug("Received ping message on KademliaReactor")
		req = new(ping)
	case pongPacket:
		t.Logger.Debug("Received pong message on KademliaReactor")
		req = new(pong)
	case findnodePacket:
		t.Logger.Debug("Received findNode message on KademliaReactor")
		req = new(findnode)
	case neighborsPacket:
		t.Logger.Debug("Received neighbours message on KademliaReactor")
		req = new(neighbors)
	default:
		return nil, hash, fmt.Errorf("unknown type: %d", ptype)
	}
	s := rlp.NewStream(bytes.NewReader(sigdata[1:]), 0)
	err := s.Decode(req)
	return req, hash, err
}

// ping sends a ping message to the given node and waits for a reply.
func (t *KademliaReactor) ping(toid p2p.NetAddress) error {
	return <-t.sendPing(toid, nil)
}

// sendPing sends a ping message to the given node and invokes the callback
// when the reply arrives.
func (t *KademliaReactor) sendPing(toid p2p.NetAddress, callback func()) <-chan error {
	p := t.Switch.Peers().Get(toid.ID);
	req := &ping{
		Version:    4,
		From:       t.ourEndpoint,
		To:			toid,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}
	b := new(bytes.Buffer)
	if err := rlp.Encode(b, req); err != nil {
		t.Logger.Error("Can't encode discv4 packet", "err", err)
		return nil // FIXME
	}

	hash := crypto.Keccak256(b.Bytes())
	errc := t.pending(toid, pongPacket, func(p interface{}) bool {
		ok := bytes.Equal(p.(*pong).ReplyTok, hash)
		if ok && callback != nil {
			callback()
		}
		return ok
	})

	p.Send(KademliaChannel, b.Bytes())
	return errc
}

func (t *KademliaReactor) waitping(from p2p.NetAddress) error {
	return <-t.pending(from, pingPacket, func(interface{}) bool { return true })
}

// findnode sends a findnode request to the given node and waits until
// the node has sent up to k neighbors.
func (t *KademliaReactor) findnode(toid p2p.NetAddress, target p2p.NodeID) ([]*Node, error) {
	p := t.Switch.Peers().Get(toid.ID);

	// If we haven't seen a ping from the destination node for a while, it won't remember
	// our endpoint proof and reject findnode. Solicit a ping first.
	if time.Since(t.nodeTable.db.lastPingReceived(toid.ID)) > nodeDBNodeExpiration {
		t.ping(toid)
		t.waitping(toid)
	}

	nodes := make([]*Node, 0, bucketSize)
	nreceived := 0
	errc := t.pending(toid, neighborsPacket, func(r interface{}) bool {
		reply := r.(*neighbors)
		for _, rn := range reply.Nodes {
			nreceived++
			n, err := t.nodeFromRPC(toid, &rn)
			if err != nil {
				t.Logger.Debug("Invalid neighbor node received", "ip", rn.IP, "addr", toid.ID.ID, "err", err)
				continue
			}
			nodes = append(nodes, n)
		}
		return nreceived >= bucketSize
	})

	req := findnode{
		Target:     target,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	b := new(bytes.Buffer)
	if err := rlp.Encode(b, req); err != nil {
		t.Logger.Error("Can't encode request data", "err", err)
		return nil, nil // FIXME
	}

	p.Send(KademliaChannel, b.Bytes())
	return nodes, <-errc
}

// pending adds a reply callback to the pending reply queue.
// see the documentation of type pending for a detailed explanation.
func (t *KademliaReactor) pending(id p2p.NetAddress, ptype byte, callback func(interface{}) bool) <-chan error {
	ch := make(chan error, 1)
	p := &pending{from: id, ptype: ptype, callback: callback, errc: ch}
	select {
	case t.addpending <- p:
		// loop will handle it
	case <-t.closing:
		ch <- errClosed
	}
	return ch
}

func (t *KademliaReactor) handleReply(from Peer, ptype byte, req packet) bool {
	matched := make(chan bool, 1)
	select {
	case t.gotreply <- reply{from, ptype, req, matched}:
		// loop will handle it
		return <-matched
	case <-t.closing:
		return false
	}
}

func (t *KademliaReactor) nodeFromRPC(sender p2p.NetAddress, rn *Node) (*Node, error) {

	if err := netutil.CheckRelayIP(sender.IP, rn.IP); err != nil {
		return nil, err
	}
	if t.netrestrict != nil && !t.netrestrict.Contains(rn.IP) {
		return nil, errors.New("not contained in netrestrict whitelist")
	}
	n := NewNode(rn.ID, rn.IP)
	err := n.validateComplete()
	return n, err
}

func (r *KademliaReactor) dialAttemptsInfo(addr *p2p.NetAddress) (attempts int, lastDialed time.Time) {
	_attempts, ok := r.attemptsToDial.Load(addr.DialString())
	if !ok {
		return
	}
	atd := _attempts.(_attemptsToDial)
	return atd.number, atd.lastDialed
}

// loop runs in its own goroutine. it keeps track of
// the refresh timer and the pending reply queue.
func (t *KademliaReactor) loop() {
	var (
		plist        = list.New()
		timeout      = time.NewTimer(0)
		nextTimeout  *pending // head of plist when timeout was last reset
		contTimeouts = 0      // number of continuous timeouts to do NTP checks
		ntpWarnTime  = time.Unix(0, 0)
	)
	<-timeout.C // ignore first timeout
	defer timeout.Stop()

	resetTimeout := func() {
		if plist.Front() == nil || nextTimeout == plist.Front().Value {
			return
		}
		// Start the timer so it fires when the next pending reply has expired.
		now := time.Now()
		for el := plist.Front(); el != nil; el = el.Next() {
			nextTimeout = el.Value.(*pending)
			if dist := nextTimeout.deadline.Sub(now); dist < 2*respTimeout {
				timeout.Reset(dist)
				return
			}
			// Remove pending replies whose deadline is too far in the
			// future. These can occur if the system clock jumped
			// backwards after the deadline was assigned.
			nextTimeout.errc <- errClockWarp
			plist.Remove(el)
		}
		nextTimeout = nil
		timeout.Stop()
	}

	for {
		resetTimeout()

		select {
		case <-t.closing:
			for el := plist.Front(); el != nil; el = el.Next() {
				el.Value.(*pending).errc <- errClosed
			}
			return

		case p := <-t.addpending:
			p.deadline = time.Now().Add(respTimeout)
			plist.PushBack(p)

		case r := <-t.gotreply:
			var matched bool
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*pending)
				b := r.from.ID().ID
				if 0 == bytes.Compare(p.from.ID.ID[:], b[:]) && p.ptype == r.ptype {
					matched = true
					// Remove the matcher if its callback indicates
					// that all replies have been received. This is
					// required for packet types that expect multiple
					// reply packets.
					if p.callback(r.data) {
						p.errc <- nil
						plist.Remove(el)
					}
					// Reset the continuous timeout counter (time drift detection)
					contTimeouts = 0
				}
			}
			r.matched <- matched

		case now := <-timeout.C:
			nextTimeout = nil

			// Notify and remove callbacks whose deadline is in the past.
			for el := plist.Front(); el != nil; el = el.Next() {
				p := el.Value.(*pending)
				if now.After(p.deadline) || now.Equal(p.deadline) {
					p.errc <- errTimeout
					plist.Remove(el)
					contTimeouts++
				}
			}
			// If we've accumulated too many timeouts, do an NTP time sync check
			if contTimeouts > ntpFailureThreshold {
				if time.Since(ntpWarnTime) >= ntpWarningCooldown {
					ntpWarnTime = time.Now()
					go checkClockDrift()
				}
				contTimeouts = 0
			}
		}
	}
}

func (req *ping) handle(t *KademliaReactor, from Peer, mac []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	t.Switch.Peers().Get(from.ID())

	pongData := &pong{
		To:         from.ID(),
		ReplyTok:   mac,
		Expiration: uint64(time.Now().Add(expiration).Unix()),
	}

	b := new(bytes.Buffer)
	if err := rlp.Encode(b, pongData); err != nil {
		t.Logger.Error("Can't encode request packet", "err", err)
		return nil // FIXME
	}

	from.Send(KademliaChannel, b.Bytes());
	t.handleReply(from, pingPacket, req)

	// Add the node to the table. Before doing so, ensure that we have a recent enough pong
	// recorded in the database so their findnode requests will be accepted later.
	n := NewNode(from.ID(), from.NodeInfo().NetAddress().IP)
	if time.Since(t.nodeTable.db.lastPongReceived(from.ID())) > nodeDBNodeExpiration {
		t.sendPing(*from.NodeInfo().NetAddress(), func() { t.nodeTable.addThroughPing(n) })
	} else {
		t.nodeTable.addThroughPing(n)
	}
	t.nodeTable.db.updateLastPingReceived(from.ID(), time.Now())
	return nil
}

func (req *ping) name() string { return "PING/v4" }

func (req *pong) handle(t *KademliaReactor, from Peer, mac []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	if !t.handleReply(from, pongPacket, req) {
		return errUnsolicitedReply
	}
	t.nodeTable.db.updateLastPongReceived(from.ID(), time.Now())
	return nil
}

func (req *pong) name() string { return "PONG/v4" }

func (req *findnode) handle(t *KademliaReactor, from Peer, mac []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	if !t.nodeTable.db.hasBond(from.ID()) {
		// No endpoint proof pong exists, we don't process the packet. This prevents an
		// attack vector where the discovery protocol could be used to amplify traffic in a
		// DDOS attack. A malicious actor would send a findnode request with the IP address
		// and UDP port of the target as the source address. The recipient of the findnode
		// packet would then send a neighbors packet (which is a much bigger packet than
		// findnode) to the victim.
		return errUnknownNode
	}
	t.nodeTable.mutex.Lock()
	closest := t.nodeTable.closest(req.Target.ID, bucketSize).entries
	t.nodeTable.mutex.Unlock()

	b := new(bytes.Buffer)
	p := neighbors{Expiration: uint64(time.Now().Add(expiration).Unix())}
	var sent bool
	// Send neighbors in chunks with at most maxNeighbors per packet
	// to stay below the 1280 byte limit.
	for _, n := range closest {
		if netutil.CheckRelayIP(from.NodeInfo().NetAddress().IP, n.IP) == nil {
			p.Nodes = append(p.Nodes, *n)
		}

		if err := rlp.Encode(b, p); err != nil {
			t.Logger.Error("Can't encode request packet", "err", err)
			return nil // FIXME
		}

		if len(p.Nodes) == maxNeighbors {
			from.Send(KademliaChannel, b.Bytes())
			p.Nodes = p.Nodes[:0]
			sent = true
		}
	}

	if err := rlp.Encode(b, p); err != nil {
		t.Logger.Error("Can't encode request packet", "err", err)
		return nil // FIXME
	}

	if len(p.Nodes) > 0 || !sent {
		from.Send(neighborsPacket, b.Bytes())
	}
	return nil
}

func (req *findnode) name() string { return "FINDNODE/v4" }

func (req *neighbors) handle(t *KademliaReactor, from Peer, mac []byte) error {
	if expired(req.Expiration) {
		return errExpired
	}
	if !t.handleReply(from, neighborsPacket, req) {
		return errUnsolicitedReply
	}
	return nil
}

func (req *neighbors) name() string { return "NEIGHBORS/v4" }

func expired(ts uint64) bool {
	return time.Unix(int64(ts), 0).Before(time.Now())
}
