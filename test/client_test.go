package test

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/protocol"
	"project/internal/testsuite"
)

func testClientSendCommand(t *testing.T, send func(cmd uint8, data []byte) ([]byte, error)) {
	t.Run("single", func(t *testing.T) {
		data := bytes.Buffer{}
		for i := 0; i < 16384; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := send(protocol.TestCommand, data.Bytes())
			require.NoError(t, err)
			require.Equal(t, data.Bytes(), reply)
			data.Reset()
		}
	})

	t.Run("parallel", func(t *testing.T) {
		wg := sync.WaitGroup{}
		send := func() {
			defer wg.Done()
			data := bytes.Buffer{}
			for i := 0; i < 32; i++ {
				data.Write(convert.Int32ToBytes(int32(i)))
				reply, err := send(protocol.TestCommand, data.Bytes())
				require.NoError(t, err)
				require.Equal(t, data.Bytes(), reply)
				data.Reset()
			}
		}
		for i := 0; i < 2*protocol.SlotSize; i++ {
			wg.Add(1)
			go send()
		}
		wg.Wait()
	})
}

func TestCtrl_Client_Send(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t)
	iNodeGUID := iNode.GUID()

	iListener, err := iNode.GetListener(InitialNodeListenerTag)
	require.NoError(t, err)
	iAddr := iListener.Addr()
	bListener := &bootstrap.Listener{
		Mode:    iListener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}

	// try to connect Initial Node and start to synchronize
	client, err := ctrl.NewClient(context.Background(), bListener, iNodeGUID, nil)
	require.NoError(t, err)
	err = client.Synchronize()
	require.NoError(t, err)

	testClientSendCommand(t, client.SendCommand)

	// clean
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}

func TestNode_Client_Send(t *testing.T) {
	iNode, bListener, cNode := generateInitialNodeAndCommonNode(t)
	iNodeGUID := iNode.GUID()
	cNodeGUID := cNode.GUID()

	// try to connect Initial Node and start to synchronize
	client, err := cNode.NewClient(context.Background(), bListener, iNodeGUID)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)
	err = client.Synchronize()
	require.NoError(t, err)

	testClientSendCommand(t, client.Conn.SendCommand)

	// clean
	cNode.Exit(nil)
	testsuite.IsDestroyed(t, cNode)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteNodeUnscoped(cNodeGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}

func TestBeacon_Client_Send(t *testing.T) {
	iNode, bListener, Beacon := generateInitialNodeAndBeacon(t)
	iNodeGUID := iNode.GUID()
	beaconGUID := Beacon.GUID()

	// try to connect Initial Node and start to synchronize
	client, err := Beacon.NewClient(context.Background(), bListener, iNodeGUID, nil)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)
	err = client.Synchronize()
	require.NoError(t, err)

	testClientSendCommand(t, client.SendCommand)

	// clean
	Beacon.Exit(nil)
	testsuite.IsDestroyed(t, Beacon)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)

	err = ctrl.DeleteBeaconUnscoped(beaconGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}
