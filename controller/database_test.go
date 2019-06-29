package controller

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/jinzhu/gorm"
	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/testdata"
)

// if need init database set true
func Test_init_database(t *testing.T) {
	if false {
		db := test_connect_database(t)
		defer func() { _ = db.Close() }()
		err := init_database(db)
		require.Nil(t, err, err)
	}
}

func Test_connect_database(t *testing.T) {
	db := test_connect_database(t)
	_ = db.Close()
}

func test_connect_database(t *testing.T) *gorm.DB {
	c := test_gen_config()
	c.DB_Log_Path = "../app/log/database.log"
	c.GORM_Log_Path = "../app/log/gorm.log"
	db, err := connect_database(c)
	require.Nil(t, err, err)
	return db
}

func Test_Insert_Proxy_Client(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_proxy_client{}).Error
	require.Nil(t, err, err)
	// insert
	proxy_clients := testdata.Proxy_Clients(t)
	for tag, client := range proxy_clients {
		m := &m_proxy_client{
			Tag:    tag,
			Mode:   client.Mode,
			Config: client.Config,
		}
		err := ctrl.Insert_Proxy_Client(m)
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
	for tag, client := range dns_clients {
		m := &m_dns_client{
			Tag:     tag,
			Method:  client.Method,
			Address: client.Address,
		}
		err := ctrl.Insert_DNS_Client(m)
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
	for tag, client := range timesync {
		config, err := toml.Marshal(client)
		require.Nil(t, err, err)
		m := &m_timesync{
			Tag:    tag,
			Mode:   client.Mode,
			Config: string(config),
		}
		err = ctrl.Insert_Timesync(m)
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

func Test_Insert_Bootstrapper(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_bootstrapper{}).Error
	require.Nil(t, err, err)
	// insert
	b := testdata.Register(t)
	for i := 0; i < len(b); i++ {
		m := &m_bootstrapper{
			Tag:      b[i].Tag,
			Mode:     b[i].Mode,
			Config:   string(b[i].Config),
			Interval: uint32(15),
			Enable:   true,
		}
		err := ctrl.Insert_Bootstrapper(m)
		require.Nil(t, err, err)
	}
}

func Test_Select_Bootstrapper(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_Bootstrapper()
	require.Nil(t, err, err)
	t.Log("select bootstrap:", spew.Sdump(clients))
}

func Test_Update_Bootstrapper(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_Bootstrapper()
	require.Nil(t, err, err)
	raw := clients[0].Mode
	clients[0].Mode = "changed"
	err = ctrl.Update_Bootstrapper(clients[0])
	require.Nil(t, err, err)
	clients[0].Mode = raw
	err = ctrl.Update_Bootstrapper(clients[0])
	require.Nil(t, err, err)
}

func Test_Delete_Bootstrapper(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_Bootstrapper()
	require.Nil(t, err, err)
	err = ctrl.Delete_Bootstrapper(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_Bootstrapper(t)
}

func Test_Insert_Listener(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	// clean table
	err := ctrl.db.Unscoped().Delete(&m_listener{}).Error
	require.Nil(t, err, err)
	// insert
	l := testdata.Listeners(t)
	for i := 0; i < len(l); i++ {
		m := &m_listener{
			Tag:    l[i].Tag,
			Mode:   l[i].Mode,
			Config: string(l[i].Config),
		}
		err := ctrl.Insert_Listener(m)
		require.Nil(t, err, err)
	}
}

func Test_Select_Listener(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_Listener()
	require.Nil(t, err, err)
	t.Log("select listener:", spew.Sdump(clients))
}

func Test_Update_Listener(t *testing.T) {
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
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
	ctrl := test_gen_ctrl(t)
	defer ctrl.Exit()
	clients, err := ctrl.Select_Listener()
	require.Nil(t, err, err)
	err = ctrl.Delete_Listener(clients[0].ID)
	require.Nil(t, err, err)
	Test_Insert_Listener(t)
}
