package controller

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/testdata"
)

func Test_connect_database(t *testing.T) {
	db, err := connect_database(test_gen_config())
	require.Nil(t, err, err)
	_ = db.Close()
}

func Test_init_database(t *testing.T) {
	db, err := connect_database(test_gen_config())
	require.Nil(t, err, err)
	defer func() { _ = db.Close() }()
	err = init_database(db)
	require.Nil(t, err, err)
}

func Test_Insert_Proxy_Client(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_proxy_client{}).Error
	require.Nil(t, err, err)
	// insert
	proxy_clients := testdata.Proxy_Clients(t)
	for tag, c := range proxy_clients {
		err := ctrl.Insert_Proxy_Client(tag, c.Mode, c.Config)
		require.Nil(t, err, err)
	}
}

func Test_Select_Proxy_Client(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_Proxy_Client()
	require.Nil(t, err, err)
	t.Log("select proxy client:", spew.Sdump(clients))
}

func Test_Update_Proxy_Client(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
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
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_Proxy_Client()
	require.Nil(t, err, err)
	err = ctrl.Delete_Proxy_Client(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_Proxy_Client(t)
}

func Test_Insert_DNS_Client(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_dns_client{}).Error
	require.Nil(t, err, err)
	// insert
	dns_clients := testdata.DNS_Clients(t)
	for tag, c := range dns_clients {
		err := ctrl.Insert_DNS_Client(tag, c.Method, c.Address)
		require.Nil(t, err, err)
	}
}

func Test_Select_DNS_Client(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_DNS_Client()
	require.Nil(t, err, err)
	t.Log("select dns client:", spew.Sdump(clients))
}

func Test_Update_DNS_Client(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
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
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_DNS_Client()
	require.Nil(t, err, err)
	err = ctrl.Delete_DNS_Client(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_DNS_Client(t)
}

func Test_Insert_Timesync(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_timesync{}).Error
	require.Nil(t, err, err)
	// insert
	timesync := testdata.Timesync(t)
	for tag, c := range timesync {
		config, err := toml.Marshal(c)
		require.Nil(t, err, err)
		err = ctrl.Insert_Timesync(tag, c.Mode, string(config))
		require.Nil(t, err, err)
	}
}

func Test_Select_Timesync(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_Timesync()
	require.Nil(t, err, err)
	t.Log("select timesync:", spew.Sdump(clients))
}

func Test_Update_Timesync(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
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
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_Timesync()
	require.Nil(t, err, err)
	err = ctrl.Delete_Timesync(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_Timesync(t)
}

func Test_Insert_Bootstrap(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	b := testdata.Register(t)
	for i := 0; i < len(b); i++ {
		c := string(b[i].Config)
		interval := uint32(15) // 15 second
		err := ctrl.Insert_Bootstrap(b[i].Tag, b[i].Mode, c, interval, true)
		require.Nil(t, err, err)
	}
}

func Test_Insert_Listener(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	l := testdata.Listeners(t)
	for i := 0; i < len(l); i++ {
		c := string(l[i].Config)
		err := ctrl.Insert_Listener(l[i].Tag, l[i].Mode, c)
		require.Nil(t, err, err)
	}
}
