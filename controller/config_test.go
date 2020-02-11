package controller

import (
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/logger"
)

func testGenerateConfig() *Config {
	cfg := Config{}

	cfg.Test.SkipTestClientDNS = true
	cfg.Test.SkipSynchronizeTime = true

	cfg.Database.Dialect = "mysql"
	cfg.Database.DSN = "pbnet:pbnet@tcp(127.0.0.1:3306)/pbnet_test?loc=Local&parseTime=true"
	cfg.Database.MaxOpenConns = 16
	cfg.Database.MaxIdleConns = 16
	cfg.Database.LogFile = "log/database.log"
	cfg.Database.GORMLogFile = "log/gorm.log"
	cfg.Database.GORMDetailedLog = false
	cfg.Database.LogWriter = logger.NewWriterWithPrefix(os.Stdout, "Ctrl")

	cfg.Logger.Level = "debug"
	cfg.Logger.File = "log/controller.log"
	cfg.Logger.Writer = logger.NewWriterWithPrefix(os.Stdout, "Ctrl")

	cfg.Global.DNSCacheExpire = time.Minute
	cfg.Global.TimeSyncSleepFixed = 15
	cfg.Global.TimeSyncSleepRandom = 10
	cfg.Global.TimeSyncInterval = time.Minute

	cfg.Client.Timeout = 10 * time.Second

	cfg.Sender.MaxConns = 16 * runtime.NumCPU()
	cfg.Sender.Worker = 64
	cfg.Sender.Timeout = 15 * time.Second
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 16 << 10

	cfg.Syncer.ExpireTime = 3 * time.Second

	cfg.Worker.Number = 64
	cfg.Worker.QueueSize = 512
	cfg.Worker.MaxBufferSize = 16 << 10

	cfg.Web.Dir = "web"
	cfg.Web.CertFile = "ca/cert.pem"
	cfg.Web.KeyFile = "ca/key.pem"
	cfg.Web.CertOpts.DNSNames = []string{"localhost"}
	cfg.Web.CertOpts.IPAddresses = []string{"127.0.0.1", "::1"}
	cfg.Web.Network = "tcp"
	cfg.Web.Address = "localhost:1657"
	cfg.Web.Username = "pbnet" // # super user, password = "pbnet"
	cfg.Web.Password = "$2a$12$zWgjYi0aAq.958UtUyDi5.QDmq4LOWsvv7I9ulvf1rHzd9/dWWmTi"
	return &cfg
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
		{expected: uint(15), actual: cfg.Global.TimeSyncSleepFixed},
		{expected: uint(10), actual: cfg.Global.TimeSyncSleepRandom},
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
		{expected: []string{"localhost"}, actual: cfg.Web.CertOpts.DNSNames},
		{expected: "tcp4", actual: cfg.Web.Network},
		{expected: "localhost:1657", actual: cfg.Web.Address},
		{expected: "pbnet", actual: cfg.Web.Username},
		{expected: "bcrypt", actual: cfg.Web.Password},
	}
	for _, td := range tds {
		require.Equal(t, td.expected, td.actual)
	}
}
