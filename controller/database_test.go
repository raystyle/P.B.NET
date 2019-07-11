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

func Test_Insert_Proxy_Client(t *testing.T) {
	init_ctrl(t)
	test_insert_proxy_client(t)
}

func test_insert_proxy_client(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_proxy_client{}).Error
	require.Nil(t, err, err)
	// insert
	proxy_clients := testdata.Proxy_Clients(t)
	for tag, client := range proxy_clients {
		m := &m_proxy_client{
			Tag:    "test_" + tag,
			Mode:   client.Mode,
			Config: client.Config,
		}
		err := ctrl.Insert_Proxy_Client(m)
		require.Nil(t, err, err)
	}
}

func Test_Select_Proxy_Client(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Proxy_Client()
	require.Nil(t, err, err)
	t.Log("select proxy client:", spew.Sdump(clients))
}

func Test_Update_Proxy_Client(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Proxy_Client()
	require.Nil(t, err, err)
	raw := clients[0].Mode
	clients[0].Mode = "changed"
	err = ctrl.Update_Proxy_Client(clients[0])
	require.Nil(t, err, err)
	clients[0].Mode = raw
	err = ctrl.Update_Proxy_Client(clients[0])
	require.Nil(t, err, err)
}

func Test_Delete_Proxy_Client(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Proxy_Client()
	require.Nil(t, err, err)
	err = ctrl.Delete_Proxy_Client(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_Proxy_Client(t)
}

func Test_Insert_DNS_Client(t *testing.T) {
	init_ctrl(t)
	test_insert_dns_client(t)
}

func test_insert_dns_client(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_dns_client{}).Error
	require.Nil(t, err, err)
	// insert
	dns_clients := testdata.DNS_Clients(t)
	for tag, client := range dns_clients {
		m := &m_dns_client{
			Tag:     "test_" + tag,
			Method:  client.Method,
			Address: client.Address,
		}
		err := ctrl.Insert_DNS_Client(m)
		require.Nil(t, err, err)
	}
}

func Test_Select_DNS_Client(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_DNS_Client()
	require.Nil(t, err, err)
	t.Log("select dns client:", spew.Sdump(clients))
}

func Test_Update_DNS_Client(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_DNS_Client()
	require.Nil(t, err, err)
	raw := clients[0].Method
	clients[0].Method = "changed"
	err = ctrl.Update_DNS_Client(clients[0])
	require.Nil(t, err, err)
	clients[0].Method = raw
	err = ctrl.Update_DNS_Client(clients[0])
	require.Nil(t, err, err)
}

func Test_Delete_DNS_Client(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_DNS_Client()
	require.Nil(t, err, err)
	err = ctrl.Delete_DNS_Client(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_DNS_Client(t)
}

func Test_Insert_Timesync(t *testing.T) {
	init_ctrl(t)
	test_insert_timesync(t)
}

func test_insert_timesync(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_timesync{}).Error
	require.Nil(t, err, err)
	// insert
	timesync := testdata.Timesync(t)
	for tag, client := range timesync {
		config, err := toml.Marshal(client)
		require.Nil(t, err, err)
		m := &m_timesync{
			Tag:    "test_" + tag,
			Mode:   client.Mode,
			Config: string(config),
		}
		err = ctrl.Insert_Timesync(m)
		require.Nil(t, err, err)
	}
}

func Test_Select_Timesync(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Timesync()
	require.Nil(t, err, err)
	t.Log("select timesync:", spew.Sdump(clients))
}

func Test_Update_Timesync(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Timesync()
	require.Nil(t, err, err)
	raw := clients[0].Mode
	clients[0].Mode = "changed"
	err = ctrl.Update_Timesync(clients[0])
	require.Nil(t, err, err)
	clients[0].Mode = raw
	err = ctrl.Update_Timesync(clients[0])
	require.Nil(t, err, err)
}

