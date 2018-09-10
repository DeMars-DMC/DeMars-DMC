package state

import (
	"fmt"

	fail "github.com/ebuchman/fail-test"
	abci "github.com/tendermint/tendermint/abci/types"
	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/proxy"
	"github.com/tendermint/tendermint/types"
)

//-----------------------------------------------------------------------------
// BlockExecutor handles block execution and state updates.
// It exposes ApplyBlock(), which validates & executes the block, updates state w/ ABCI responses,
// then commits and updates the mempool atomically, then saves state.

// BlockExecutor provides the context and accessories for properly executing a block.
type BlockExecutor struct {
	// save state, validators, consensus params, abci responses here
	db dbm.DB

	// execute the app against this
	proxyApp proxy.AppConnConsensus

	// events
	eventBus types.BlockEventPublisher

	// update these with block results after commit
	mempool Mempool

	logger log.Logger
}

func (blockexec BlockExecutor) GetProxyApp() proxy.AppConnConsensus {
	return blockexec.proxyApp
}

// NewBlockExecutor returns a new BlockExecutor with a NopEventBus.
// Call SetEventBus to provide one.
func NewBlockExecutor(db dbm.DB, logger log.Logger, proxyApp proxy.AppConnConsensus,
	mempool Mempool) *BlockExecutor {
	return &BlockExecutor{
		db:       db,
		proxyApp: proxyApp,
		eventBus: types.NopEventBus{},
		mempool:  mempool,
		logger:   logger,
	}
}

// SetEventBus - sets the event bus for publishing block related events.
// If not called, it defaults to types.NopEventBus.
func (blockExec *BlockExecutor) SetEventBus(eventBus types.BlockEventPublisher) {
	blockExec.eventBus = eventBus
}

// ValidateBlock validates the given block against the given state.
// If the block is invalid, it returns an error.
// Validation does not mutate state, but does require historical information from the stateDB,
// ie. to verify evidence from a validator at an old height.
func (blockExec *BlockExecutor) ValidateBlock(state State, block *types.Block) error {
	return validateBlock(blockExec.db, state, block)
}

// TODO This makes an EndBlock call for now but this should be replaced with an update validators call
func (blockExec *BlockExecutor) UpdateValidatorsSet(state State) error {
	abciResponses := ABCIResponses{}
	var err error
	abciResponses.EndBlock, err = blockExec.proxyApp.EndBlockSync(abci.RequestEndBlock{})
	if err != nil {
		return err
	}

	// copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators
	prevValSet := state.Validators.Copy()
	nextValSet := prevValSet.Copy()
	err = updateValidators(nextValSet, abciResponses.EndBlock.ValidatorUpdates)
	if err != nil {
		return err
	}

	return nil
}

func (blockExec *BlockExecutor) ApplyBucket(height int64, txBucket *types.TxBucket) error {
	// Run txs of bucket
	for _, tx := range txBucket.Txs {
		blockExec.proxyApp.DeliverBucketedTxSync(tx, height, txBucket.BucketId)
		if err := blockExec.proxyApp.Error(); err != nil {
			return err
		}
	}
	return nil
}

