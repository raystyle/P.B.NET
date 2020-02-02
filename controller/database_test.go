package controller

import (
	"bytes"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/xnet"

	"project/testdata"
)

func TestInsertProxyClient(t *testing.T) {
	testInitializeController(t)
	testInsertProxyClient(t)
}

func testInsertProxyClient(t require.TestingT) {
	// clean table
	err := ctrl.database.db.Unscoped().Delete(&mProxyClient{}).Error
	require.NoError(t, err)
	// insert
	for _, client := range testdata.ProxyClients(t) {
		m := &mProxyClient{
			Tag:     client.Tag,
			Mode:    client.Mode,
			Network: client.Network,
			Address: client.Address,
			Options: client.Options,
		}
		err := ctrl.database.InsertProxyClient(m)
		require.NoError(t, err)
	}
}

func TestSelectProxyClient(t *testing.T) {
	testInitializeController(t)
	clients, err := ctrl.database.SelectProxyClient()
	require.NoError(t, err)
	t.Log("select proxy client:", spew.Sdump(clients))
}

func TestUpdateProxyClient(t *testing.T) {
	testInitializeController(t)
	clients, err := ctrl.database.SelectProxyClient()
	require.NoError(t, err)
	raw := clients[0].Mode
	clients[0].Mode = "changed"
	err = ctrl.database.UpdateProxyClient(clients[0])
	require.NoError(t, err)
	clients[0].Mode = raw
	err = ctrl.database.UpdateProxyClient(clients[0])
	require.NoError(t, err)
}

func TestDeleteProxyClient(t *testing.T) {
	testInitializeController(t)
	clients, err := ctrl.database.SelectProxyClient()
	require.NoError(t, err)
	err = ctrl.database.DeleteProxyClient(clients[0].ID)
	require.NoError(t, err)
	TestInsertProxyClient(t)
}

func TestInsertDNSServer(t *testing.T) {
	testInitializeController(t)
	testInsertDNSServer(t)
}

func testInsertDNSServer(t require.TestingT) {
	// clean table
	err := ctrl.database.db.Unscoped().Delete(&mDNSServer{}).Error
	require.NoError(t, err)
	// insert
	for tag, server := range testdata.DNSServers() {
		m := &mDNSServer{
			Tag:      tag,
			Method:   server.Method,
			Address:  server.Address,
			SkipTest: server.SkipTest,
		}
		err := ctrl.database.InsertDNSServer(m)
		require.NoError(t, err)
	}
}

func TestSelectDNSServer(t *testing.T) {
	testInitializeController(t)
	servers, err := ctrl.database.SelectDNSServer()
	require.NoError(t, err)
	t.Log("select DNS server:", spew.Sdump(servers))
}

func TestUpdateDNSServer(t *testing.T) {
	testInitializeController(t)
	servers, err := ctrl.database.SelectDNSServer()
	require.NoError(t, err)
	raw := servers[0].Method
	servers[0].Method = "changed"
	err = ctrl.database.UpdateDNSServer(servers[0])
	require.NoError(t, err)
	servers[0].Method = raw
	err = ctrl.database.UpdateDNSServer(servers[0])
	require.NoError(t, err)
}

func TestDeleteDNSServer(t *testing.T) {
	testInitializeController(t)
	servers, err := ctrl.database.SelectDNSServer()
	require.NoError(t, err)
	err = ctrl.database.DeleteDNSServer(servers[0].ID)
	require.NoError(t, err)
	TestInsertDNSServer(t)
}

func TestInsertTimeSyncerClient(t *testing.T) {
	testInitializeController(t)
	testInsertTimeSyncerClient(t)
}

func testInsertTimeSyncerClient(t require.TestingT) {
	// clean table
	err := ctrl.database.db.Unscoped().Delete(&mTimeSyncer{}).Error
	require.NoError(t, err)
	// insert
	for tag, client := range testdata.TimeSyncerClients() {
		m := &mTimeSyncer{
			Tag:      tag,
			Mode:     client.Mode,
			Config:   client.Config,
			SkipTest: client.SkipTest,
		}
		err = ctrl.database.InsertTimeSyncerClient(m)
		require.NoError(t, err)
	}
}

func TestSelectTimeSyncerClient(t *testing.T) {
	testInitializeController(t)
	clients, err := ctrl.database.SelectTimeSyncerClient()
	require.NoError(t, err)
	t.Log("select time syncer client:", spew.Sdump(clients))
}

func TestUpdateTimeSyncerClient(t *testing.T) {
	testInitializeController(t)
	configs, err := ctrl.database.SelectTimeSyncerClient()
	require.NoError(t, err)
	raw := configs[0].Mode
	configs[0].Mode = "changed"
	err = ctrl.database.UpdateTimeSyncerClient(configs[0])
	require.NoError(t, err)
	configs[0].Mode = raw
	err = ctrl.database.UpdateTimeSyncerClient(configs[0])
	require.NoError(t, err)
}

func TestDeleteTimeSyncerClient(t *testing.T) {
	testInitializeController(t)
	configs, err := ctrl.database.SelectTimeSyncerClient()
	require.NoError(t, err)
	err = ctrl.database.DeleteTimeSyncerClient(configs[0].ID)
	require.NoError(t, err)
	TestInsertTimeSyncerClient(t)
}

