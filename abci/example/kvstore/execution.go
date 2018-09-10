package kvstore

import (
	"fmt"

	abci "github.com/tendermint/abci/types"
	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/events"
)

// If the tx is invalid, a TMSP error will be returned.
func ExecTx(state *State, tx Tx, isCheckTx bool, evc events.Fireable, height int64) abci.ResponseDeliverTx {
	//chainID := state.GetChainID()

	// Exec tx
	switch tx := tx.(type) {
	case *DMCTx:
		if height%100 == 1 {

		}
		res := validateInputBasic(tx.Input)
		if res == 0 {
			return abci.ResponseDeliverTx{
				Log: fmt.Sprintf("In validateinputBasic()")}
		}
		res = validateOutputBasic(tx.Output)
		if res == 0 {
			return abci.ResponseDeliverTx{
				Log: fmt.Sprintf("in validateOutputBasic()")}
		}
		account, res := getInput(state, tx.Input)
		if res == 0 {
			return abci.ResponseDeliverTx{
				Log: fmt.Sprintf("in getInput()")}
		}

		// Get or make outputs.
		account, res = getOrMakeOutput(state, account, tx.Output)
		if res == 0 {
			return abci.ResponseDeliverTx{
				Log: fmt.Sprintf("in getOrMakeOutput()")}
		}

		// Validate inputs and outputs, advanced
		signBytes := tx.SignBytes()
		res = validateInputAdvanced(account, signBytes, tx.Input)
		inTotal := tx.Input.Coins
		if res == 0 {
			return abci.ResponseDeliverTx{
				Log: fmt.Sprintf("in validateInputAdvanced()")}
		}
		outTotal := tx.Output.Coins
		outPlusFees := outTotal
		fees := tx.Fee
		if fees != 1 { // TODO: fix coins.Plus()
			outPlusFees = outTotal + fees
		}
		if inTotal != outPlusFees {
			return abci.ResponseDeliverTx{
				Log: fmt.Sprintf(cmn.Fmt("Input total (%v) != output total + fees (%v)", inTotal, outPlusFees))}
		}

		// TODO: Fee validation for DMCTx

		// Good! Adjust accounts
		adjustByInput(state, account, tx.Input)
		adjustByOutput(state, account, tx.Output, isCheckTx)

		/*
			// Fire events
			if !isCheckTx {
				if evc != nil {
					for _, i := range tx.Inputs {
						evc.FireEvent(types.EventStringAccInput(i.Address), types.EventDataTx{tx, nil, ""})
					}
					for _, o := range tx.Outputs {
						evc.FireEvent(types.EventStringAccOutput(o.Address), types.EventDataTx{tx, nil, ""})
					}
				}
			}
		*/

		return abci.ResponseDeliverTx{
			Log: fmt.Sprintf(string(TxID(tx)), "")}

		// For future smart contract functionality
	case *AppTx:
		// Validate input, basic
		// res := tx.Input.ValidateBasic()
		// if res.IsErr() {
		// 	return res
		// }
		//
		// // Get input account
		// inAcc := state.GetAccount(tx.Input.Address)
		// if inAcc == nil {
		// 	return abci.ErrBaseUnknownAddress
		// }
		// if !tx.Input.PubKey.Empty() {
		// 	inAcc.PubKey = tx.Input.PubKey
		// }
		//
		// // Validate input, advanced
		// signBytes := tx.SignBytes(chainID)
		// res = validateInputAdvanced(inAcc, signBytes, tx.Input)
		// if res.IsErr() {
		// 	state.logger.Info(cmn.Fmt("validateInputAdvanced failed on %X: %v", tx.Input.Address, res))
		// 	return res.PrependLog("in validateInputAdvanced()")
		// }
		// if tx.Input.Coins < (types.Coins{tx.Fee}) {
		// 	state.logger.Info(cmn.Fmt("Sender did not send enough to cover the fee %X", tx.Input.Address))
		// 	return abci.ErrBaseInsufficientFunds.AppendLog(cmn.Fmt("input coins is %v, but fee is %v", tx.Input.Coins, types.Coins{tx.Fee}))
		// }
		//
		// // Validate call address
		// plugin := pgz.GetByName(tx.Name)
		// if plugin == nil {
		// 	return abci.ErrBaseUnknownAddress.AppendLog(
		// 		cmn.Fmt("Unrecognized plugin name%v", tx.Name))
		// }
		//
		// // Good!
		// coins := tx.Input.Coins.Minus(types.Coins{tx.Fee})
		// inAcc.Sequence += 1
		// inAcc.Balance = inAcc.Balance.Minus(tx.Input.Coins)
		//
		// // If this is a CheckTx, stop now.
		// if isCheckTx {
		// 	state.SetAccount(tx.Input.Address, inAcc)
		// 	return 1
		// }
		//
		// // Create inAcc checkpoint
		// inAccCopy := inAcc.Copy()
		//
		// // Run the tx.
		// cache := state.CacheWrap()
		// cache.SetAccount(tx.Input.Address, inAcc)
		// ctx := types.NewCallContext(tx.Input.Address, inAcc, coins)
		// res = plugin.RunTx(cache, ctx, tx.Data)
		// if res.IsOK() {
		// 	cache.CacheSync()
		// 	state.logger.Info("Successful execution")
		// 	// Fire events
		// 	/*
		// 		if evc != nil {
		// 			exception := ""
		// 			if res.IsErr() {
		// 				exception = res.Error()
		// 			}
		// 			evc.FireEvent(types.EventStringAccInput(tx.Input.Address), types.EventDataTx{tx, ret, exception})
		// 			evc.FireEvent(types.EventStringAccOutput(tx.Address), types.EventDataTx{tx, ret, exception})
		// 		}
		// 	*/
		// } else {
		// 	state.logger.Info("AppTx failed", "error", res)
		// 	// Just return the coins and return.
		// 	inAccCopy.Balance = inAccCopy.Balance.Plus(coins)
		// 	// But take the gas
		// 	// TODO
		// 	state.SetAccount(tx.Input.Address, inAccCopy)
		// }
		// return res
		return abci.ResponseDeliverTx{
			Log: fmt.Sprintf(string(TxID(tx)), "")}
		// @arun
		// TODO: case for DMC tx
	default:
		return abci.ResponseDeliverTx{
			Log: fmt.Sprintf(string(TxID(tx)), "")}
	}
}

