package controller

import (
	"bytes"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/guid"
	"project/internal/logger"
	"project/testdata"
)

var (
	test_guid = bytes.Repeat([]byte{0}, guid.SIZE)
)

func Test_DB_init(t *testing.T) {
	db := test_connect_database(t)
	defer db.Close()
	err := db.init()
	require.Nil(t, err, err)
}

func Test_DB_Ctrl_Log(t *testing.T) {
	db := test_connect_database(t)
	defer db.Close()
	// insert
	err := db.Insert_Ctrl_Log(logger.DEBUG, "test src", "test log")
	require.Nil(t, err, err)
	err = db.Insert_Ctrl_Log(logger.DEBUG, "test src", "test log")
	require.Nil(t, err, err)
	// select
	var logs []*m_controller_log
	err = db.db.Find(&logs).Error
	require.Nil(t, err, err)
	t.Log("select controller log:", spew.Sdump(logs))
	// soft delete
	err = db.Delete_Ctrl_Log()
	require.Nil(t, err, err)
}

func Test_DB_Proxy_Client(t *testing.T) {
	db := test_connect_database(t)
	defer db.Close()
	// clean table
	err := db.db.Unscoped().Delete(&m_proxy_client{}).Error
	require.Nil(t, err, err)
	// insert
	proxy_clients := testdata.Proxy_Clients(t)
	for tag, c := range proxy_clients {
		err := db.Insert_Proxy_Client(tag, c.Mode, c.Config)
		require.Nil(t, err, err)
	}
	// select
	clients, err := db.Select_Proxy_Client()
	require.Nil(t, err, err)
	t.Log("select proxy client:", spew.Sdump(clients))
	// update
	clients[0].Mode = "changed"
	err = db.Update_Proxy_Client(clients[0])
	require.Nil(t, err, err)
	// soft delete
	err = db.Delete_Proxy_Client(clients[0].ID)
	require.Nil(t, err, err)
}

func Test_DB_DNS_Client(t *testing.T) {
	db := test_connect_database(t)
	defer db.Close()
	// clean table
	err := db.db.Unscoped().Delete(&m_dns_client{}).Error
	require.Nil(t, err, err)
	// insert
	dns_clients := testdata.DNS_Clients(t)
	for tag, c := range dns_clients {
		err := db.Insert_DNS_Client(tag, c.Method, c.Address)
		require.Nil(t, err, err)
	}
	// select
	clients, err := db.Select_DNS_Client()
	require.Nil(t, err, err)
	t.Log("select dns client:", spew.Sdump(clients))
	// update
	clients[0].Method = "changed"
	err = db.Update_DNS_Client(clients[0])
	require.Nil(t, err, err)
	// soft delete
	err = db.Delete_DNS_Client(clients[0].ID)
	require.Nil(t, err, err)
}

func Test_DB_Timesync(t *testing.T) {
	db := test_connect_database(t)
	defer db.Close()
	// clean table
	err := db.db.Unscoped().Delete(&m_timesync{}).Error
	require.Nil(t, err, err)
	// insert
	timesync := testdata.Timesync_Full(t)
	for tag, c := range timesync {
		config, err := toml.Marshal(c)
		require.Nil(t, err, err)
		err = db.Insert_Timesync(tag, c.Mode, string(config))
		require.Nil(t, err, err)
	}
	// select
	clients, err := db.Select_Timesync()
	require.Nil(t, err, err)
	t.Log("select timesync:", spew.Sdump(clients))
	// update
	clients[0].Mode = "changed"
	err = db.Update_Timesync(clients[0])
	require.Nil(t, err, err)
	// soft delete
	err = db.Delete_Timesync(clients[0].ID)
	require.Nil(t, err, err)
}

func Test_Insert_Bootstrap(t *testing.T) {
	db := test_connect_database(t)
	b := testdata.Register(t)
	for i := 0; i < len(b); i++ {
		c := string(b[i].Config)
		interval := uint32(15) // 15 second
		err := db.Insert_Bootstrap(b[i].Tag, b[i].Mode, c, interval, true)
		require.Nil(t, err, err)
	}
}

func Test_Insert_Listener(t *testing.T) {
	db := test_connect_database(t)
	l := testdata.Listeners(t)
	for i := 0; i < len(l); i++ {
		c := string(l[i].Config)
		err := db.Insert_Listener(l[i].Tag, l[i].Mode, c)
		require.Nil(t, err, err)
	}
}

func Test_Insert_Log(t *testing.T) {
	//db := test_connect_database(t)
	//err := db.Insert_Node_Log(test_guid, logger.DEBUG, "test src", "test log")
	//require.Nil(t, err, err)
	//err = db.Insert_Beacon_Log(test_guid, logger.DEBUG, "test src", "test log")
	//require.Nil(t, err, err)
}

func test_connect_database(t *testing.T) *database {
	CTRL, err := New(test_gen_config())
	require.Nil(t, err, err)
	d, err := new_database(CTRL)
	require.Nil(t, err, err)
	err = d.Connect()
	require.Nil(t, err, err)
	return d
}
