package test

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/protocol"
	"project/internal/testsuite"

	"project/beacon"
	"project/node"
)

func TestNode_Client_Send(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t)
	iNodeGUID := iNode.GUID()

	// create bootstrap
	iListener, err := iNode.GetListener(InitialNodeListenerTag)
	require.NoError(t, err)
	iAddr := iListener.Addr()
	bListener := &bootstrap.Listener{
		Mode:    iListener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}
	boot, key := generateBootstrap(t, bListener)

	// create common node
	cNodeCfg := generateNodeConfig(t, "Common Node")
	cNodeCfg.Register.FirstBoot = boot
	cNodeCfg.Register.FirstKey = key

	ctrl.Test.CreateNodeRegisterRequestChannel()

	// run common node
	cNode, err := node.New(cNodeCfg)
	require.NoError(t, err)
	go func() {
		err := cNode.Main()
		require.NoError(t, err)
	}()

	// read Node register request
	select {
	case nrr := <-ctrl.Test.NodeRegisterRequest:
		spew.Dump(nrr)
		err = ctrl.AcceptRegisterNode(nrr, false)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read Ctrl.Test.NodeRegisterRequest timeout")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("node register timeout")
	})
	cNode.Wait()
	timer.Stop()

	// try to connect initial node and start to synchronize
	client, err := cNode.NewClient(context.Background(), bListener, iNodeGUID)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)
	err = client.Synchronize()
	require.NoError(t, err)

	t.Run("single", func(t *testing.T) {
		data := bytes.Buffer{}
		for i := 0; i < 16384; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := client.Conn.SendCommand(protocol.TestCommand, data.Bytes())
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
				reply, err := client.Conn.SendCommand(protocol.TestCommand, data.Bytes())
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

	// clean
	cNode.Exit(nil)
	testsuite.IsDestroyed(t, cNode)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
}

func TestBeacon_Client_Send(t *testing.T) {
	iNode := generateInitialNodeAndTrust(t)
	iNodeGUID := iNode.GUID()

	// create bootstrap
	iListener, err := iNode.GetListener(InitialNodeListenerTag)
	require.NoError(t, err)
	iAddr := iListener.Addr()
	bListener := &bootstrap.Listener{
		Mode:    iListener.Mode(),
		Network: iAddr.Network(),
		Address: iAddr.String(),
	}
	boot, key := generateBootstrap(t, bListener)
	ctrl.Test.CreateBeaconRegisterRequestChannel()

	// create Beacon
	beaconCfg := generateBeaconConfig(t, "Beacon")
	beaconCfg.Register.FirstBoot = boot
	beaconCfg.Register.FirstKey = key

	// run Beacon
	Beacon, err := beacon.New(beaconCfg)
	require.NoError(t, err)
	go func() {
		err := Beacon.Main()
		require.NoError(t, err)
	}()

	// read Beacon register request
	select {
	case brr := <-ctrl.Test.BeaconRegisterRequest:
		spew.Dump(brr)
		err = ctrl.AcceptRegisterBeacon(brr)
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("read Ctrl.Test.BeaconRegisterRequest timeout")
	}

	timer := time.AfterFunc(10*time.Second, func() {
		t.Fatal("beacon register timeout")
	})
	Beacon.Wait()
	timer.Stop()

	// try to connect initial node and start to synchronize
	client, err := Beacon.NewClient(context.Background(), bListener, iNodeGUID, nil)
	require.NoError(t, err)
	err = client.Connect()
	require.NoError(t, err)
	err = client.Synchronize()
	require.NoError(t, err)

	t.Run("single", func(t *testing.T) {
		data := bytes.Buffer{}
		for i := 0; i < 16384; i++ {
			data.Write(convert.Int32ToBytes(int32(i)))
			reply, err := client.SendCommand(protocol.TestCommand, data.Bytes())
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
				reply, err := client.SendCommand(protocol.TestCommand, data.Bytes())
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

	// clean
	Beacon.Exit(nil)
	testsuite.IsDestroyed(t, Beacon)
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
}
