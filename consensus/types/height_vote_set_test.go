package types

import (
	"testing"
	"time"

	cfg "github.com/Demars-DMC/Demars-DMC/config"
	"github.com/Demars-DMC/Demars-DMC/types"
	cmn "github.com/Demars-DMC/Demars-DMC/libs/common"
	"github.com/Demars-DMC/Demars-DMC/p2p"
)

var config *cfg.Config // NOTE: must be reset for each _test.go file

func init() {
	config = cfg.ResetTestRoot("consensus_height_vote_set_test")
}

func TestPeerCatchupRounds(t *testing.T) {
	valSet, privVals := types.RandValidatorSet(10, 1)

	hvs := NewHeightVoteSet(config.ChainID(), 1, valSet)

	vote999_0 := makeVoteHR(t, 1, 999, privVals, 0)
	peer1, _ := p2p.HexID("peer1")
	peer2, _ := p2p.HexID("peer2")

	added, err := hvs.AddVote(vote999_0, peer1)
	if !added || err != nil {
		t.Error("Expected to successfully add vote from peer", added, err)
	}

	vote1000_0 := makeVoteHR(t, 1, 1000, privVals, 0)
	added, err = hvs.AddVote(vote1000_0, peer1)
	if !added || err != nil {
		t.Error("Expected to successfully add vote from peer", added, err)
	}

	vote1001_0 := makeVoteHR(t, 1, 1001, privVals, 0)
	added, err = hvs.AddVote(vote1001_0, peer2)
	if err != GotVoteFromUnwantedRoundError {
		t.Errorf("Expected GotVoteFromUnwantedRoundError, but got %v", err)
	}
	if added {
		t.Error("Expected to *not* add vote from peer, too many catchup rounds.")
	}

	added, err = hvs.AddVote(vote1001_0, peer2)
	if !added || err != nil {
		t.Error("Expected to successfully add vote from another peer")
	}

}

func makeVoteHR(t *testing.T, height int64, round int, privVals []types.PrivValidator, valIndex int) *types.Vote {
	privVal := privVals[valIndex]
	vote := &types.Vote{
		ValidatorAddress: privVal.GetAddress(),
		ValidatorIndex:   valIndex,
		Height:           height,
		Round:            round,
		Timestamp:        time.Now().UTC(),
		Type:             types.VoteTypePrecommit,
		BlockID:          types.BlockID{[]byte("fakehash"), types.PartSetHeader{}},
	}
	chainID := config.ChainID()
	err := privVal.SignVote(chainID, vote)
	if err != nil {
		panic(cmn.Fmt("Error signing vote: %v", err))
		return nil
	}
	return vote
}