// ApplyBlock validates the block against the state, executes it against the app,
// fires the relevant events, commits the app, and saves the new state and responses.
// It's the only function that needs to be called
// from outside this package to process and commit an entire block.
// It takes a blockID to avoid recomputing the parts hash.
func (blockExec *BlockExecutor) ApplyBlock(state State, blockID types.BlockID, block *types.Block) (State, error) {
	blockExec.logger.Debug("[ApplyBlock] Validating block")
	if err := blockExec.ValidateBlock(state, block); err != nil {
		return state, ErrInvalidBlock(err)
	}

	blockExec.logger.Debug("[ApplyBlock] Executing block")
	abciResponses, err := execBlockOnProxyApp(blockExec.logger, blockExec.proxyApp, block, state.LastValidators, blockExec.db)
	if err != nil {
		return state, ErrProxyAppConn(err)
	}

	fail.Fail() // XXX

	// save the results before we commit
	blockExec.logger.Debug("[ApplyBlock] Save ABCI responses")
	saveABCIResponses(blockExec.db, block.Height, abciResponses)

	fail.Fail() // XXX

	// update the state with the block and responses
	blockExec.logger.Debug("[ApplyBlock] Update the state")
	state, err = updateState(state, blockID, block.Header, abciResponses)
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}

	// lock mempool, commit app state, update mempoool
	blockExec.logger.Debug("[ApplyBlock] Committing the block")
	appHash, err := blockExec.Commit(block)
	if err != nil {
		return state, fmt.Errorf("Commit failed for application: %v", err)
	}

	fail.Fail() // XXX

	// update the app hash and save the state
	state.AppHash = appHash
	blockExec.logger.Debug("[ApplyBlock] Persisting state")
	SaveState(blockExec.db, state)

	fail.Fail() // XXX

	// events are fired after everything else
	// NOTE: if we crash between Commit and Save, events wont be fired during replay
	blockExec.logger.Debug("[ApplyBlock] Fire events")
	fireEvents(blockExec.logger, blockExec.eventBus, block, abciResponses)

	return state, nil
}

// Commit locks the mempool, runs the ABCI Commit message, and updates the mempool.
// It returns the result of calling abci.Commit (the AppHash), and an error.
// The Mempool must be locked during commit and update because state is typically reset on Commit and old txs must be replayed
// against committed state before new txs are run in the mempool, lest they be invalid.
func (blockExec *BlockExecutor) Commit(block *types.Block) ([]byte, error) {
	blockExec.mempool.Lock()
	defer blockExec.mempool.Unlock()

	// while mempool is Locked, flush to ensure all async requests have completed
	// in the ABCI app before Commit.
	err := blockExec.mempool.FlushAppConn()
	if err != nil {
		blockExec.logger.Error("Client error during mempool.FlushAppConn", "err", err)
		return nil, err
	}

	// Commit block, get hash back
	res, err := blockExec.proxyApp.CommitSync()
	if err != nil {
		blockExec.logger.Error("Client error during proxyAppConn.CommitSync", "err", err)
		return nil, err
	}
	// ResponseCommit has no error code - just data

	blockExec.logger.Info("Committed state",
		"height", block.Height,
		"txs", block.Data.TotalTxsInBuckets(),
		"appHash", fmt.Sprintf("%X", res.Data))

	// Update mempool.
	if err := blockExec.mempool.Update(block.Height, block.TxBuckets); err != nil {
		return nil, err
	}

	return res.Data, nil
}

//---------------------------------------------------------
// Helper functions for executing blocks and updating state

