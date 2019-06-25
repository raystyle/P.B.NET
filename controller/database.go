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
	db.DB().SetMaxOpenConns(config.DB_Max_Open_Conns)
	db.DB().SetMaxIdleConns(config.DB_Max_Idle_Conn)
	db.SetLogger(gorm_l)
	db.LogMode(false)
	db.SingularTable(true)
	return nil
}

func (this *database) Insert_Controller_Log(level uint8, src, log string) error {
	m := &m_controller_log{
		Level:  level,
		Source: src,
		Log:    log,
	}
	return this.db.Table("controller_log").Create(m).Error
}

func (this *database) Insert_Proxy_Client(tag, mode, config string) error {
	m := &m_proxy_client{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Table("proxy_client").Create(m).Error
}

func (this *database) Insert_DNS_Client(tag, method, address string) error {
	m := &m_dns_client{
		Tag:     tag,
		Method:  method,
		Address: address,
	}
	return this.db.Table("dns_client").Create(m).Error
}

func (this *database) Insert_Timesync(tag, mode, config string) error {
	m := &m_timesync{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Table("timesync").Create(m).Error
}

// interval = second
func (this *database) Insert_Bootstrap(tag, mode, config string,
	interval uint32, enable bool) error {
	m := &m_bootstrap{
		Tag:      tag,
		Mode:     mode,
		Config:   config,
		Interval: interval,
		Enable:   enable,
	}
	return this.db.Table("bootstrap").Create(m).Error
}

func (this *database) Insert_Listener(tag, mode, config string) error {
	m := &m_listener{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Table("listener").Create(m).Error
}

func (this *database) Insert_Node_Log(guid []byte, level uint8, src, log string) error {
	m := &m_role_log{
		GUID:   guid,
		Level:  level,
		Source: src,
		Log:    log,
	}
	return this.db.Table("node_log").Create(m).Error
}

func (this *database) Insert_Beacon_Log(guid []byte, level uint8, src, log string) error {
	m := &m_role_log{
		GUID:   guid,
		Level:  level,
		Source: src,
		Log:    log,
	}
	return this.db.Table("beacon_log").Create(m).Error
}

// first use this project
func (this *database) init_db() error {
	db := this.db
	// controller log
	db.Exec("DROP TABLE controller_log")
	err := db.Table("controller_log").CreateTable(&m_controller_log{}).Error
	if err != nil {
		return errors.Wrap(err, "create table controller_log failed")
	}
	// proxy client
	db.Exec("DROP TABLE proxy_client")
	err = db.Table("proxy_client").CreateTable(&m_proxy_client{}).Error
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
	// listener
	db.Exec("DROP TABLE listener")
	err = db.Table("listener").CreateTable(&m_listener{}).Error
	if err != nil {
		return errors.Wrap(err, "create table listener failed")
	}
	// node log
	db.Exec("DROP TABLE node_log")
	err = db.Table("node_log").CreateTable(&m_role_log{}).Error
	if err != nil {
		return errors.Wrap(err, "create table node_log failed")
	}
	// beacon log
	db.Exec("DROP TABLE beacon_log")
	err = db.Table("beacon_log").CreateTable(&m_role_log{}).Error
	if err != nil {
		return errors.Wrap(err, "create table beacon_log failed")
	}
	return nil
}
