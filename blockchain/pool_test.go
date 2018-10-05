package blockchain

import (
	"math/rand"
	"testing"
	"time"

	cmn "github.com/Demars-DMC/Demars-DMC/libs/common"
	"github.com/Demars-DMC/Demars-DMC/libs/log"

	"github.com/Demars-DMC/Demars-DMC/p2p"
	"github.com/Demars-DMC/Demars-DMC/types"
)

func init() {
	peerTimeout = 2 * time.Second
}

type testPeer struct {
	id     p2p.NodeID
	height int64
}

func makePeers(numPeers int, minHeight, maxHeight int64) map[p2p.NodeID]testPeer {
	peers := make(map[p2p.NodeID]testPeer, numPeers)
	for i := 0; i < numPeers; i++ {
		peerID, _ := p2p.HexID(cmn.RandStr(12))

		height := minHeight + rand.Int63n(maxHeight-minHeight)
		peers[peerID] = testPeer{peerID, height}
	}
	return peers
}

func TestBasic(t *testing.T) {
	t.Skip() // FIXME: The test currently locks
	start := int64(42)
	peers := makePeers(10, start+1, 1000)
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(start, requestsCh, errorsCh)
	pool.SetLogger(log.TestingLogger())

	err := pool.Start()
	if err != nil {
		t.Error(err)
	}

	defer pool.Stop()

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerHeight(peer.id, peer.height)
		}
	}()

	// Start a goroutine to pull blocks
	go func() {
		for {
			if !pool.IsRunning() {
				return
			}
			first, second := pool.PeekTwoBlocks()
			if first != nil && second != nil {
				pool.PopRequest()
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Pull from channels
	for {
		select {
		case err := <-errorsCh:
			t.Error(err)
		case request := <-requestsCh:
			t.Logf("Pulled new BlockRequest %v", request)
			if request.Height == 300 {
				return // Done!
			}
			// Request desired, pretend like we got the block immediately.
			go func() {
				block := &types.Block{Header: &types.Header{Height: request.Height}}
				pool.AddBlock(request.PeerID, block, 123)
				t.Logf("Added block from peer %v (height: %v)", request.PeerID, request.Height)
			}()
		}
	}
}

func TestTimeout(t *testing.T) {
	t.Skip() // FIXME
	start := int64(42)
	peers := makePeers(10, start+1, 1000)
	errorsCh := make(chan peerError, 1000)
	requestsCh := make(chan BlockRequest, 1000)
	pool := NewBlockPool(start, requestsCh, errorsCh)
	pool.SetLogger(log.TestingLogger())
	err := pool.Start()
	if err != nil {
		t.Error(err)
	}
	defer pool.Stop()

	for _, peer := range peers {
		t.Logf("Peer %v", peer.id)
	}

	// Introduce each peer.
	go func() {
		for _, peer := range peers {
			pool.SetPeerHeight(peer.id, peer.height)
		}
	}()

	// Start a goroutine to pull blocks
	go func() {
		for {
			if !pool.IsRunning() {
				return
			}
			first, second := pool.PeekTwoBlocks()
			if first != nil && second != nil {
				pool.PopRequest()
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}()

	// Pull from channels
	counter := 0
	timedOut := map[p2p.NodeID]struct{}{}
	for {
		select {
		case err := <-errorsCh:
			t.Log(err)
			// consider error to be always timeout here
			if _, ok := timedOut[err.peerID]; !ok {
				counter++
				if counter == len(peers) {
					return // Done!
				}
			}
		case request := <-requestsCh:
			t.Logf("Pulled new BlockRequest %+v", request)
		}
	}
}