// Executes block's transactions on proxyAppConn.
// Returns a list of transaction results and updates to the validator set
func execBlockOnProxyApp(logger log.Logger, proxyAppConn proxy.AppConnConsensus,
	block *types.Block, lastValSet *types.ValidatorSet, stateDB dbm.DB) (*ABCIResponses, error) {
	var validTxs, invalidTxs = 0, 0

	txIndex := 0
	abciResponses := NewABCIResponses(block)

	// Execute transactions and get hash
	proxyCb := func(req *abci.Request, res *abci.Response) {
		switch r := res.Value.(type) {
		case *abci.Response_DeliverTx:
			// TODO: make use of res.Log
			// TODO: make use of this info
			// Blocks may include invalid txs.
			txRes := r.DeliverTx
			if txRes.Code == abci.CodeTypeOK {
				validTxs++
			} else {
				logger.Debug("Invalid tx", "code", txRes.Code, "log", txRes.Log)
				invalidTxs++
			}
			abciResponses.DeliverTx[txIndex] = txRes
			txIndex++
		}
	}
	proxyAppConn.SetResponseCallback(proxyCb)

	signVals := getBeginBlockValidatorInfo(block, lastValSet, stateDB)

	// Begin block
	logger.Info("[execBlockOnProxyApp] BeginBlock sync")
	_, err := proxyAppConn.BeginBlockSync(abci.RequestBeginBlock{
		Hash:       block.Hash(),
		Header:     types.TM2PB.Header(block.Header),
		Validators: signVals,
	})
	if err != nil {
		logger.Error("Error in proxyAppConn.BeginBlock", "err", err)
		return nil, err
	}

	logger.Info("[execBlockOnProxyApp] Delivering Tx")
	// Run txs of block
	for _, txBucket := range block.TxBuckets {
		for _, tx := range txBucket.Txs {
			proxyAppConn.DeliverTxAsync(tx, block.Height)
			if err := proxyAppConn.Error(); err != nil {
				return nil, err
			}
		}
	}

	// End block
	logger.Info("[execBlockOnProxyApp] End block")
	abciResponses.EndBlock, err = proxyAppConn.EndBlockSync(abci.RequestEndBlock{})
	if err != nil {
		logger.Error("Error in proxyAppConn.EndBlockSync", "err", err)
		return nil, err
	}

	// Update validator set for next block
	logger.Info("[execBlockOnProxyApp] Updating validator set")
	abciResponses.GetValidatorSet, err = proxyAppConn.GetValidatorSetSync(block.Height + 1)
	if err != nil {
		logger.Error("Error in proxyAppConn.GetValidatorSetSync", "err", err)
		return nil, err
	}

	logger.Info("Executed block", "height", block.Height, "validTxs", validTxs, "invalidTxs", invalidTxs)

	valUpdates := abciResponses.GetValidatorSet.ValidatorSet
	if len(valUpdates) > 0 {
		logger.Info("Updates to validators", "updates", abci.ValidatorsString(valUpdates))
	}

	return abciResponses, nil
}

func getBeginBlockValidatorInfo(block *types.Block, lastValSet *types.ValidatorSet, stateDB dbm.DB) []abci.SigningValidator {

	// Sanity check that commit length matches validator set size -
	// only applies after first block
	if block.Height > 1 {
		precommitLen := len(block.Commit.Precommits)
		valSetLen := len(lastValSet.Validators)
		if precommitLen != valSetLen {
			// sanity check
			panic(fmt.Sprintf("precommit length (%d) doesn't match valset length (%d) at height %d\n\n%v\n\n%v",
				precommitLen, valSetLen, block.Height, block.Commit.Precommits, lastValSet.Validators))
		}
	}

	// determine which validators did not sign last block.
	signVals := make([]abci.SigningValidator, len(lastValSet.Validators))
	for i, val := range lastValSet.Validators {
		var vote *types.Vote
		if i < len(block.Commit.Precommits) {
			vote = block.Commit.Precommits[i]
		}
		val := abci.SigningValidator{
			Validator:       types.TM2PB.Validator(val),
			SignedLastBlock: vote != nil,
		}
		signVals[i] = val
	}

	return signVals

}

// If more or equal than 1/3 of total voting power changed in one block, then
// a light client could never prove the transition externally. See
// ./lite/doc.go for details on how a light client tracks validators.
func updateValidators(currentSet *types.ValidatorSet, abciUpdates []abci.Validator) error {
	updates, err := types.PB2TM.Validators(abciUpdates)
	if err != nil {
		return err
	}

	// these are tendermint types now
	for _, valUpdate := range updates {
		address := valUpdate.Address
		_, val := currentSet.GetByAddress(address)
		if val == nil {
			// add val
			added := currentSet.Add(valUpdate)
			if !added {
				return fmt.Errorf("Failed to add new validator %v", valUpdate)
			}
		} else if valUpdate.VotingPower == 0 {
			// remove val
			_, removed := currentSet.Remove(address)
			if !removed {
				return fmt.Errorf("Failed to remove validator %X", address)
			}
		} else {
			// update val
			updated := currentSet.Update(valUpdate)
			if !updated {
				return fmt.Errorf("Failed to update validator %X to %v", address, valUpdate)
			}
		}
	}
	return nil
}

