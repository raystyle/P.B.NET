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
	"project/internal/xnet"
	"project/testdata"
)

func TestInsertProxyClient(t *testing.T) {
	initCtrl(t)
	testInsertProxyClient(t)
}

func testInsertProxyClient(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&mProxyClient{}).Error
	require.NoError(t, err)
	// insert
	clients := testdata.ProxyClients(t)
	for tag, client := range clients {
		m := &mProxyClient{
			Tag:    "test_" + tag,
			Mode:   client.Mode,
			Config: client.Config,
		}
		err := ctrl.InsertProxyClient(m)
		require.NoError(t, err)
	}
}

func TestSelectProxyClient(t *testing.T) {
	initCtrl(t)
	clients, err := ctrl.SelectProxyClient()
	require.NoError(t, err)
	t.Log("select proxy client:", spew.Sdump(clients))
}

func TestUpdateProxyClient(t *testing.T) {
	initCtrl(t)
	clients, err := ctrl.SelectProxyClient()
	require.NoError(t, err)
	raw := clients[0].Mode
	clients[0].Mode = "changed"
	err = ctrl.UpdateProxyClient(clients[0])
	require.NoError(t, err)
	clients[0].Mode = raw
	err = ctrl.UpdateProxyClient(clients[0])
	require.NoError(t, err)
}

func TestDeleteProxyClient(t *testing.T) {
	initCtrl(t)
	clients, err := ctrl.SelectProxyClient()
	require.NoError(t, err)
	err = ctrl.DeleteProxyClient(clients[0].ID)
	require.NoError(t, err)
	TestInsertProxyClient(t)
}

func TestInsertDNSServer(t *testing.T) {
	initCtrl(t)
	testInsertDNSServer(t)
}

func testInsertDNSServer(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&mDNSServer{}).Error
	require.NoError(t, err)
	// insert
	servers := testdata.DNSServers(t)
	for tag, server := range servers {
		m := &mDNSServer{
			Tag:     "test_" + tag,
			Method:  server.Method,
			Address: server.Address,
		}
		err := ctrl.InsertDNSServer(m)
		require.NoError(t, err)
	}
}

func TestSelectDNSServer(t *testing.T) {
	initCtrl(t)
	servers, err := ctrl.SelectDNSServer()
	require.NoError(t, err)
	t.Log("select dns client:", spew.Sdump(servers))
}

func TestUpdateDNSServer(t *testing.T) {
	initCtrl(t)
	servers, err := ctrl.SelectDNSServer()
	require.NoError(t, err)
	raw := servers[0].Method
	servers[0].Method = "changed"
	err = ctrl.UpdateDNSServer(servers[0])
	require.NoError(t, err)
	servers[0].Method = raw
	err = ctrl.UpdateDNSServer(servers[0])
	require.NoError(t, err)
}

func TestDeleteDNSServer(t *testing.T) {
	initCtrl(t)
	servers, err := ctrl.SelectDNSServer()
	require.NoError(t, err)
	err = ctrl.DeleteDNSServer(servers[0].ID)
	require.NoError(t, err)
	TestInsertDNSServer(t)
}

func TestInsertTimeSyncerConfig(t *testing.T) {
	initCtrl(t)
	testInsertTimeSyncerConfig(t)
}

func testInsertTimeSyncerConfig(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&mTimeSyncer{}).Error
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
		err = ctrl.InsertTimeSyncer(m)
		require.NoError(t, err)
	}
}

func TestSelectTimeSyncerConfig(t *testing.T) {
	initCtrl(t)
	clients, err := ctrl.SelectTimeSyncer()
	require.NoError(t, err)
	t.Log("select time syncer config:", spew.Sdump(clients))
}

func TestUpdateTimeSyncerConfig(t *testing.T) {
	initCtrl(t)
	configs, err := ctrl.SelectTimeSyncer()
	require.NoError(t, err)
	raw := configs[0].Mode
	configs[0].Mode = "changed"
	err = ctrl.UpdateTimeSyncer(configs[0])
	require.NoError(t, err)
	configs[0].Mode = raw
	err = ctrl.UpdateTimeSyncer(configs[0])
	require.NoError(t, err)
}

func TestDeleteTimeSyncerConfig(t *testing.T) {
	initCtrl(t)
	configs, err := ctrl.SelectTimeSyncer()
	require.NoError(t, err)
	err = ctrl.DeleteTimeSyncer(configs[0].ID)
	require.NoError(t, err)
	TestInsertTimeSyncerConfig(t)
}

