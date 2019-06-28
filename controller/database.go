package controller

import (
	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/xreflect"
)

func init() {
	// gorm custom namer: table name delete "m_"
	// table "m_proxy_client" -> "proxy_client"
	default_namer := gorm.TheNamingStrategy.Table
	gorm.TheNamingStrategy.Table = func(name string) string {
		return default_namer(name)[2:]
	}
}

func connect_database(c *Config) (*gorm.DB, error) {
	// set db logger
	db_logger, err := new_db_logger(c.Dialect, c.DB_Log_Path)
	if err != nil {
		return nil, errors.Wrapf(err, "create %s logger failed", c.Dialect)
	}
	// if you need, add DB Driver
	switch c.Dialect {
	case "mysql":
		_ = mysql.SetLogger(db_logger)
	default:
		return nil, errors.Errorf("unknown dialect: %s", c.Dialect)
	}
	// connect database
	db, err := gorm.Open(c.Dialect, c.DSN)
	if err != nil {
		return nil, errors.Wrapf(err, "connect %s server failed", c.Dialect)
	}
	// connection
	db.DB().SetMaxOpenConns(c.DB_Max_Open_Conns)
	db.DB().SetMaxIdleConns(c.DB_Max_Idle_Conns)
	// logger
	gorm_l, err := new_gorm_logger(c.GORM_Log_Path)
	if err != nil {
		return nil, errors.Wrap(err, "create gorm logger failed")
	}
	db.SetLogger(gorm_l)
	if c.GORM_Detailed_Log {
		db.LogMode(true)
	}
	// not add s
	db.SingularTable(true)
	return db, nil
}

// first use this project
func init_database(db *gorm.DB) error {
	tables := []*struct {
		name  string
		model interface{}
	}{
		{
			name:  t_ctrl_log,
			model: &m_ctrl_log{},
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

func (this *CTRL) Init_Database() error {
	return init_database(this.db)
}

// -------------------------------proxy client----------------------------------------

func (this *CTRL) Insert_Proxy_Client(tag, mode, config string) error {
	m := &m_proxy_client{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Create(m).Error
}

func (this *CTRL) Select_Proxy_Client() ([]*m_proxy_client, error) {
	var clients []*m_proxy_client
	return clients, this.db.Find(&clients).Error
}

func (this *CTRL) Update_Proxy_Client(m *m_proxy_client) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_Proxy_Client(id uint64) error {
	return this.db.Delete(&m_proxy_client{ID: id}).Error
}

// -------------------------------dns client----------------------------------------

func (this *CTRL) Insert_DNS_Client(tag, method, address string) error {
	m := &m_dns_client{
		Tag:     tag,
		Method:  method,
		Address: address,
	}
	return this.db.Create(m).Error
}

func (this *CTRL) Select_DNS_Client() ([]*m_dns_client, error) {
	var clients []*m_dns_client
	return clients, this.db.Find(&clients).Error
}

func (this *CTRL) Update_DNS_Client(m *m_dns_client) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_DNS_Client(id uint64) error {
	return this.db.Delete(&m_dns_client{ID: id}).Error
}

// ---------------------------------timesync----------------------------------------

func (this *CTRL) Insert_Timesync(tag, mode, config string) error {
	m := &m_timesync{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Create(m).Error
}

func (this *CTRL) Select_Timesync() ([]*m_timesync, error) {
	var clients []*m_timesync
	return clients, this.db.Find(&clients).Error
}

func (this *CTRL) Update_Timesync(m *m_timesync) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_Timesync(id uint64) error {
	return this.db.Delete(&m_timesync{ID: id}).Error
}

// ---------------------------------bootstrap----------------------------------------

// interval = second
func (this *CTRL) Insert_Bootstrap(tag, mode, config string,
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

// ----------------------------------listener----------------------------------------

func (this *CTRL) Insert_Listener(tag, mode, config string) error {
	m := &m_listener{
		Tag:    tag,
		Mode:   mode,
		Config: config,
	}
	return this.db.Create(m).Error
}
