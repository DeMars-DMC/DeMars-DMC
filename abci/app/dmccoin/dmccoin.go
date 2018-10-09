package dmccoin

import (
	"encoding/json"
	"fmt"

	"github.com/Demars-DMC/Demars-DMC/abci/app/code"
	"github.com/Demars-DMC/Demars-DMC/abci/types"
	"github.com/Demars-DMC/Demars-DMC/libs/log"
	wire "github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
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

var _ types.Application = (*DMCCoinApplication)(nil)

type DMCCoinApplication struct {
	types.BaseApplication

	state      State
	ValUpdates []types.Validator
	logger     log.Logger
}

func NewDMCCoinApplication(logger log.Logger) *DMCCoinApplication {
	return &DMCCoinApplication{
		state:  *NewState(NewMemKVS(), logger),
		logger: logger,
	}

}

func (app *DMCCoinApplication) SetInitAccount(address data.Bytes, balance uint64) {
	app.state.SetAccount(address, &Account{PubKey: nil, Height: 0, Balance: balance, Address: address})
}

func (app *DMCCoinApplication) Info(req types.RequestInfo) (resInfo types.ResponseInfo) {
	return types.ResponseInfo{Data: fmt.Sprintf("{\"size\":%v}", 0)}
}

// tx is either "key=value" or just arbitrary bytes
func (app *DMCCoinApplication) DeliverTx(txBytes []byte, height int64) types.ResponseDeliverTx {
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

func (app *DMCCoinApplication) GetState() *State {
	return &app.state
}

func (app *DMCCoinApplication) DeliverBucketedTx(txBytes []byte, height int64, bucketID string) (res types.ResponseDeliverTx) {
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

func (app *DMCCoinApplication) CheckTx(txBytes []byte, height int64) types.ResponseCheckTx {
	app.logger.Debug("Checking transaction, height = ", height)
	if len(txBytes) > maxTxSize {
		app.logger.Debug("Max transaction size exceeded")
		return types.ResponseCheckTx{Code: 1}

	}

	trx := TxUTXO{}
	trs := DMCTx{}
	if height%100 == 1 && height > 1 {
		json.Unmarshal(txBytes, &trx)
		if app.GetState().GetAccount(trx.Address).Height != (int)(height-1) {
			// get bucketIDs from address and return
			var BucketIDs [1]string
			BucketIDs[0] = string(trx.Address[:2])
		}
	} else {
		json.Unmarshal(txBytes, &trs)
		s := string(txBytes[:])
		app.logger.Debug(fmt.Sprintf("String %s", s))
		app.logger.Debug(fmt.Sprintf("Input %s", trs.Input))
		app.logger.Debug(fmt.Sprintf("Output %s", trs.Output))
		app.logger.Debug(fmt.Sprintf("Fee %d", trs.Fee))
		app.logger.Debug(fmt.Sprintf("Sequence %d", trs.Input.Sequence))
		app.logger.Debug(fmt.Sprintf("Input Address %s", trs.Input.Address))
		if int64(trs.Input.Sequence) != height {
			app.logger.Debug(fmt.Sprintf("Wrong sequence. Expected: %d Got: %d", height, trs.Input.Sequence))
			return types.ResponseCheckTx{Code: 1}
		}
		bucketIDs := make([]string, 1)
		// We only need to check input address, the output address does not need to be validated
		InputAddressString := fmt.Sprintf("%s", trs.Input.Address)

		bucketIDs[0] = InputAddressString[:2]
		//InputAddressString := string(trs.Input.Address[:])
		//InputAddressString2 := fmt.Sprintf("%s", trs.Input.Address)
		//app.logger.Debug(fmt.Sprintf("Input address string = %s", InputAddressString))
		//app.logger.Debug(fmt.Sprintf("Input address string2 = %s", InputAddressString2))
		app.logger.Debug(fmt.Sprintf("BucketIDs %v", bucketIDs[0]))
		return types.ResponseCheckTx{BucketIDs: bucketIDs}
	}
	return types.ResponseCheckTx{Code: code.CodeTypeOK}
}

// FIXME: At the moment this call is also used to get UTXO account balances
func (app *DMCCoinApplication) GetValidatorSet(height int64) (res types.ResponseGetValidatorSet) {
	app.logger.Debug(fmt.Sprintf("Retrieving validator set at height %d", height))
	if height < 0 {
		allAccounts := app.GetState().GetAllAccounts()

		accBalances := make([]types.Validator, 0)

		for _, acc1 := range allAccounts {
			validator := types.Validator{}
			validator.Power = int64(acc1.Balance)
			validator.Address = acc1.Address
			accBalances = append(accBalances, validator)
		}
		app.logger.Debug(fmt.Sprintf("Got %d validators", len(allAccounts)))
		app.logger.Debug(fmt.Sprintf("Val1: %d", allAccounts[0].Balance))

		return types.ResponseGetValidatorSet{ValidatorSet: accBalances}
	}

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
func (app *DMCCoinApplication) Commit() types.ResponseCommit {
	// Using a memdb - just return the big endian size of the db
	// Commit state
	app.state.Commit()
	return types.ResponseCommit{} //res
}

func (app *DMCCoinApplication) Query(reqQuery types.RequestQuery) (resQuery types.ResponseQuery) {
	return
}
