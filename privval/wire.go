package privval

import (
	"github.com/tendermint/go-amino"
	"github.com/Demars-DMC/Demars-DMC/crypto"
)

var cdc = amino.NewCodec()

func init() {
	crypto.RegisterAmino(cdc)
	RegisterSocketPVMsg(cdc)
}
