# Mempool Configuration

Here we describe configuration options around mempool.
For the purposes of this document, they are described
as command-line flags, but they can also be passed in as
environmental variables or in the config.toml file. The
following are all equivalent:

Flag: `--mempool.recheck_empty=false`

Environment: `DM_MEMPOOL_RECHECK_EMPTY=false`

Config:
```
[mempool]
recheck_empty = false
```


## Recheck

`--mempool.recheck=false` (default: true)

`--mempool.recheck_empty=false` (default: true)

Recheck determines if the mempool rechecks all pending
transactions after a block was committed. Once a block
is committed, the mempool removes all valid transactions
that were successfully included in the block.

If `recheck` is true, then it will rerun CheckTx on
all remaining transactions with the new block state.

If the block contained no transactions, it will skip the
recheck unless `recheck_empty` is true.

## Broadcast

`--mempool.broadcast=false` (default: true)

Determines whether this node gossips any valid transactions
that arrive in mempool. Default is to gossip anything that
passes checktx. If this is disabled, transactions are not
gossiped, but instead stored locally and added to the next
block this node is the proposer.