//--------------------------------------------------------------------------------

// The accounts from the TxInputs must either already have
// crypto.PubKey.(type) != nil, (it must be known),
// or it must be specified in the TxInput.
func getInputs(state AccountGetter, ins []TxInput) (map[string]*Account, int) {
	accounts := map[string]*Account{}
	for _, in := range ins {
		// Account shouldn't be duplicated
		if _, ok := accounts[string(in.Address)]; ok {
			return nil, 0
		}

		acc := state.GetAccount(in.Address)
		if acc == nil {
			return nil, 0
		}

		if !in.PubKey.Equals(acc.PubKey) {
			acc.PubKey = in.PubKey
		}
		accounts[string(in.Address)] = acc
	}
	return accounts, 1
}

// @arun
func getInput(state AccountGetter, in TxInput) (*Account, int) {
	//accounts := types.Account{}

	acc := state.GetAccount(in.Address)
	// if acc == nil {
	// 	return acc, abci.ErrBaseUnknownAddress
	// }

	if !in.PubKey.Equals(acc.PubKey) {
		acc.PubKey = in.PubKey
	}
	return acc, 1
}

func getOrMakeOutputs(state AccountGetter, accounts map[string]*Account, outs []TxOutput) (map[string]*Account, int) {
	if accounts == nil {
		accounts = make(map[string]*Account)
	}

	for _, out := range outs {
		chain, outAddress, _ := out.ChainAndAddress() // already validated
		if chain != nil {
			// we dont need an account for the other chain.
			// we'll just create an outgoing ibc packet
			continue
		}
		// Account shouldn't be duplicated
		if _, ok := accounts[string(outAddress)]; ok {
			return nil, 0
		}
		acc := state.GetAccount(outAddress)
		// output account may be nil (new)
		if acc == nil {
			// zero value is valid, empty account
			acc = &Account{}
		}
		accounts[string(outAddress)] = acc
	}
	return accounts, 1
}

func getOrMakeOutput(state AccountGetter, accounts *Account, out TxOutput) (*Account, int) {
	outAddress := out.Address
	acc := state.GetAccount(outAddress)
	// output account may be nil (new)
	if acc == nil {
		// zero value is valid, empty account
		acc = &Account{}
	}
	return acc, 1
}

