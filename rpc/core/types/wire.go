package core_types

import (
	"github.com/tendermint/go-amino"
	"github.com/Demars-DMC/Demars-DMC/crypto"
	"github.com/Demars-DMC/Demars-DMC/types"
)

func RegisterAmino(cdc *amino.Codec) {
	types.RegisterEventDatas(cdc)
	types.RegisterEvidences(cdc)
	crypto.RegisterAmino(cdc)
}
