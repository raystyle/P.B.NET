package controller

import (
	"time"
)

func testGenerateConfig() *Config {
	c := Config{
		Debug: Debug{
			SkipTimeSyncer: true,
		},
		LogLevel: "debug",
	}
	c.Database.Dialect = "mysql"
	c.Database.DSN = "root:123456@tcp(127.0.0.1:3306)/p.b.net_test?loc=Local&parseTime=true"
	c.Database.MaxOpenConns = 16
	c.Database.MaxIdleConns = 16
	c.Database.LogFile = "../app/log/database.log"
	c.Database.GORMLogFile = "../app/log/gorm.log"
	c.Database.GORMDetailedLog = false

	c.Global.BuiltinDir = "../app/builtin"
	c.Global.KeyDir = "../app/key"
	c.Global.DNSCacheExpire = 3 * time.Minute
	c.Global.TimeSyncInterval = 1 * time.Minute

	c.Sender.MaxBufferSize = 16384
	c.Sender.Worker = 64
	c.Sender.QueueSize = 512
	c.Sender.MaxConns = 3

	c.Syncer.MaxBufferSize = 16384
	c.Syncer.Worker = 64
	c.Syncer.QueueSize = 512
	c.Syncer.ExpireTime = 3 * time.Minute

	c.Web.Dir = "../app/web"
	c.Web.CertFile = "../app/cert/server.crt"
	c.Web.KeyFile = "../app/cert/server.key"
	c.Web.Address = "localhost:9931"
	c.Web.Username = "admin"
	c.Web.Password = "56c10b0f6a18abe0247c31fd1d1a70e51e5a09f2"
	return &c
}
