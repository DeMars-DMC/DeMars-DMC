package kvstore

import (
	"encoding/json"
	"fmt"

	"github.com/tendermint/tendermint/libs/log"

	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	"github.com/tendermint/tendermint/abci/example/code"
	"github.com/tendermint/tendermint/abci/types"
)

var (
	stateKey        = []byte("stateKey")
	kvPairPrefixKey = []byte("kvPairKey:")
)

const (
	maxTxSize = 10240
)

func prefixKey(key []byte) []byte {
	return append(kvPairPrefixKey, key...)
}

//---------------------------------------------------

var _ types.Application = (*KVStoreApplication)(nil)

type KVStoreApplication struct {
	types.BaseApplication

	state      State
	ValUpdates []types.Validator
	logger     log.Logger
}

func NewKVStoreApplication(logger log.Logger) *KVStoreApplication {
	return &KVStoreApplication{
		state:  *NewState(NewMemKVS(), logger),
		logger: logger,
	}

}

func (app *KVStoreApplication) SetInitAccount(address data.Bytes, balance uint64) {
	app.state.SetAccount(address, &Account{PubKey: nil, Height: 0, Balance: balance, Address: address})
}

func (app *KVStoreApplication) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{Data: fmt.Sprintf("{\"size\":%v}", 0)}
}

// tx is either "key=value" or just arbitrary bytes
func (app *KVStoreApplication) DeliverTx(txBytes []byte, height int64) types.ResponseDeliverTx {
	if len(txBytes) > maxTxSize {
		return types.ResponseDeliverTx{Code: 1}
	}

	// Decode tx
	var tx Tx
	err := wire.ReadBinaryBytes(txBytes, &tx)
	if err != nil {
		return types.ResponseDeliverTx{Code: 1}
	}

	// Validate and exec tx
	_ = ExecTx(&app.state, tx, false, nil, height)
	return types.ResponseDeliverTx{Code: 0}
}

func (app *KVStoreApplication) GetState() *State {
	return &app.state
}

func (app *KVStoreApplication) DeliverBucketedTx(txBytes []byte, height int64, bucketID string) (res types.ResponseDeliverTx) {
	if len(txBytes) > maxTxSize {
		return types.ResponseDeliverTx{Code: 1}
	}

	var trs DMCTx
	var trx TxUTXO
	if height%100 == 1 {
		var acc Account
		acc.Height = int(height)
		acc.Balance = trx.Balance
		app.GetState().SetAccount(trs.Input.Address, &acc)
	}

	if string(trs.Output.Address[:2]) == bucketID {
		var acc Account
		acc.Height = int(height)
		acc.Balance = app.GetState().GetAccount(trs.Output.Address).Balance - trs.Output.Coins
		app.GetState().SetAccount(trs.Output.Address, &acc)
	}
	return types.ResponseDeliverTx{}
}

func (app *KVStoreApplication) CheckTx(txBytes []byte, height int64) types.ResponseCheckTx {
	app.logger.Debug("Checking transaction")
	if len(txBytes) > maxTxSize {
		app.logger.Debug("Max transaction size exceeded")
		return types.ResponseCheckTx{Code: 1}

	}

	var trx TxUTXO
	trs := DMCTx{}
	if height%100 == 1 {
		_ = wire.ReadBinaryBytes(txBytes, &trx)
		if app.GetState().GetAccount(trx.Address).Height != (int)(height-1) {
			// get bucketIDs from address and return
			var BucketIDs [1]string
			BucketIDs[0] = string(trx.Address[:2])
		}
	} else {
		json.Unmarshal(txBytes, &trs)
		s := string(txBytes[:])
		app.logger.Debug(fmt.Sprintf("String %s", s))
		app.logger.Debug(fmt.Sprintf("Sequence %d", trs.Input.Sequence))
		app.logger.Debug(fmt.Sprintf("Address %s", trs.Input.Address))
		if int64(trs.Input.Sequence) != height {
			app.logger.Debug(fmt.Sprintf("Wrong sequence. Expected: %d Got: %d", height, trs.Input.Sequence))
			return types.ResponseCheckTx{Code: 1}
		}
		var BucketIDs []string
		if app.GetState().GetAccount(trs.Input.Address).Height != (int)(height-1) {
			BucketIDs[0] = string(trs.Input.Address[:2])
		}
		return types.ResponseCheckTx{BucketIDs: BucketIDs}
	}
	return types.ResponseCheckTx{Code: code.CodeTypeOK}
}

func (app *KVStoreApplication) GetValidatorSet(height int64) (res types.ResponseGetValidatorSet) {
	app.logger.Debug(fmt.Sprintf("Retrieving validator set at height %d", height))
	if height < 0 {
		allaccounts := app.GetState().GetAllAccounts()

		accBalances := make([]types.Validator, 0)

		for _, acc1 := range allaccounts {
			validator := types.Validator{}
			validator.Power = int64(acc1.Balance)
			validator.Address = acc1.Address
			accBalances = append(accBalances, validator)
		}
		app.logger.Debug(fmt.Sprintf("Got %d validators", len(allaccounts)))
		app.logger.Debug(fmt.Sprintf("Val1: %d", allaccounts[0].Balance))
		
		return types.ResponseGetValidatorSet{ValidatorSet: accBalances}
	} else {
		segmentID := height % 100
		SegmentHex := fmt.Sprintf("%x", segmentID)
		lastUTXOheight := (height/100)*100 + 1

		accs := app.GetState().GetAllAccountsInSegment([]byte(SegmentHex), 2)
		app.logger.Debug(fmt.Sprintf("Number of validators: %d", len(accs)))
		app.logger.Debug(fmt.Sprintf("Validator Address: %d", accs[0].Address))
		app.logger.Debug(fmt.Sprintf("Validator Height: %d", accs[0].Height))
		app.logger.Debug(fmt.Sprintf("Last UTXO Height: %d", lastUTXOheight))
		for _, acc := range accs {
			if acc.Height >= (int)(lastUTXOheight) {
				// Abnormal behaviour: we do not have the latest account balances
				// FIXME: We need to keep track of account balances at last UTXO to get
				// list of validators
				return types.ResponseGetValidatorSet{}
			}
		}
		validatorAccounts := TopkAccounts(accs, 100)
		validatorSet := make([]types.Validator, 0)

		for _, acc1 := range validatorAccounts {
			validator := types.Validator{}
			validator.Power = 1
			validator.Address = acc1.Address
			validatorSet = append(validatorSet, validator)
		}
		return types.ResponseGetValidatorSet{ValidatorSet: validatorSet}
	}
}
func (app *KVStoreApplication) Commit() types.ResponseCommit {
	// Using a memdb - just return the big endian size of the db
	// Commit state
	res := app.state.Commit()

	if res == "" {
	}
	return types.ResponseCommit{} //res
}

func (app *KVStoreApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	return
}
