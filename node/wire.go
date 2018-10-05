package node

import (
	amino "github.com/tendermint/go-amino"
	crypto "github.com/Demars-DMC/Demars-DMC/crypto"
)

var cdc = amino.NewCodec()

func init() {
	crypto.RegisterAmino(cdc)
}
