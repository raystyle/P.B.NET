package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_Controller(t *testing.T) {
	CTRL, err := New(test_gen_config())
	require.Nil(t, err, err)
	err = CTRL.Main()
	require.Nil(t, err, err)
}

func test_gen_config() *Config {
	c := &Config{
		Log_Level: "debug",
		// global
		DNS_Cache_Deadline: 3 * time.Minute,
		Timesync_Interval:  15 * time.Minute,
		// database
		Dialect:  "mysql",
		DSN:      "root:asf15asfujks1d@tcp(127.0.0.1:3306)/p.b.net?loc=Local&parseTime=true",
		DB_Log:   "../app/log/database.log",
		GORM_Log: "../app/log/gorm.log",
	}
	return c
}
