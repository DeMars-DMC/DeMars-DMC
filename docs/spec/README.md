# DéMars Specification

This is a markdown specification of the DéMars blockchain.
It defines the base data structures, how they are validated,
and how they are communicated over the network.

## Contents

### Data Structures

- [Encoding and Digests](https://github.com/demars-dmc/demars-dmc/docs/spec/blockchain/encoding.md)
- [Blockchain](https://github.com/demars-dmc/demars-dmc/docs/spec/blockchain/blockchain.md)
- [State](https://github.com/demars-dmc/demars-dmc/spec/blockchain/state.md)

### Consensus Protocol

- [Algorithm and Time](/docs/spec/consensus/bft-time.md)


### P2P and Network Protocols

- [The Base P2P Layer](https://github.com/demars-dmc/demars-dmc/docs/spec/p2p): multiplex the protocols ("reactors") on authenticated and encrypted TCP connections
- [Peer Exchange (PEX)](https://github.com/demars-dmc/demars-dmc/docs/spec/reactors/pex): gossip known peer addresses so peers can find each other
- [Block Sync](https://github.com/demars-dmc/demars-dmc/docs/spec/reactors/block_sync): gossip blocks so peers can catch up quickly
- [Consensus](https://github.com/demars-dmc/demars-dmc/docs/spec/reactors/consensus): gossip votes and block parts so new blocks can be committed
- [Mempool](https://github.com/demars-dmc/demars-dmc/spec/reactors/mempool): gossip transactions so they get included in blocks
- Evidence: TODO

### Software

- [ABCI](/abci/README.md): Details about interactions between the
  application and consensus engine over ABCI


## Overview

DéMars provides a Byzantine Fault Tolerant blockchain hosted on a segmented Kademlia network.

Each block in DéMars has a unique index - its Height.
Height's in the blockchain are monotonic.
Each block is committed by a known set of weighted Validators.
Membership and weighting within this validator set may change over time.
DéMars guarantees the safety and liveness of the blockchain
so long as less than 1/3 of the total weight of the Validator set
is malicious or faulty.

A commit in DéMars is a set of signed messages from more than 2/3 of
the total weight of the current Validator set. Validators take turns proposing
blocks and voting on them. Once enough votes are received, the block is considered
committed. These votes are included in the *next* block as proof that the previous block
was committed - they cannot be included in the current block, as that block has already been
created.

Once a block is committed, it can be executed against an application.
The application returns results for each of the transactions in the block.
The application can also return changes to be made to the validator set,
as well as a cryptographic digest of its latest state.

DéMars is designed to enable efficient verification and authentication
of the latest state of the blockchain. To achieve this, it embeds
cryptographic commitments to certain information in the block "header".
This information includes the contents of the block (eg. the transactions),
the validator set committing the block, as well as the various results returned by the application.
Note, however, that block execution only occurs *after* a block is committed.
Thus, application results can only be included in the *next* block
