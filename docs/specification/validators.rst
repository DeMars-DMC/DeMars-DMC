Validators
==========

Validators are responsible for committing new blocks in the blockchain.
These validators participate in the consensus protocol by broadcasting
*votes* which contain cryptographic signatures signed by each
validator's private key.

Some Proof-of-Stake consensus algorithms aim to create a "completely"
decentralized system where all stakeholders (even those who are not
always available online) participate in the committing of blocks. Others
have a fixed set of validators which can make them potential targets.
DéMars has a different approach to block creation. Validators are
expected to be online, and the set of validators is determined for every
block.

Validators have a cryptographic key-pair and equal voting power (1 vote).

Becoming a Validator
--------------------

There are two ways to become validator.

1. The validators for the first block are pre-established in the `genesis
   state <./genesis.html>`__
2. For further blocks, the ABCI app responds to the GetValidatorSet message
 with the new validator set.

Committing a Block
------------------

*+2/3 is short for "more than 2/3"*

A block is committed when +2/3 of the validator set sign `precommit
votes <./block-structure.html#vote>`__ for that block at the same
``round``. The +2/3 set of precommit votes is
called a `*commit* <./block-structure.html#commit>`__. While any
+2/3 set of precommits for the same block at the same height&round can
serve as validation, the canonical commit is included in the next block
(see `LastCommit <./block-structure.html>`__).
