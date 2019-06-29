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
			model: &m_bootstrapper{},
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

func (this *CTRL) Insert_Proxy_Client(m *m_proxy_client) error {
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

func (this *CTRL) Insert_DNS_Client(m *m_dns_client) error {
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

func (this *CTRL) Insert_Timesync(m *m_timesync) error {
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

func (this *CTRL) Insert_Bootstrapper(m *m_bootstrapper) error {
	return this.db.Create(m).Error
}

func (this *CTRL) Select_Bootstrapper() ([]*m_bootstrapper, error) {
	var clients []*m_bootstrapper
	return clients, this.db.Find(&clients).Error
}

func (this *CTRL) Update_Bootstrapper(m *m_bootstrapper) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_Bootstrapper(id uint64) error {
	return this.db.Delete(&m_bootstrapper{ID: id}).Error
}

// ----------------------------------listener----------------------------------------

func (this *CTRL) Insert_Listener(m *m_listener) error {
	return this.db.Create(m).Error
}

func (this *CTRL) Select_Listener() ([]*m_listener, error) {
	var clients []*m_listener
	return clients, this.db.Find(&clients).Error
}

func (this *CTRL) Update_Listener(m *m_listener) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_Listener(id uint64) error {
	return this.db.Delete(&m_listener{ID: id}).Error
}
