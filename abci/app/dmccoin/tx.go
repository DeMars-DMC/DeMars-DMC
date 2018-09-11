package dmccoin

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/tendermint/tendermint/crypto"

	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
)

/*
Tx (Transaction) is an atomic operation on the ledger state.

Account Types:
 - DMCTx         Send coins to address
 - TxUTXO        Account balance update
*/
type Tx interface {
	AssertIsTx()
	SignBytes() []byte
}

// Types of Tx implementations
const (
	// Account transactions
	TxTypeUTXO = byte(0x01)
	TxTypeApp  = byte(0x02)
	TxTypeDMC  = byte(0x03)
	TxNameUTXO = "utxo"
	TxNameApp  = "app"
	TxNameDMC  = "dmc"
)

func (_ *AppTx) AssertIsTx()  {}
func (_ *DMCTx) AssertIsTx()  {}
func (_ *TxUTXO) AssertIsTx() {}

var txMapper data.Mapper

// register both private key types with go-wire/data (and thus go-wire)
func init() {
	txMapper = data.NewMapper(TxS{}).
		RegisterImplementation(&TxUTXO{}, TxNameUTXO, TxTypeUTXO).
		RegisterImplementation(&AppTx{}, TxNameApp, TxTypeApp).
		RegisterImplementation(&DMCTx{}, TxNameDMC, TxTypeDMC)
}

// TxS add json serialization to Tx
type TxS struct {
	Tx `json:"unwrap"`
}

func (p TxS) MarshalJSON() ([]byte, error) {
	return txMapper.ToJSON(p.Tx)
}

func (p *TxS) UnmarshalJSON(data []byte) (err error) {
	parsed, err := txMapper.FromJSON(data)
	if err == nil {
		p.Tx = parsed.(Tx)
	}
	return
}

//-----------------------------------------------------------------------------

type TxInput struct {
	Address   data.Bytes       `json:"address"`   // Hash of the PubKey
	Coins     uint64           `json:"coins"`     //
	Sequence  int              `json:"sequence"`  // Must be 1 greater than the last committed TxInput
	Signature crypto.Signature `json:"signature"` // Depends on the PubKey type and the whole Tx
	PubKey    crypto.PubKey    `json:"pub_key"`   // Is present iff Sequence == 0
}

func (txIn TxInput) ValidateBasic() int {
	// @arun address length 128 hex bytes
	if len(txIn.Address) != 64 {
		return 0
	}
	// @arun upper limit on tx?
	if txIn.Coins > 100 || txIn.Coins < 0 {
		return 0
	}
	if txIn.Coins == 0 {
		return 0
	}
	if txIn.Sequence <= 0 {
		return 0
	}
	if txIn.Sequence == 1 && len(txIn.PubKey.Bytes()) == 0 {
		return 0
	}
	if txIn.Sequence > 1 && len(txIn.PubKey.Bytes()) > 0 {
		return 0
	}
	return 1
}

func (txIn TxInput) String() string {
	return fmt.Sprintf("TxInput{%X,%v,%v,%v,%v}", txIn.Address, txIn.Coins, txIn.Sequence, txIn.Signature, txIn.PubKey)
}

func NewTxInput(pubKey crypto.PubKey, coins uint64, sequence int) TxInput {
	input := TxInput{
		Address:  pubKey.Address().Bytes(),
		Coins:    coins,
		Sequence: sequence,
	}
	if sequence == 1 {
		input.PubKey = pubKey
	}
	return input
}

//-----------------------------------------------------------------------------

type TxOutput struct {
	Address data.Bytes `json:"address"` // Hash of the PubKey
	Coins   uint64     `json:"coins"`   //
}

// An output destined for another chain may be formatted as `chainID/address`.
// ChainAndAddress returns the chainID prefix and the address.
// If there is no chainID prefix, the first returned value is nil.
func (txOut TxOutput) ChainAndAddress() ([]byte, []byte, int) {
	var chainPrefix []byte
	address := txOut.Address
	if len(address) > 20 {
		spl := bytes.SplitN(address, []byte("/"), 2)
		if len(spl) != 2 {
			return nil, nil, 0
		}
		chainPrefix = spl[0]
		address = spl[1]
	}

	if len(address) != 20 {
		return nil, nil, 0
	}
	return chainPrefix, address, 1
}

func (txOut TxOutput) ValidateBasic() int {
	_, _, r := txOut.ChainAndAddress()
	if r == 0 {
		return r
	}

	if txOut.Coins > 100 {
		return 0
	}
	if txOut.Coins == 0 {
		return 0
	}
	return 1
}

func (txOut TxOutput) String() string {
	return fmt.Sprintf("TxOutput{%X,%v}", txOut.Address, txOut.Coins)
}

type TxUTXO struct {
	Address data.Bytes `json:"address"` // Hash of the PubKey
	Balance uint64     `json:"coins"`   //
}

func (tx *TxUTXO) SignBytes() []byte {
	return []byte{}
}

//-----------------------------------------------------------------------------

type DMCTx struct {
	Fee    uint64   `json:"DoS prevention fee"`
	Input  TxInput  `json:"Input"`
	Output TxOutput `json:"Output"`
}

func (tx *DMCTx) SignBytes() []byte {
	sigz := tx.Input.Signature
	tx.Input.Signature = nil
	signBytes := wire.BinaryBytes(tx)
	tx.Input.Signature = sigz
	return signBytes
}

func (tx *DMCTx) SetSignature(addr []byte, sig crypto.Signature) bool {
	if bytes.Equal(tx.Input.Address, addr) {
		tx.Input.Signature = sig
		return true
	}
	return false
}

func (tx *DMCTx) String() string {
	return fmt.Sprintf("DMCTx{%v %v->%v}", tx.Fee, tx.Input, tx.Output)
}

//-----------------------------------------------------------------------------
// For smart contracts
type AppTx struct {
	Gas   int64           `json:"gas"`   // Gas
	Fee   uint64          `json:"fee"`   // Fee
	Name  string          `json:"type"`  // Which plugin
	Input TxInput         `json:"input"` // Hmmm do we want coins?
	Data  json.RawMessage `json:"data"`
}

func (tx *AppTx) SignBytes() []byte {
	sig := tx.Input.Signature
	signBytes := wire.BinaryBytes(tx)
	tx.Input.Signature = sig
	return signBytes
}

func (tx *AppTx) SetSignature(sig crypto.Signature) bool {
	tx.Input.Signature = sig
	return true
}

func (tx *AppTx) String() string {
	return fmt.Sprintf("AppTx{%v/%v %v %v %X}", tx.Gas, tx.Fee, tx.Name, tx.Input, tx.Data)
}

//-----------------------------------------------------------------------------

func TxID(tx Tx) []byte {
	signBytes := tx.SignBytes()
	return wire.BinaryRipemd160(signBytes)
}

//--------------------------------------------------------------------------------

// Contract: This function is deterministic and completely reversible.
func jsonEscape(str string) string {
	escapedBytes, err := json.Marshal(str)
	if err != nil {
		fmt.Sprintf("Error json-escaping a string %s", str)
	}
	return string(escapedBytes)
}
