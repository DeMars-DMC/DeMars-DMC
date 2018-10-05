package client_test

import (
	"os"
	"testing"

	"github.com/Demars-DMC/Demars-DMC/abci/example/kvstore"
	nm "github.com/Demars-DMC/Demars-DMC/node"
	rpctest "github.com/Demars-DMC/Demars-DMC/rpc/test"
)

var node *nm.Node

func TestMain(m *testing.M) {
	// start a Demars-DMC node (and kvstore) in the background to test against
	app := kvstore.NewKVStoreApplication()
	node = rpctest.StartDemars-DMC(app)
	code := m.Run()

	// and shut down proper at the end
	node.Stop()
	node.Wait()
	os.Exit(code)
}
