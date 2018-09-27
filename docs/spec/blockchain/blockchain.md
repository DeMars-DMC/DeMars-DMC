# DéMars Blockchain

Here we describe the data structures in the DéMars blockchain and the rules for validating them.

## Data Structures

The DéMars blockchains consists of a short list of basic data types:

- `Block`
- `Header`
- `Vote`
- `BlockID`
- `Signature`

## Block

A block consists of a header, a list of transactions (inside buckets), a list of votes (the commit),
and a list of evidence of malfeasance (ie. signing conflicting votes).

```go
type Block struct {
	*Header
	*Data
	Commit *Commit
}

type Data struct {
	TxBuckets TxBuckets

	hash cmn.HexBytes
}
```

## Header

A block header contains metadata about the block and about the consensus, as well as commitments to
the data in the current block, the previous block, and the results returned by the application:

```go
type Header struct {
    // block metadata
    Version             string  // Version string
    ChainID             string  // ID of the chain
    Height              int64   // Current block height
    Time                int64   // UNIX time, in millisconds

    // current block
    NumTxs              int64   // Number of txs in this block
    TxHash              []byte  // SimpleMerkle of the block.Txs
    LastCommitHash      []byte  // SimpleMerkle of the block.LastCommit

    // previous block
    TotalTxs            int64   // prevBlock.TotalTxs + block.NumTxs
    LastBlockID         BlockID // BlockID of prevBlock

    // application
    ResultsHash         []byte  // SimpleMerkle of []abci.Result from prevBlock
    AppHash             []byte  // Arbitrary state digest
    ValidatorsHash      []byte  // SimpleMerkle of the ValidatorSet
    ConsensusParamsHash []byte  // SimpleMerkle of the ConsensusParams

    // consensus
    Proposer            []byte  // Address of the block proposer
}
```

## Vote

A vote is a signed message from a validator for a particular block.
The vote includes information about the validator signing it.

```go
type Vote struct {
    Timestamp   int64
    Address     []byte
    Index       int
    Height      int64
    Round       int
    Type        int8
    BlockID     BlockID
    Signature   Signature
}
```

There are two types of votes:
a *prevote* has `vote.Type == 1` and
a *precommit* has `vote.Type == 2`.

## Signature

DéMars allows for multiple signature schemes to be used by prepending a single type-byte
to the signature bytes. Different signatures may also come with fixed or variable lengths.
Currently, DéMars supports Ed25519 and Secp256k1.

### ED25519

An ED25519 signature has `Type == 0x1`. It looks like:

```go
// Implements Signature
type Ed25519Signature struct {
    Type        int8        = 0x1
    Signature   [64]byte
}
```

where `Signature` is the 64 byte signature.

### Secp256k1

A `Secp256k1` signature has `Type == 0x2`. It looks like:

```go
// Implements Signature
type Secp256k1Signature struct {
    Type        int8        = 0x2
    Signature   []byte
}
```

where `Signature` is the DER encoded signature, ie:

```hex
0x30 <length of whole message> <0x02> <length of R> <R> 0x2 <length of S> <S>.
```

# Execution

Once a block is validated, it can be executed against the state.

The state follows this recursive equation:

```go
state(1) = InitialState
state(h+1) <- Execute(state(h), ABCIApp, block(h))
```

where `InitialState` includes the initial consensus parameters and validator set,
and `ABCIApp` is an ABCI application that can return results and changes to the validator
set (TODO). Execute is defined as:

```go
Execute(s State, app ABCIApp, block Block) State {
    TODO: just spell out ApplyBlock here
    and remove ABCIResponses struct.
    abciResponses := app.ApplyBlock(block)

    return State{
        LastResults: abciResponses.DeliverTxResults,
        AppHash: abciResponses.AppHash,
        Validators: UpdateValidators(state.Validators, abciResponses.ValidatorChanges),
        LastValidators: state.Validators,
        ConsensusParams: UpdateConsensusParams(state.ConsensusParams, abci.Responses.ConsensusParamChanges),
    }
}

type ABCIResponses struct {
    DeliverTxResults        []Result
    ValidatorChanges        []Validator
    ConsensusParamChanges   ConsensusParams
    AppHash                 []byte
}
```
