package core_grpc_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Demars-DMC/Demars-DMC/abci/example/kvstore"
	"github.com/Demars-DMC/Demars-DMC/rpc/grpc"
	"github.com/Demars-DMC/Demars-DMC/rpc/test"
)

func TestMain(m *testing.M) {
	// start a Demars-DMC node in the background to test against
	app := kvstore.NewKVStoreApplication()
	node := rpctest.StartDemars-DMC(app)
	code := m.Run()

	// and shut down proper at the end
	node.Stop()
	node.Wait()
	os.Exit(code)
}

func TestBroadcastTx(t *testing.T) {
	require := require.New(t)
	res, err := rpctest.GetGRPCClient().BroadcastTx(context.Background(), &core_grpc.RequestBroadcastTx{[]byte("this is a tx")})
	require.Nil(err, "%+v", err)
	require.EqualValues(0, res.CheckTx.Code)
	require.EqualValues(0, res.DeliverTx.Code)
}
