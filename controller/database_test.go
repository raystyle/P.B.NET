package controller

import (
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/testdata"
)

func Test_Init_DB(t *testing.T) {
	db := test_connect_database(t)
	err := db.init_db()
	require.Nil(t, err, err)
}

func Test_Insert_Global(t *testing.T) {
	db := test_connect_database(t)
	// proxy clients
	proxy_clients := testdata.Proxy_Clients(t)
	for tag, c := range proxy_clients {
		err := db.Insert_Proxy_Client(tag, c.Mode, c.Config)
		require.Nil(t, err, err)
	}
	// dns clients
	dns_clients := testdata.DNS_Clients(t)
	for tag, c := range dns_clients {
		err := db.Insert_DNS_Client(tag, c.Method, c.Address)
		require.Nil(t, err, err)
	}
	// timesync
	timesync := testdata.Timesync_Full(t)
	for tag, c := range timesync {
		config, err := toml.Marshal(c)
		require.Nil(t, err, err)
		err = db.Insert_Timesync(tag, c.Mode, string(config))
		require.Nil(t, err, err)
	}
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

func Test_Insert_listener(t *testing.T) {
	db := test_connect_database(t)
	l := testdata.Listeners(t)
	for i := 0; i < len(l); i++ {
		c := string(l[i].Config)
		err := db.Insert_Listener(l[i].Tag, l[i].Mode, c)
		require.Nil(t, err, err)
	}
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
