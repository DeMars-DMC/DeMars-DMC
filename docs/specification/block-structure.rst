Block Structure
===============

The DéMars consensus engine records all agreements by a
supermajority of nodes into a blockchain, which is replicated in segments.

Block
~~~~~

A
`Block <https://godoc.org/github.com/demars-dmc/demars-dmc/types#Block>`__
contains:

-  a `Header <#header>`__ contains merkle hashes for various chain
   states
-  the
   `Data <https://godoc.org/github.com/demars-dmc/demars-dmc/types#Data>`__
   is all transactions which are to be processed, stored inside buckets.
-  the `LastCommit <#commit>`__ > 2/3 signatures for the last block


Commit
~~~~~~

The
`Commit <https://godoc.org/github.com/demars-dmc/demars-dmc/types#Commit>`__
contains a set of
`Votes <https://godoc.org/github.com/demars-dmc/demars-dmc/types#Vote>`__
that were made by the validator set to reach consensus on this block.
This is the key to the security in any PoS system, and actually no data
that cannot be traced back to a block header with a valid set of Votes
can be trusted. Thus, getting the Commit data and verifying the votes is
extremely important.

As mentioned above, in order to find the ``precommit votes`` for block
header ``H``, we need to query block ``H+1``. Then we need to check the
votes, make sure they really are for that block, and properly formatted.

Transaction
~~~~~~~~~~~

A transaction is any sequence of bytes. It is up to your
ABCI application to accept or reject transactions.