func Test_Delete_Timesync(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Timesync()
	require.Nil(t, err, err)
	err = ctrl.Delete_Timesync(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_Timesync(t)
}

func Test_Insert_Boot(t *testing.T) {
	init_ctrl(t)
	test_insert_boot(t)
}

func test_insert_boot(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_boot{}).Error
	require.Nil(t, err, err)
	// insert
	b := testdata.Register(t)
	for i := 0; i < len(b); i++ {
		m := &m_boot{
			Tag:      "test_" + b[i].Tag,
			Mode:     b[i].Mode,
			Config:   string(b[i].Config),
			Interval: uint32(15),
		}
		if m.Mode == bootstrap.M_DIRECT {
			m.Enable = true
		}
		err := ctrl.Insert_Boot(m)
		require.Nil(t, err, err)
	}
}

func Test_Select_Boot(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Boot()
	require.Nil(t, err, err)
	t.Log("select bootstrap:", spew.Sdump(clients))
}

func Test_Update_Boot(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Boot()
	require.Nil(t, err, err)
	raw := clients[0].Mode
	clients[0].Mode = "changed"
	err = ctrl.Update_Boot(clients[0])
	require.Nil(t, err, err)
	clients[0].Mode = raw
	err = ctrl.Update_Boot(clients[0])
	require.Nil(t, err, err)
}

func Test_Delete_Boot(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Boot()
	require.Nil(t, err, err)
	err = ctrl.Delete_Boot(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_Boot(t)
}

func Test_Insert_Listener(t *testing.T) {
	init_ctrl(t)
	test_insert_listener(t)
}

func test_insert_listener(t require.TestingT) {
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_listener{}).Error
	require.Nil(t, err, err)
	// insert
	l := testdata.Listeners(t)
	for i := 0; i < len(l); i++ {
		m := &m_listener{
			Tag:    "test_" + l[i].Tag,
			Mode:   l[i].Mode,
			Config: string(l[i].Config),
		}
		err := ctrl.Insert_Listener(m)
		require.Nil(t, err, err)
	}
}

func Test_Select_Listener(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Listener()
	require.Nil(t, err, err)
	t.Log("select listener:", spew.Sdump(clients))
}

func Test_Update_Listener(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Listener()
	require.Nil(t, err, err)
	raw := clients[0].Mode
	clients[0].Mode = "changed"
	err = ctrl.Update_Listener(clients[0])
	require.Nil(t, err, err)
	clients[0].Mode = raw
	err = ctrl.Update_Listener(clients[0])
	require.Nil(t, err, err)
}

func Test_Delete_Listener(t *testing.T) {
	init_ctrl(t)
	clients, err := ctrl.Select_Listener()
	require.Nil(t, err, err)
	err = ctrl.Delete_Listener(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_Listener(t)
}

func Test_Insert_Node(t *testing.T) {
	init_ctrl(t)
	node := &m_node{
		GUID:      bytes.Repeat([]byte{52}, guid.SIZE),
		AES_Key:   bytes.Repeat([]byte{52}, aes.BIT256+aes.IV_SIZE),
		Publickey: bytes.Repeat([]byte{52}, ed25519.PublicKey_Size),
	}
	err := ctrl.db.Unscoped().Delete(node).Error
	require.Nil(t, err, err)
	err = ctrl.Insert_Node(node)
	require.Nil(t, err, err)
	// insert listener
	nl := &m_node_listener{
		GUID:    node.GUID,
		Tag:     "tls_1",
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "127.0.0.1:1234",
	}
	err = ctrl.Insert_Node_Listener(nl)
	require.Nil(t, err, err)
	nl = &m_node_listener{
		GUID:    node.GUID,
		Tag:     "tls_2",
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "127.0.0.1:1235",
	}
	err = ctrl.Insert_Node_Listener(nl)
	require.Nil(t, err, err)
	// insert log
	lg := &m_role_log{
		GUID:   node.GUID,
		Level:  logger.DEBUG,
		Source: "test",
		Log:    "test log",
	}
	err = ctrl.Insert_Node_Log(lg)
	require.Nil(t, err, err)
}

func Test_Delete_Node(t *testing.T) {
	init_ctrl(t)
	err := ctrl.Delete_Node(bytes.Repeat([]byte{52}, guid.SIZE))
	require.Nil(t, err, err)
}

func Test_Delete_Node_Unscoped(t *testing.T) {
	init_ctrl(t)
	// ctrl.db.LogMode(true)
	err := ctrl.Delete_Node_Unscoped(bytes.Repeat([]byte{52}, guid.SIZE))
	require.Nil(t, err, err)
}
