package controller

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/toml"
	"project/internal/testsuite"
)

func testGenerateConfig() *Config {
	cfg := Config{}

	cfg.Database.Dialect = "mysql"
	cfg.Database.DSN = "pbnet:pbnet@tcp(127.0.0.1:3306)/pbnet_dev?loc=Local&parseTime=true"
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
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateRootCA = true
	cfg.Client.TLSConfig.LoadFromCertPool.LoadPrivateClient = true

	cfg.Sender.MaxConns = 16
	cfg.Sender.Worker = 64
	cfg.Sender.Timeout = 15 * time.Second
	cfg.Sender.QueueSize = 512
	cfg.Sender.MaxBufferSize = 16 << 10

	cfg.Syncer.ExpireTime = 3 * time.Second

	cfg.Worker.Number = 64
	cfg.Worker.QueueSize = 512
	cfg.Worker.MaxBufferSize = 16 << 10

	cfg.WebServer.Directory = "web"
	cfg.WebServer.CertFile = "ca/cert.pem"
	cfg.WebServer.KeyFile = "ca/key.pem"
	cfg.WebServer.CertOpts.DNSNames = []string{"localhost"}
	cfg.WebServer.CertOpts.IPAddresses = []string{"127.0.0.1", "::1"}
	cfg.WebServer.Network = "tcp"
	cfg.WebServer.Address = "localhost:1657"
	cfg.WebServer.Username = "pbnet" // # super user, password = "pbnet"
	cfg.WebServer.Password = "$2a$12$zWgjYi0aAq.958UtUyDi5.QDmq4LOWsvv7I9ulvf1rHzd9/dWWmTi"

	cfg.Test.SkipTestClientDNS = true
	cfg.Test.SkipSynchronizeTime = true
	return &cfg
}

func TestConfig(t *testing.T) {
	var (
		data []byte
		err  error
	)
	for _, path := range []string{
		"testdata/config.toml",
		"../controller/testdata/config.toml",
	} {
		data, err = ioutil.ReadFile(path)
		if err == nil {
			break
		}
	}
	require.NotEmpty(t, data)

	// check unnecessary field
	cfg := Config{}
	err = toml.Unmarshal(data, &cfg)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, cfg)

	for _, testdata := range [...]*struct {
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

		{expected: 15 * time.Second, actual: cfg.Client.Timeout},
		{expected: "test", actual: cfg.Client.ProxyTag},
		{expected: "custom", actual: cfg.Client.DNSOpts.Mode},
		{expected: "test.com", actual: cfg.Client.TLSConfig.ServerName},

		{expected: 7, actual: cfg.Sender.MaxConns},
		{expected: 64, actual: cfg.Sender.Worker},
		{expected: 15 * time.Second, actual: cfg.Sender.Timeout},
		{expected: 512, actual: cfg.Sender.QueueSize},
		{expected: 16384, actual: cfg.Sender.MaxBufferSize},

		{expected: 30 * time.Second, actual: cfg.Syncer.ExpireTime},

		{expected: 64, actual: cfg.Worker.Number},
		{expected: 512, actual: cfg.Worker.QueueSize},
		{expected: 16384, actual: cfg.Worker.MaxBufferSize},

		{expected: "web", actual: cfg.WebServer.Directory},
		{expected: "ca/cert.pem", actual: cfg.WebServer.CertFile},
		{expected: "ca/key.pem", actual: cfg.WebServer.KeyFile},
		{expected: []string{"localhost"}, actual: cfg.WebServer.CertOpts.DNSNames},
		{expected: "tcp4", actual: cfg.WebServer.Network},
		{expected: "localhost:1657", actual: cfg.WebServer.Address},
		{expected: "pbnet", actual: cfg.WebServer.Username},
		{expected: "bcrypt", actual: cfg.WebServer.Password},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