func TestInsertBoot(t *testing.T) {
	initCtrl(t)
	testInsertBoot(t)
}

func testInsertBoot(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&mBoot{}).Error
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
		err := ctrl.InsertBoot(m)
		require.NoError(t, err)
	}
}

func TestSelectBoot(t *testing.T) {
	initCtrl(t)
	boots, err := ctrl.SelectBoot()
	require.NoError(t, err)
	t.Log("select boot:", spew.Sdump(boots))
}

func TestUpdateBoot(t *testing.T) {
	initCtrl(t)
	boots, err := ctrl.SelectBoot()
	require.NoError(t, err)
	raw := boots[0].Mode
	boots[0].Mode = "changed"
	err = ctrl.UpdateBoot(boots[0])
	require.NoError(t, err)
	boots[0].Mode = raw
	err = ctrl.UpdateBoot(boots[0])
	require.NoError(t, err)
}

func TestDeleteBoot(t *testing.T) {
	initCtrl(t)
	boots, err := ctrl.SelectBoot()
	require.NoError(t, err)
	err = ctrl.DeleteBoot(boots[0].ID)
	require.NoError(t, err)
	TestInsertBoot(t)
}

func TestInsertListener(t *testing.T) {
	initCtrl(t)
	testInsertListener(t)
}

func testInsertListener(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&mListener{}).Error
	require.NoError(t, err)
	// insert
	listeners := testdata.Listeners(t)
	for i := 0; i < len(listeners); i++ {
		m := &mListener{
			Tag:    "test_" + listeners[i].Tag,
			Mode:   listeners[i].Mode,
			Config: string(listeners[i].Config),
		}
		err := ctrl.InsertListener(m)
		require.NoError(t, err)
	}
}

func TestSelectListener(t *testing.T) {
	initCtrl(t)
	listeners, err := ctrl.SelectListener()
	require.NoError(t, err)
	t.Log("select listener:", spew.Sdump(listeners))
}

func TestUpdateListener(t *testing.T) {
	initCtrl(t)
	listeners, err := ctrl.SelectListener()
	require.NoError(t, err)
	raw := listeners[0].Mode
	listeners[0].Mode = "changed"
	err = ctrl.UpdateListener(listeners[0])
	require.NoError(t, err)
	listeners[0].Mode = raw
	err = ctrl.UpdateListener(listeners[0])
	require.NoError(t, err)
}

func TestDeleteListener(t *testing.T) {
	initCtrl(t)
	listeners, err := ctrl.SelectListener()
	require.NoError(t, err)
	err = ctrl.DeleteListener(listeners[0].ID)
	require.NoError(t, err)
	TestInsertListener(t)
}

func TestInsertNode(t *testing.T) {
	initCtrl(t)
	node := &mNode{
		GUID:       bytes.Repeat([]byte{52}, guid.SIZE),
		SessionKey: bytes.Repeat([]byte{52}, aes.Bit256+aes.IVSize),
		PublicKey:  bytes.Repeat([]byte{52}, ed25519.PublicKeySize),
	}
	err := ctrl.db.Unscoped().Delete(node).Error
	require.NoError(t, err)
	err = ctrl.InsertNode(node)
	require.NoError(t, err)
	// insert listener
	nl := &mNodeListener{
		GUID:    node.GUID,
		Tag:     "tls_1",
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "127.0.0.1:1234",
	}
	err = ctrl.InsertNodeListener(nl)
	require.NoError(t, err)
	nl = &mNodeListener{
		GUID:    node.GUID,
		Tag:     "tls_2",
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "127.0.0.1:1235",
	}
	err = ctrl.InsertNodeListener(nl)
	require.NoError(t, err)
	// insert log
	lg := &mRoleLog{
		GUID:   node.GUID,
		Level:  logger.DEBUG,
		Source: "test",
		Log:    "test log",
	}
	err = ctrl.InsertNodeLog(lg)
	require.NoError(t, err)
}

func TestDeleteNode(t *testing.T) {
	initCtrl(t)
	err := ctrl.DeleteNode(bytes.Repeat([]byte{52}, guid.SIZE))
	require.NoError(t, err)
}

func TestDeleteNodeUnscoped(t *testing.T) {
	initCtrl(t)
	// ctrl.db.LogMode(true)
	err := ctrl.DeleteNodeUnscoped(bytes.Repeat([]byte{52}, guid.SIZE))
	require.NoError(t, err)
}
