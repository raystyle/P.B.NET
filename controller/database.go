package controller

import (
	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

type database struct {
	ctx *CONTROLLER
	db  *gorm.DB
}

func new_database(ctx *CONTROLLER) (*database, error) {
	d := &database{
		ctx: ctx,
	}
	// set logger
	mysql_l, err := new_db_logger("mysql", ctx.config.Database_Log)
	if err != nil {
		return nil, errors.Wrap(err, "create db logger failed")
	}
	_ = mysql.SetLogger(mysql_l)
	return d, nil
}

func (this *database) Connect() error {
	config := this.ctx.config
	// set logger
	gorm_l, err := new_gorm_logger(config.Gorm_Log)
	if err != nil {
		return errors.Wrap(err, "create gorm logger failed")
	}
	db, err := gorm.Open(config.Database, config.DSN)
	if err != nil {
		return err
	}
	this.db = db
	db.SetLogger(gorm_l)
	db.LogMode(true)
	db.DB().SetMaxOpenConns(config.DB_Max_Open_Conns)
	db.DB().SetMaxIdleConns(config.DB_Max_Idle_Conn)
	err = this.init()
	if err != nil {
		return err
	}
	return nil
}

func (this *database) init() error {
	db := this.db
	db.SingularTable(true)
	// proxy client
	db.Exec("DROP TABLE proxy_client")
	db.AutoMigrate(&proxy_client{})
	a := &proxy_client{Tag: "test_socks5", Mode: "socks5", Config: "toml"}

	db.Create(a)

	asd := &proxy_client{}

	db.First(asd)

	return nil
}
