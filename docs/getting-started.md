# Getting Started
TODO
<!--
Project DéMars as a platform to build various apps?

## First DéMars App

As a general purpose blockchain engine, Tendermint is agnostic to the
application you want to run. So, to run a complete blockchain that does
something useful, you must start two programs: one is Tendermint Core,
the other is your application, which can be written in any programming
language. Recall from [the intro to
ABCI](./introduction.md#ABCI-Overview) that Tendermint Core handles all
the p2p and consensus stuff, and just forwards transactions to the
application when they need to be validated, or when they're ready to be
committed to a block.

In this guide, we show you some examples of how to run an application
using Tendermint.

### Install

The first apps we will work with are written in Go. To install them, you
need to [install Go](https://golang.org/doc/install) and put
`$GOPATH/bin` in your `$PATH`; see
[here](https://github.com/tendermint/tendermint/wiki/Setting-GOPATH) for
more info.

Then run

    go get github.com/tendermint/tendermint
    cd $GOPATH/src/github.com/tendermint/tendermint
    make get_tools
    make get_vendor_deps
    make install_abci

Now you should have the `abci-cli` installed; you'll see a couple of
commands (`counter` and `kvstore`) that are example applications written
in Go. See below for an application written in JavaScript.

Now, let's run some apps!

## KVStore - A First Example

The kvstore app is a [Merkle
tree](https://en.wikipedia.org/wiki/Merkle_tree) that just stores all
transactions. If the transaction contains an `=`, e.g. `key=value`, then
the `value` is stored under the `key` in the Merkle tree. Otherwise, the
full transaction bytes are stored as the key and the value.

Let's start a kvstore application.

    abci-cli kvstore

In another terminal, we can start Tendermint. If you have never run
Tendermint before, use:

    tendermint init
    tendermint node

If you have used Tendermint, you may want to reset the data for a new
blockchain by running `tendermint unsafe_reset_all`. Then you can run
`tendermint node` to start Tendermint, and connect to the app. For more
details, see [the guide on using Tendermint](./using-tendermint.md).

You should see Tendermint making blocks! We can get the status of our
Tendermint node as follows:

    curl -s localhost:26657/status

The `-s` just silences `curl`. For nicer output, pipe the result into a
tool like [jq](https://stedolan.github.io/jq/) or `json_pp`.

Now let's send some transactions to the kvstore.

    curl -s 'localhost:26657/broadcast_tx_commit?tx="abcd"'

Note the single quote (`'`) around the url, which ensures that the
double quotes (`"`) are not escaped by bash. This command sent a
transaction with bytes `abcd`, so `abcd` will be stored as both the key
and the value in the Merkle tree. The response should look something
like:

    {
      "jsonrpc": "2.0",
      "id": "",
      "result": {
        "check_tx": {
          "fee": {}
        },
        "deliver_tx": {
          "tags": [
            {
              "key": "YXBwLmNyZWF0b3I=",
              "value": "amFl"
            },
            {
              "key": "YXBwLmtleQ==",
              "value": "YWJjZA=="
            }
          ],
          "fee": {}
        },
        "hash": "9DF66553F98DE3C26E3C3317A3E4CED54F714E39",
        "height": 14
      }
    }

We can confirm that our transaction worked and the value got stored by
querying the app:

    curl -s 'localhost:26657/abci_query?data="abcd"'

The result should look like:

    {
      "jsonrpc": "2.0",
      "id": "",
      "result": {
        "response": {
          "log": "exists",
          "index": "-1",
          "key": "YWJjZA==",
          "value": "YWJjZA=="
        }
      }
    }

Note the `value` in the result (`YWJjZA==`); this is the base64-encoding
of the ASCII of `abcd`. You can verify this in a python 2 shell by
running `"YWJjZA==".decode('base64')` or in python 3 shell by running
`import codecs; codecs.decode("YWJjZA==", 'base64').decode('ascii')`.
Stay tuned for a future release that [makes this output more
human-readable](https://github.com/tendermint/tendermint/issues/1794).

Now let's try setting a different key and value:

    curl -s 'localhost:26657/broadcast_tx_commit?tx="name=satoshi"'

Now if we query for `name`, we should get `satoshi`, or `c2F0b3NoaQ==`
in base64:

    curl -s 'localhost:26657/abci_query?data="name"'

Try some other transactions and queries to make sure everything is
working!

-->
