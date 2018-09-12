# Deploy a Testnet

Now that we've seen how ABCI works, and even played with a few
applications on a single validator node, it's time to deploy a test
network to four validator nodes.

## Manual Deployments

It's relatively easy to setup a DéMars cluster manually. The only
requirements for a particular DéMars node are a private key for the
validator, stored as `priv_validator.json`, a node key, stored as
`node_key.json` and a list of the public keys of all validators, stored
as `genesis.json`. These files should be stored in
`~/.tendermint/config`, or wherever the `$TMHOME` variable might be set
to.

Here are the steps to setting up a testnet manually:

1)  Provision nodes on your cloud provider of choice
2)  Install DéMars and the DMCCoin app on all nodes
3)  Generate a private key and a node key for each validator using
    `demars init`
4)  Compile a list of public keys for each validator into a
    `genesis.json` file and replace the existing file with it.
5)  Run
    `demars node --proxy_app=dmccoin --p2p.persistent_peers=< peer addresses >`
    on each node, where `< peer addresses >` is a comma separated list
    of the IP:PORT combination for each node. The default port for
    Tendermint is `26656`. Thus, if the IP addresses of your nodes were
    `192.168.0.1, 192.168.0.2, 192.168.0.3, 192.168.0.4`, the command
    would look like:


    demars node --proxy_app=dmccoin --p2p.persistent_peers=96663a3dd0d7b9d17d4c8211b191af259621c693@192.168.0.1:26656, 429fcf25974313b95673f58d77eacdd434402665@192.168.0.2:26656, 0491d373a8e0fcf1023aaf18c51d6a1d0d4f31bd@192.168.0.3:26656, f9baeaa15fedf5e1ef7448dd60f46c01f1a9e9c4@192.168.0.4:26656

After a few seconds, all the nodes should connect to each other and
start making blocks! For more information, see the Tendermint Networks
section of [the guide to using DéMars](./using-demars.md).
