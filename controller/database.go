package controller

import (
	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/xreflect"
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
	// custom namer
	// table name like m_proxy_client so delete "m_"
	default_namer := gorm.TheNamingStrategy.Table
	gorm.TheNamingStrategy.Table = func(name string) string {
		return default_namer(name)[2:]
	}
	db.DB().SetMaxOpenConns(config.DB_Max_Open_Conns)
	db.DB().SetMaxIdleConns(config.DB_Max_Idle_Conn)
	db.SetLogger(gorm_l)
	db.LogMode(false)
	db.SingularTable(true)
	return nil
}

// -------------------------------controller log--------------------------------------

func (this *database) Select_Ctrl_Log(args ...interface{}) *gorm.DB {
	return this.db.Model(&m_controller_log{})
}

func (this *database) Insert_Ctrl_Log(level uint8, src, log string) error {
	m := &m_controller_log{
		Level:  level,
		Source: src,
		Log:    log,
	}
	return this.db.Create(m).Error
}

func (this *database) Delete_Ctrl_Log(where ...interface{}) error {
	return this.db.Delete(&m_controller_log{}, where...).Error
}

// -------------------------------proxy client----------------------------------------

func (this *database) Select_Proxy_Client(tag, mode, config string) error {
	m := &m_proxy_client{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Create(m).Error
}

func (this *database) Update_Proxy_Client(tag, mode, config string) error {
	m := &m_proxy_client{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Create(m).Error
}

func (this *database) Insert_Proxy_Client(tag, mode, config string) error {
	m := &m_proxy_client{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Create(m).Error
}

func (this *database) Delete_Proxy_Client(tag, mode, config string) error {
	m := &m_proxy_client{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Create(m).Error
}

// -------------------------------dns client----------------------------------------

func (this *database) Insert_DNS_Client(tag, method, address string) error {
	m := &m_dns_client{
		Tag:     tag,
		Method:  method,
		Address: address,
	}
	return this.db.Create(m).Error
}

func (this *database) Insert_Timesync(tag, mode, config string) error {
	m := &m_timesync{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Create(m).Error
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
	return this.db.Create(m).Error
}

func (this *database) Insert_Listener(tag, mode, config string) error {
	m := &m_listener{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Create(m).Error
}

func (this *database) Insert_Node_Log(guid []byte, level uint8, src, log string) error {
	m := &m_role_log{
		GUID:   guid,
		Level:  level,
		Source: src,
		Log:    log,
	}
	return this.db.Table(t_node_log).Create(m).Error
}

func (this *database) Insert_Beacon_Log(guid []byte, level uint8, src, log string) error {
	m := &m_role_log{
		GUID:   guid,
		Level:  level,
		Source: src,
		Log:    log,
	}
	return this.db.Table(t_beacon_log).Create(m).Error
}

// first use this project
func (this *database) init_db() error {
	db := this.db
	tables := []*struct {
		name  string
		model interface{}
	}{
		{
			name:  "",
			model: &m_controller_log{},
		},
		{
			name:  "",
			model: &m_proxy_client{},
		},
		{
			name:  "",
			model: &m_dns_client{},
		},
		{
			name:  "",
			model: &m_timesync{},
		},
		{
			name:  "",
			model: &m_bootstrap{},
		},
		{
			name:  "",
			model: &m_listener{},
		},
		{
			name:  t_node_log,
			model: &m_role_log{},
		},
		{
			name:  t_beacon_log,
			model: &m_role_log{},
		},
	}
	for i := 0; i < len(tables); i++ {
		n := tables[i].name
		m := tables[i].model
		if n == "" {
			db.DropTableIfExists(m)
			err := db.CreateTable(m).Error
			if err != nil {
				name := gorm.TheNamingStrategy.Table(xreflect.Struct_Name(m))
				return errors.Wrapf(err, "create table %s failed", name)
			}
		} else {
			db.Table(n).DropTableIfExists(m)
			err := db.Table(n).CreateTable(m).Error
			if err != nil {
				return errors.Wrapf(err, "create table %s failed", n)
			}
		}
	}
	return nil
}
