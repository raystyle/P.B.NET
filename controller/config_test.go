package controller

import (
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"
)

func testGenerateConfig() *Config {
	c := Config{}

	c.Debug.SkipTestClientDNS = true
	c.Debug.SkipSynchronizeTime = true

	c.Database.Dialect = "mysql"
	c.Database.DSN = "pbnet:pbnet@tcp(127.0.0.1:3306)/pbnet_test?loc=Local&parseTime=true"
	c.Database.MaxOpenConns = 16
	c.Database.MaxIdleConns = 16
	c.Database.LogFile = "log/database.log"
	c.Database.GORMLogFile = "log/gorm.log"
	c.Database.GORMDetailedLog = false

	c.Logger.Level = "debug"
	c.Logger.File = "log/controller.log"
	c.Logger.Writer = os.Stdout

	c.Global.DNSCacheExpire = time.Minute
	c.Global.TimeSyncInterval = time.Minute

	c.Client.Timeout = 10 * time.Second

	c.Sender.MaxConns = 16 * runtime.NumCPU()
	c.Sender.Worker = 64
	c.Sender.Timeout = 15 * time.Second
	c.Sender.QueueSize = 512
	c.Sender.MaxBufferSize = 16384

	c.Syncer.ExpireTime = 3 * time.Second

	c.Worker.Number = 64
	c.Worker.QueueSize = 512
	c.Worker.MaxBufferSize = 16384

	c.Web.Dir = "web"
	c.Web.CertFile = "ca/cert.pem"
	c.Web.KeyFile = "ca/key.pem"
	c.Web.Address = "localhost:1657"
	c.Web.Username = "pbnet" // # super user, password = sha256(sha256("pbnet"))
	c.Web.Password = "d6b3ced503b70f7894bd30f36001de4af84a8c2af898f06e29bca95f2dcf5100"
	return &c
}

func TestConfig(t *testing.T) {
	b, err := ioutil.ReadFile("testdata/config.toml")
	require.NoError(t, err)
	cfg := Config{}
	require.NoError(t, toml.Unmarshal(b, &cfg))

	tds := [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "mysql", actual: cfg.Database.Dialect},
		{expected: "dsn", actual: cfg.Database.DSN},
		{expected: 16, actual: cfg.Database.MaxOpenConns},
		{expected: 16, actual: cfg.Database.MaxIdleConns},
		{expected: "log1", actual: cfg.Database.LogFile},
		{expected: "log2", actual: cfg.Database.GORMLogFile},
		{expected: true, actual: cfg.Database.GORMDetailedLog},

		{expected: "debug", actual: cfg.Logger.Level},
		{expected: "log3", actual: cfg.Logger.File},

		{expected: 2 * time.Minute, actual: cfg.Global.DNSCacheExpire},
		{expected: time.Minute, actual: cfg.Global.TimeSyncInterval},

		{expected: "test", actual: cfg.Client.ProxyTag},
		{expected: 15 * time.Second, actual: cfg.Client.Timeout},
		{expected: "custom", actual: cfg.Client.DNSOpts.Mode},

		{expected: 7, actual: cfg.Sender.MaxConns},
		{expected: 64, actual: cfg.Sender.Worker},
		{expected: 15 * time.Second, actual: cfg.Sender.Timeout},
		{expected: 512, actual: cfg.Sender.QueueSize},
		{expected: 16384, actual: cfg.Sender.MaxBufferSize},

		{expected: 3 * time.Minute, actual: cfg.Syncer.ExpireTime},

		{expected: 64, actual: cfg.Worker.Number},
		{expected: 512, actual: cfg.Worker.QueueSize},
		{expected: 16384, actual: cfg.Worker.MaxBufferSize},

		{expected: "web", actual: cfg.Web.Dir},
		{expected: "ca/cert.pem", actual: cfg.Web.CertFile},
		{expected: "ca/key.pem", actual: cfg.Web.KeyFile},
		{expected: "localhost:1657", actual: cfg.Web.Address},
		{expected: "pbnet", actual: cfg.Web.Username},
		{expected: "sha256", actual: cfg.Web.Password},
	}
	for _, td := range tds {
		require.Equal(t, td.expected, td.actual)
	}
}