func TestInsertBoot(t *testing.T) {
	testInitializeController(t)
	testInsertBoot(t)
}

func testInsertBoot(t require.TestingT) {
	// clean table
	err := ctrl.database.db.Unscoped().Delete(&mBoot{}).Error
	require.NoError(t, err)
	// insert
	b := testdata.Bootstrap(t)
	for i := 0; i < len(b); i++ {
		m := &mBoot{
			Tag:      b[i].Tag,
			Mode:     b[i].Mode,
			Config:   string(b[i].Config),
			Interval: uint32(15),
		}
		if m.Mode == bootstrap.ModeDirect {
			m.Enable = true
		}
		err := ctrl.database.InsertBoot(m)
		require.NoError(t, err)
	}
}

func TestSelectBoot(t *testing.T) {
	testInitializeController(t)
	boots, err := ctrl.database.SelectBoot()
	require.NoError(t, err)
	t.Log("select boot:", spew.Sdump(boots))
}

func TestUpdateBoot(t *testing.T) {
	testInitializeController(t)
	boots, err := ctrl.database.SelectBoot()
	require.NoError(t, err)
	raw := boots[0].Mode
	boots[0].Mode = "changed"
	err = ctrl.database.UpdateBoot(boots[0])
	require.NoError(t, err)
	boots[0].Mode = raw
	err = ctrl.database.UpdateBoot(boots[0])
	require.NoError(t, err)
}

func TestDeleteBoot(t *testing.T) {
	testInitializeController(t)
	boots, err := ctrl.database.SelectBoot()
	require.NoError(t, err)
	err = ctrl.database.DeleteBoot(boots[0].ID)
	require.NoError(t, err)
	TestInsertBoot(t)
}

func TestInsertListener(t *testing.T) {
	testInitializeController(t)
	testInsertListener(t)
}

func testInsertListener(t require.TestingT) {
	// clean table
	err := ctrl.database.db.Unscoped().Delete(&mListener{}).Error
	require.NoError(t, err)
	// insert
	for _, listener := range testdata.Listeners(t) {
		m := &mListener{
			Tag:  listener.Tag,
			Mode: listener.Mode,
			// Config: string(listeners[i].Config),
		}
		err := ctrl.database.InsertListener(m)
		require.NoError(t, err)
	}
}

func TestSelectListener(t *testing.T) {
	testInitializeController(t)
	listeners, err := ctrl.database.SelectListener()
	require.NoError(t, err)
	t.Log("select listener:", spew.Sdump(listeners))
}

func TestUpdateListener(t *testing.T) {
	testInitializeController(t)
	listeners, err := ctrl.database.SelectListener()
	require.NoError(t, err)
	raw := listeners[0].Mode
	listeners[0].Mode = "changed"
	err = ctrl.database.UpdateListener(listeners[0])
	require.NoError(t, err)
	listeners[0].Mode = raw
	err = ctrl.database.UpdateListener(listeners[0])
	require.NoError(t, err)
}

func TestDeleteListener(t *testing.T) {
	testInitializeController(t)
	listeners, err := ctrl.database.SelectListener()
	require.NoError(t, err)
	err = ctrl.database.DeleteListener(listeners[0].ID)
	require.NoError(t, err)
	TestInsertListener(t)
}

func TestInsertNode(t *testing.T) {
	testInitializeController(t)
	node := &mNode{
		SessionKey: bytes.Repeat([]byte{48}, aes.Key256Bit),
		PublicKey:  bytes.Repeat([]byte{48}, ed25519.PublicKeySize),
	}
	copy(node.GUID[:], bytes.Repeat([]byte{48}, guid.Size))
	err := ctrl.database.db.Unscoped().Delete(node).Error
	require.NoError(t, err)
	err = ctrl.database.InsertNode(node)
	require.NoError(t, err)
	// insert listener
	nl := &mNodeListener{
		GUID:    node.GUID,
		Tag:     "tls_1",
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "127.0.0.1:1234",
	}
	err = ctrl.database.InsertNodeListener(nl)
	require.NoError(t, err)
	nl = &mNodeListener{
		GUID:    node.GUID,
		Tag:     "tls_2",
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "127.0.0.1:1235",
	}
	err = ctrl.database.InsertNodeListener(nl)
	require.NoError(t, err)
	// insert log
	lg := &mRoleLog{
		GUID:   node.GUID,
		Level:  logger.Debug,
		Source: "test",
		Log:    "test log",
	}
	err = ctrl.database.InsertNodeLog(lg)
	require.NoError(t, err)
}

func TestDeleteNode(t *testing.T) {
	testInitializeController(t)
	g := guid.GUID{}
	copy(g[:], bytes.Repeat([]byte{48}, guid.Size))
	err := ctrl.database.DeleteNode(&g)
	require.NoError(t, err)
}

func TestDeleteNodeUnscoped(t *testing.T) {
	testInitializeController(t)
	g := guid.GUID{}
	copy(g[:], bytes.Repeat([]byte{48}, guid.Size))
	err := ctrl.database.DeleteNodeUnscoped(&g)
	require.NoError(t, err)
}
