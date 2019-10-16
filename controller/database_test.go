package controller

import (
	"bytes"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/testdata"
	"project/internal/xnet"
)

func TestInsertProxyClient(t *testing.T) {
	testInitCtrl(t)
	testInsertProxyClient(t)
}

func testInsertProxyClient(t require.TestingT) {
	// clean table
	err := ctrl.db.db.Unscoped().Delete(&mProxyClient{}).Error
	require.NoError(t, err)
	// insert
	clients := testdata.ProxyClients(t)
	for tag, client := range clients {
		m := &mProxyClient{
			Tag:    "test_" + tag,
			Mode:   client.Mode,
			Config: client.Config,
		}
		err := ctrl.db.InsertProxyClient(m)
		require.NoError(t, err)
	}
}

func TestSelectProxyClient(t *testing.T) {
	testInitCtrl(t)
	clients, err := ctrl.db.SelectProxyClient()
	require.NoError(t, err)
	t.Log("select proxy client:", spew.Sdump(clients))
}

func TestUpdateProxyClient(t *testing.T) {
	testInitCtrl(t)
	clients, err := ctrl.db.SelectProxyClient()
	require.NoError(t, err)
	raw := clients[0].Mode
	clients[0].Mode = "changed"
	err = ctrl.db.UpdateProxyClient(clients[0])
	require.NoError(t, err)
	clients[0].Mode = raw
	err = ctrl.db.UpdateProxyClient(clients[0])
	require.NoError(t, err)
}

func TestDeleteProxyClient(t *testing.T) {
	testInitCtrl(t)
	clients, err := ctrl.db.SelectProxyClient()
	require.NoError(t, err)
	err = ctrl.db.DeleteProxyClient(clients[0].ID)
	require.NoError(t, err)
	TestInsertProxyClient(t)
}

func TestInsertDNSServer(t *testing.T) {
	testInitCtrl(t)
	testInsertDNSServer(t)
}

func testInsertDNSServer(t require.TestingT) {
	// clean table
	err := ctrl.db.db.Unscoped().Delete(&mDNSServer{}).Error
	require.NoError(t, err)
	// insert
	servers := testdata.DNSServers(t)
	for tag, server := range servers {
		m := &mDNSServer{
			Tag:     "test_" + tag,
			Method:  server.Method,
			Address: server.Address,
		}
		err := ctrl.db.InsertDNSServer(m)
		require.NoError(t, err)
	}
}

func TestSelectDNSServer(t *testing.T) {
	testInitCtrl(t)
	servers, err := ctrl.db.SelectDNSServer()
	require.NoError(t, err)
	t.Log("select dns client:", spew.Sdump(servers))
}

func TestUpdateDNSServer(t *testing.T) {
	testInitCtrl(t)
	servers, err := ctrl.db.SelectDNSServer()
	require.NoError(t, err)
	raw := servers[0].Method
	servers[0].Method = "changed"
	err = ctrl.db.UpdateDNSServer(servers[0])
	require.NoError(t, err)
	servers[0].Method = raw
	err = ctrl.db.UpdateDNSServer(servers[0])
	require.NoError(t, err)
}

func TestDeleteDNSServer(t *testing.T) {
	testInitCtrl(t)
	servers, err := ctrl.db.SelectDNSServer()
	require.NoError(t, err)
	err = ctrl.db.DeleteDNSServer(servers[0].ID)
	require.NoError(t, err)
	TestInsertDNSServer(t)
}

func TestInsertTimeSyncerConfig(t *testing.T) {
	testInitCtrl(t)
	testInsertTimeSyncerConfig(t)
}

func testInsertTimeSyncerConfig(t require.TestingT) {
	// clean table
	err := ctrl.db.db.Unscoped().Delete(&mTimeSyncer{}).Error
	require.NoError(t, err)
	// insert
	configs := testdata.TimeSyncerConfigs(t)
	for tag, config := range configs {
		b, err := toml.Marshal(config)
		require.NoError(t, err)
		m := &mTimeSyncer{
			Tag:    "test_" + tag,
			Mode:   config.Mode,
			Config: string(b),
		}
		err = ctrl.db.InsertTimeSyncer(m)
		require.NoError(t, err)
	}
}

func TestSelectTimeSyncerConfig(t *testing.T) {
	testInitCtrl(t)
	clients, err := ctrl.db.SelectTimeSyncer()
	require.NoError(t, err)
	t.Log("select time syncer config:", spew.Sdump(clients))
}

func TestUpdateTimeSyncerConfig(t *testing.T) {
	testInitCtrl(t)
	configs, err := ctrl.db.SelectTimeSyncer()
	require.NoError(t, err)
	raw := configs[0].Mode
	configs[0].Mode = "changed"
	err = ctrl.db.UpdateTimeSyncer(configs[0])
	require.NoError(t, err)
	configs[0].Mode = raw
	err = ctrl.db.UpdateTimeSyncer(configs[0])
	require.NoError(t, err)
}

func TestDeleteTimeSyncerConfig(t *testing.T) {
	testInitCtrl(t)
	configs, err := ctrl.db.SelectTimeSyncer()
	require.NoError(t, err)
	err = ctrl.db.DeleteTimeSyncer(configs[0].ID)
	require.NoError(t, err)
	TestInsertTimeSyncerConfig(t)
}

