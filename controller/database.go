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
	mysql_l, err := new_db_logger("mysql", ctx.config.DB_Log)
	if err != nil {
		return nil, errors.Wrap(err, "create db logger failed")
	}
	_ = mysql.SetLogger(mysql_l)
	return d, nil
}

func (this *database) Connect() error {
	config := this.ctx.config
	// set logger
	gorm_l, err := new_gorm_logger(config.GORM_Log)
	if err != nil {
		return errors.Wrap(err, "create gorm logger failed")
	}
	db, err := gorm.Open(config.Dialect, config.DSN)
	if err != nil {
		return err
	}
	this.db = db
	db.SetLogger(gorm_l)
	db.LogMode(true)
	db.DB().SetMaxOpenConns(config.DB_Max_Open_Conns)
	db.DB().SetMaxIdleConns(config.DB_Max_Idle_Conn)
	err = this.init_db()
	if err != nil {
		return err
	}
	return nil
}

func (this *database) init_db() error {
	db := this.db
	db.SingularTable(true)
	// proxy client
	db.Exec("DROP TABLE proxy_client")
	err := db.Table("proxy_client").CreateTable(&m_proxy_client{}).Error
	if err != nil {
		return errors.Wrap(err, "create table proxy_client failed")
	}
	// dns client
	db.Exec("DROP TABLE dns_client")
	err = db.Table("dns_client").CreateTable(&m_dns_client{}).Error
	if err != nil {
		return errors.Wrap(err, "create table dns_client failed")
	}
	// timesync
	db.Exec("DROP TABLE timesync")
	err = db.Table("timesync").CreateTable(&m_timesync{}).Error
	if err != nil {
		return errors.Wrap(err, "create table timesync failed")
	}
	// bootstrap
	db.Exec("DROP TABLE bootstrap")
	err = db.Table("bootstrap").CreateTable(&m_bootstrap{}).Error
	if err != nil {
		return errors.Wrap(err, "create table bootstrap failed")
	}
	// node listener
	db.Exec("DROP TABLE node_listener")
	err = db.Table("node_listener").CreateTable(&m_listener{}).Error
	if err != nil {
		return errors.Wrap(err, "create table node_listener failed")
	}
	return nil
}