// Validate inputs basic structure
func validateInputsBasic(ins []TxInput) (res int) {
	for _, in := range ins {
		// Check TxInput basic
		if res := in.ValidateBasic(); res == 0 {
			return res
		}
	}
	return 1
}

// Validate input basic structure
func validateInputBasic(in TxInput) (res int) {
	if res := in.ValidateBasic(); res == 0 {
		return res
	}
	return 1
}

// Validate inputs and compute total amount of coins
func validateInputsAdvanced(accounts map[string]*Account, signBytes []byte, ins []TxInput) (total uint64, res int) {
	for _, in := range ins {
		acc := accounts[string(in.Address)]
		if acc == nil {
			cmn.PanicSanity("validateInputsAdvanced() expects account in accounts")
		}
		res = validateInputAdvanced(acc, signBytes, in)
		if res == 0 {
			return
		}
		// Good. Add amount to total
		total = total + in.Coins
	}
	return total, 1
}

func validateInputAdvanced(acc *Account, signBytes []byte, in TxInput) (res int) {
	// Check sequence/coins
	//height, balance := acc.Height, acc.Balance
	balance := acc.Balance
	//if seq+1 != in.Sequence {
	//return abci.ErrBaseInvalidSequence.AppendLog(cmn.Fmt("Got %v, expected %v. (acc.seq=%v)", in.Sequence, seq+1, acc.Sequence))
	//}
	// Check amount
	if balance < in.Coins {
		return 0
	}
	// Check signatures
	if !acc.PubKey.VerifyBytes(signBytes, in.Signature) {
		return 0
	}
	return 1
}

func validateOutputBasic(out TxOutput) (res int) {
	if res := out.ValidateBasic(); res == 0 {
		return res
	}
	return 1
}

func validateOutputsBasic(outs []TxOutput) (res int) {
	for _, out := range outs {
		// Check TxOutput basic
		if res := out.ValidateBasic(); res == 0 {
			return res
		}
	}
	return 1
}

func sumOutputs(outs []TxOutput) (total uint64) {
	for _, out := range outs {
		total = total + (out.Coins)
	}
	return total
}

func adjustByInputs(state AccountSetter, accounts map[string]*Account, ins []TxInput) {
	for _, in := range ins {
		acc := accounts[string(in.Address)]
		if acc == nil {
			cmn.PanicSanity("adjustByInputs() expects account in accounts")
		}
		if acc.Balance < in.Coins {
			cmn.PanicSanity("adjustByInputs() expects sufficient funds")
		}
		acc.Balance = acc.Balance - (in.Coins)
		acc.Height += 1
		state.SetAccount(in.Address, acc)
	}
}

func adjustByInput(state AccountSetter, acc *Account, in TxInput) {
	//for _, in := range ins {
	//acc := accounts[string(in.Address)]
	if acc == nil {
		cmn.PanicSanity("adjustByInput() expects account in accounts")
	}
	if acc.Balance < in.Coins {
		cmn.PanicSanity("adjustByInput() expects sufficient funds")
	}
	acc.Balance = acc.Balance - in.Coins
	acc.Height += 1
	state.SetAccount(in.Address, acc)
}

func adjustByOutputs(state *State, accounts map[string]*Account, outs []TxOutput, isCheckTx bool) {
	/*for _, out := range outs {
		destChain, outAddress, _ := out.ChainAndAddress() // already validated
		if destChain != nil {
			payload := ibc.CoinsPayload{outAddress, out.Coins}
			ibc.SaveNewIBCPacket(state, state.GetChainID(), string(destChain), payload)
			continue
		}

		acc := accounts[string(outAddress)]
		if acc == nil {
			cmn.PanicSanity("adjustByOutputs() expects account in accounts")
		}
		acc.Balance = acc.Balance + (out.Coins)
		if !isCheckTx {
			state.SetAccount(outAddress, acc)
		}
	}*/
}

func adjustByOutput(state *State, acc *Account, out TxOutput, isCheckTx bool) {
	_, outAddress, _ := out.ChainAndAddress() // already validated
	//if destChain != nil {
	//	payload := ibc.CoinsPayload{outAddress, out.Coins}
	//	ibc.SaveNewIBCPacket(state, state.GetChainID(), string(destChain), payload)
	//}

	if acc == nil {
		cmn.PanicSanity("adjustByOutputs() expects account in accounts")
	}
	acc.Balance = acc.Balance + out.Coins
	if !isCheckTx {
		state.SetAccount(outAddress, acc)
	}
}