func TestInsertBoot(t *testing.T) {
	testInitCtrl(t)
	testInsertBoot(t)
}

func testInsertBoot(t require.TestingT) {
	// clean table
	err := ctrl.db.db.Unscoped().Delete(&mBoot{}).Error
	require.NoError(t, err)
	// insert
	b := testdata.Register(t)
	for i := 0; i < len(b); i++ {
		m := &mBoot{
			Tag:      "test_" + b[i].Tag,
			Mode:     b[i].Mode,
			Config:   string(b[i].Config),
			Interval: uint32(15),
		}
		if m.Mode == bootstrap.ModeDirect {
			m.Enable = true
		}
		err := ctrl.db.InsertBoot(m)
		require.NoError(t, err)
	}
}

func TestSelectBoot(t *testing.T) {
	testInitCtrl(t)
	boots, err := ctrl.db.SelectBoot()
	require.NoError(t, err)
	t.Log("select boot:", spew.Sdump(boots))
}

func TestUpdateBoot(t *testing.T) {
	testInitCtrl(t)
	boots, err := ctrl.db.SelectBoot()
	require.NoError(t, err)
	raw := boots[0].Mode
	boots[0].Mode = "changed"
	err = ctrl.db.UpdateBoot(boots[0])
	require.NoError(t, err)
	boots[0].Mode = raw
	err = ctrl.db.UpdateBoot(boots[0])
	require.NoError(t, err)
}

func TestDeleteBoot(t *testing.T) {
	testInitCtrl(t)
	boots, err := ctrl.db.SelectBoot()
	require.NoError(t, err)
	err = ctrl.db.DeleteBoot(boots[0].ID)
	require.NoError(t, err)
	TestInsertBoot(t)
}

func TestInsertListener(t *testing.T) {
	testInitCtrl(t)
	testInsertListener(t)
}

func testInsertListener(t require.TestingT) {
	// clean table
	err := ctrl.db.db.Unscoped().Delete(&mListener{}).Error
	require.NoError(t, err)
	// insert
	listeners := testdata.Listeners(t)
	for i := 0; i < len(listeners); i++ {
		m := &mListener{
			Tag:    "test_" + listeners[i].Tag,
			Mode:   listeners[i].Mode,
			Config: string(listeners[i].Config),
		}
		err := ctrl.db.InsertListener(m)
		require.NoError(t, err)
	}
}

func TestSelectListener(t *testing.T) {
	testInitCtrl(t)
	listeners, err := ctrl.db.SelectListener()
	require.NoError(t, err)
	t.Log("select listener:", spew.Sdump(listeners))
}

func TestUpdateListener(t *testing.T) {
	testInitCtrl(t)
	listeners, err := ctrl.db.SelectListener()
	require.NoError(t, err)
	raw := listeners[0].Mode
	listeners[0].Mode = "changed"
	err = ctrl.db.UpdateListener(listeners[0])
	require.NoError(t, err)
	listeners[0].Mode = raw
	err = ctrl.db.UpdateListener(listeners[0])
	require.NoError(t, err)
}

func TestDeleteListener(t *testing.T) {
	testInitCtrl(t)
	listeners, err := ctrl.db.SelectListener()
	require.NoError(t, err)
	err = ctrl.db.DeleteListener(listeners[0].ID)
	require.NoError(t, err)
	TestInsertListener(t)
}

func TestInsertNode(t *testing.T) {
	testInitCtrl(t)
	node := &mNode{
		GUID:       bytes.Repeat([]byte{52}, guid.Size),
		SessionKey: bytes.Repeat([]byte{52}, aes.Bit256),
		PublicKey:  bytes.Repeat([]byte{52}, ed25519.PublicKeySize),
	}
	err := ctrl.db.db.Unscoped().Delete(node).Error
	require.NoError(t, err)
	err = ctrl.db.InsertNode(node)
	require.NoError(t, err)
	// insert listener
	nl := &mNodeListener{
		GUID:    node.GUID,
		Tag:     "tls_1",
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "127.0.0.1:1234",
	}
	err = ctrl.db.InsertNodeListener(nl)
	require.NoError(t, err)
	nl = &mNodeListener{
		GUID:    node.GUID,
		Tag:     "tls_2",
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "127.0.0.1:1235",
	}
	err = ctrl.db.InsertNodeListener(nl)
	require.NoError(t, err)
	// insert log
	lg := &mRoleLog{
		GUID:   node.GUID,
		Level:  logger.Debug,
		Source: "test",
		Log:    "test log",
	}
	err = ctrl.db.InsertNodeLog(lg)
	require.NoError(t, err)
}

func TestDeleteNode(t *testing.T) {
	testInitCtrl(t)
	err := ctrl.db.DeleteNode(bytes.Repeat([]byte{52}, guid.Size))
	require.NoError(t, err)
}

func TestDeleteNodeUnscoped(t *testing.T) {
	testInitCtrl(t)
	err := ctrl.db.DeleteNodeUnscoped(bytes.Repeat([]byte{52}, guid.Size))
	require.NoError(t, err)
}
