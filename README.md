# DéMars-DMC

[Hypercube-based](https://en.wikipedia.org/wiki/Hypercube_internetwork_topology) [Byzantine-Fault Tolerant](https://en.wikipedia.org/wiki/Byzantine_fault_tolerance) [Blockchain](https://en.wikipedia.org/wiki/Blockchain_(database))

<!---
[![version](https://img.shields.io/github/tag/Demars-DMC/Demars-DMC.svg)](https://github.com/Demars-DMC/Demars-DMC/releases/latest)
[![API Reference](
https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667
)](https://godoc.org/github.com/Demars-DMC/Demars-DMC)
[![Go version](https://img.shields.io/badge/go-1.9.2-blue.svg)](https://github.com/moovweb/gvm)
[![riot.im](https://img.shields.io/badge/riot.im-JOIN%20CHAT-green.svg)](https://riot.im/app/#/room/#Demars-DMC:matrix.org)
[![license](https://img.shields.io/github/license/Demars-DMC/Demars-DMC.svg)](https://github.com/Demars-DMC/Demars-DMC/blob/master/LICENSE)
[![](https://tokei.rs/b1/github.com/DeMars-DMC/DeMars-DMC?category=lines)](https://github.com/DeMars-DMC/DeMars-DMC)


Branch    | Tests | Coverage
----------|-------|----------
master    | [![CircleCI](https://circleci.com/gh/Demars-DMC/Demars-DMC/tree/master.svg?style=shield)](https://circleci.com/gh/Demars-DMC/Demars-DMC/tree/master) | [![codecov](https://codecov.io/gh/Demars-DMC/Demars-DMC/branch/master/graph/badge.svg)](https://codecov.io/gh/Demars-DMC/Demars-DMC)
develop   | [![CircleCI](https://circleci.com/gh/Demars-DMC/Demars-DMC/tree/develop.svg?style=shield)](https://circleci.com/gh/Demars-DMC/Demars-DMC/tree/develop) | [![codecov](https://codecov.io/gh/Demars-DMC/Demars-DMC/branch/develop/graph/badge.svg)](https://codecov.io/gh/Demars-DMC/Demars-DMC)
-->
DéMars is Byzantine Fault Tolerant (BFT) blockchain which uses network segments in a hypercube geometry to reduce the storage and network transfer costs, thereby enabling it to execute on mobile nodes.

This is only a minimal prototype which has been forked from Tendermint (https://github.com/tendermint/tendermint) and modified to use Kademlia XOR metric. The proof of concept is under development.

## Simplifications (w.r.t. the Whitepaper)
* The prototype uses a Kademlia XOR P2P network similar to Ethereum (https://github.com/ethereum/wiki/wiki/Kademlia-Peer-Selection). We propose to implement a Hamming distance-based hypercube DHT for optimizing zone selection.
* The validators and proposer are chosen naively (the top 100 nodes in a zone based on account balances). The final version will be based on cryptographic sortition like Algorand.
* The distribution condition to ensure that adversaries cannot concentrate wealth in zones is yet to be implemented.

<!--
For protocol details, see [the specification](/docs/spec).

## A Note on Production Readiness

While Demars-DMC is being used in production in private, permissioned
environments, we are still working actively to harden and audit it in preparation
for use in public blockchains, such as the [Cosmos Network](https://cosmos.network/).
We are also still making breaking changes to the protocol and the APIs.
Thus we tag the releases as *alpha software*.

In any case, if you intend to run Demars-DMC in production,
please [contact us](https://riot.im/app/#/room/#Demars-DMC:matrix.org) :)

## Security

To report a security vulnerability, see our [bug bounty
program](https://Demars-DMC.com/security).

For examples of the kinds of bugs we're looking for, see [SECURITY.md](SECURITY.md)
-->
## Minimum requirements

Requirement|Notes
---|---
Go version | Go1.9 or higher

## Install

See the [install instructions](/docs/install.md)

<!--
## Quick Start

- [Single node](/docs/using-Demars-DMC.md)
- [Local cluster using docker-compose](/networks/local)
- [Remote cluster using terraform and ansible](/docs/terraform-and-ansible.md)
- [Join the public testnet](https://cosmos.network/testnet)
-->
## Resources

### DéMars

For details about the blockchain data structures and the p2p protocols, see the
the [DMC specification](/docs/spec).

<!--
For details on using the software, [Read The Docs](https://Demars-DMC.readthedocs.io/en/master/).
Additional information about some - and eventually all - of the sub-projects below, can be found at Read The Docs.


### Sub-projects

* [Amino](http://github.com/tendermint/go-amino), a reflection-based improvement on proto3
* [IAVL](http://github.com/Demars-DMC/iavl), Merkleized IAVL+ Tree implementation

### Tools
* [Deployment, Benchmarking, and Monitoring](http://Demars-DMC.readthedocs.io/projects/tools/en/develop/index.html#Demars-DMC-tools)

### Applications

* [Cosmos SDK](http://github.com/cosmos/cosmos-sdk); a cryptocurrency application framework
* [Ethermint](http://github.com/Demars-DMC/ethermint); Ethereum on Demars-DMC
* [Many more](https://Demars-DMC.readthedocs.io/en/master/ecosystem.html#abci-applications)
-->
### More

<!--
* [Master's Thesis on Demars-DMC](https://atrium.lib.uoguelph.ca/xmlui/handle/10214/9769)
* -->

* [Original Whitepaper](Please contact us for more information: dmc@demars.io)

<!--
* [Demars-DMC Blog](https://blog.cosmos.network/Demars-DMC/home)
* [Cosmos Blog](https://blog.cosmos.network)

## Contributing

Yay open source! Please see our [contributing guidelines](CONTRIBUTING.md).
-->
<!--
## Versioning

### SemVer

Demars-DMC uses [SemVer](http://semver.org/) to determine when and how the version changes.
According to SemVer, anything in the public API can change at any time before version 1.0.0

To provide some stability to Demars-DMC users in these 0.X.X days, the MINOR version is used
to signal breaking changes across a subset of the total public API. This subset includes all
interfaces exposed to other processes (cli, rpc, p2p, etc.), but does not
include the in-process Go APIs.

That said, breaking changes in the following packages will be documented in the
CHANGELOG even if they don't lead to MINOR version bumps:

- types
- rpc/client
- config
- node

Exported objects in these packages that are not covered by the versioning scheme
are explicitly marked by `// UNSTABLE` in their go doc comment and may change at any
time without notice. Functions, types, and values in any other package may also change at any time.

### Upgrades

In an effort to avoid accumulating technical debt prior to 1.0.0,
we do not guarantee that breaking changes (ie. bumps in the MINOR version)
will work with existing Demars-DMC blockchains. In these cases you will
have to start a new blockchain, or write something custom to get the old
data into the new chain.

However, any bump in the PATCH version should be compatible with existing histories
(if not please open an [issue](https://github.com/Demars-DMC/Demars-DMC/issues)).

## Code of Conduct

Please read, understand and adhere to our [code of conduct](CODE_OF_CONDUCT.md).
-->
