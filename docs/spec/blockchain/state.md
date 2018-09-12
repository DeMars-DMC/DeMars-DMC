# DéMars State

## State

The state contains information whose cryptographic digest is included in block headers, and thus is
necessary for validating new blocks. For instance, the set of validators and the results of
transactions are never included in blocks, but their hashes are - the state keeps track of them.

Note that the `State` object itself is an implementation detail, since it is never
included in a block or gossipped over the network, and we never compute
its hash.


```go
type State struct {
	LastBlockHeight  int64
	LastBlockTotalTx int64
	LastBlockID      types.BlockID
	LastBlockTime    time.Time

	Validators                  *types.ValidatorSet
	LastValidators              *types.ValidatorSet
	LastHeightValidatorsChanged int64

	ConsensusParams                  types.ConsensusParams
	LastHeightConsensusParamsChanged int64

	LastResultsHash []byte

	AppHash []byte
}
```

### Validator

A validator is an active participant in the consensus with a public key and a voting power.
Validator's also contain an address which is derived from the PubKey:

```go
type Validator struct {
    Address     []byte
    PubKey      PubKey
    Balance int64
}
```


### ConsensusParams

TODO