// updateState returns a new State updated according to the header and responses.
func updateState(state State, blockID types.BlockID, header *types.Header,
	abciResponses *ABCIResponses) (State, error) {

	// copy the valset so we can apply changes from EndBlock
	// and update s.LastValidators and s.Validators
	prevValSet := state.Validators.Copy()
	nextValSet := prevValSet.Copy()

	// update the validator set with the latest abciResponses
	lastHeightValsChanged := state.LastHeightValidatorsChanged
	if len(abciResponses.GetValidatorSet.ValidatorSet) > 0 {
		err := updateValidators(nextValSet, abciResponses.GetValidatorSet.ValidatorSet)
		if err != nil {
			return state, fmt.Errorf("Error changing validator set: %v", err)
		}
		// change results from this height but only applies to the next height
		lastHeightValsChanged = header.Height + 1
	}

	// Update validator accums and set state variables
	nextValSet.IncrementAccum(1)

	// update the params with the latest abciResponses
	nextParams := state.ConsensusParams
	lastHeightParamsChanged := state.LastHeightConsensusParamsChanged
	if abciResponses.EndBlock.ConsensusParamUpdates != nil {
		// NOTE: must not mutate s.ConsensusParams
		nextParams = state.ConsensusParams.Update(abciResponses.EndBlock.ConsensusParamUpdates)
		err := nextParams.Validate()
		if err != nil {
			return state, fmt.Errorf("Error updating consensus params: %v", err)
		}
		// change results from this height but only applies to the next height
		lastHeightParamsChanged = header.Height + 1
	}

	// NOTE: the AppHash has not been populated.
	// It will be filled on state.Save.
	return State{
		ChainID:                          state.ChainID,
		LastBlockHeight:                  header.Height,
		LastBlockTotalTx:                 state.LastBlockTotalTx + header.NumTxs,
		LastBlockID:                      blockID,
		LastBlockTime:                    header.Time,
		Validators:                       nextValSet,
		LastValidators:                   state.Validators.Copy(),
		LastHeightValidatorsChanged:      lastHeightValsChanged,
		ConsensusParams:                  nextParams,
		LastHeightConsensusParamsChanged: lastHeightParamsChanged,
		LastResultsHash:                  abciResponses.ResultsHash(),
		AppHash:                          nil,
	}, nil
}

// Fire NewBlock, NewBlockHeader.
// Fire TxEvent for every tx.
// NOTE: if Tendermint crashes before commit, some or all of these events may be published again.
func fireEvents(logger log.Logger, eventBus types.BlockEventPublisher, block *types.Block, abciResponses *ABCIResponses) {
	eventBus.PublishEventNewBlock(types.EventDataNewBlock{block})
	eventBus.PublishEventNewBlockHeader(types.EventDataNewBlockHeader{block.Header})

	for i, txBucket := range block.Data.TxBuckets {
		for _, tx := range txBucket.Txs {
			eventBus.PublishEventTx(types.EventDataTx{types.TxResult{
				Height: block.Height,
				Index:  uint32(i),
				Tx:     tx,
				Result: *(abciResponses.DeliverTx[i]),
			}})
		}
	}
}

//----------------------------------------------------------------------------------------------------
// Execute block without state. TODO: eliminate

// ExecCommitBlock executes and commits a block on the proxyApp without validating or mutating the state.
// It returns the application root hash (result of abci.Commit).
func ExecCommitBlock(appConnConsensus proxy.AppConnConsensus, block *types.Block,
	logger log.Logger, lastValSet *types.ValidatorSet, stateDB dbm.DB) ([]byte, error) {
	_, err := execBlockOnProxyApp(logger, appConnConsensus, block, lastValSet, stateDB)
	if err != nil {
		logger.Error("Error executing block on proxy app", "height", block.Height, "err", err)
		return nil, err
	}
	// Commit block, get hash back
	res, err := appConnConsensus.CommitSync()
	if err != nil {
		logger.Error("Client error during proxyAppConn.CommitSync", "err", res)
		return nil, err
	}
	// ResponseCommit has no error or log, just data
	return res.Data, nil
}
