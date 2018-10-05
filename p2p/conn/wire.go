package conn

import (
	"github.com/tendermint/go-amino"
	"github.com/Demars-DMC/Demars-DMC/crypto"
)

var cdc *amino.Codec = amino.NewCodec()

func init() {
	crypto.RegisterAmino(cdc)
	RegisterPacket(cdc)
}
