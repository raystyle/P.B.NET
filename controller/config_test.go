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
	c.Database.GORMDetailedLog = true

	c.Logger.Level = "debug"
	c.Logger.File = "log/controller.log"
	c.Logger.Writer = os.Stdout

	c.Global.DNSCacheExpire = time.Minute
	c.Global.TimeSyncInterval = time.Minute

	c.Client.Timeout = 5 * time.Second

	c.Sender.MaxBufferSize = 16384
	c.Sender.Worker = 64
	c.Sender.QueueSize = 512
	c.Sender.MaxConns = 3

	c.Syncer.MaxBufferSize = 16384
	c.Syncer.Worker = 64
	c.Syncer.QueueSize = 512
	c.Syncer.ExpireTime = 3 * time.Minute

	c.Web.Dir = "../app/web"
	c.Web.CertFile = "../app/cert/cert.pem"
	c.Web.KeyFile = "../app/cert/.key"
	c.Web.Address = "localhost:9931"
	c.Web.Username = "admin"
	c.Web.Password = "56c10b0f6a18abe0247c31fd1d1a70e51e5a09f2"
	return &c
}

func TestConfig(t *testing.T) {

}
