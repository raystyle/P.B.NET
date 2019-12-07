package controller

import (
	"os"
	"testing"
	"time"
)

func testGenerateConfig() *Config {
	c := Config{}

	c.Debug.SkipTimeSyncer = true

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

	c.Sender.MaxConns = 3
	c.Sender.Worker = 64
	c.Sender.Timeout = 15 * time.Second
	c.Sender.QueueSize = 512
	c.Sender.MaxBufferSize = 16384

	c.Syncer.ExpireTime = 3 * time.Minute

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

}
