package consensus

import (
	"github.com/tendermint/go-amino"
	"github.com/Demars-DMC/Demars-DMC/crypto"
)

var cdc = amino.NewCodec()

func init() {
	RegisterConsensusMessages(cdc)
	RegisterWALMessages(cdc)
	crypto.RegisterAmino(cdc)
}
