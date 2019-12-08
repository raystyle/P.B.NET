package controller

import (
	"reflect"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

func init() {
	// gorm custom name: table name delete "m"
	// table "mProxyClient" -> "m_proxy_client" -> "proxy_client"
	n := gorm.TheNamingStrategy.Table
	gorm.TheNamingStrategy.Table = func(name string) string {
		return n(name)[2:]
	}
}

// different table have the same model
const (
	tableCtrlLog   = "log"
	tableNodeLog   = "node_log"
	tableBeaconLog = "beacon_log"
)

// Model include time, most model need it
type Model struct {
	CreatedAt time.Time  `gorm:"not null"`
	UpdatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type mCtrlLog struct {
	ID        uint64     `gorm:"primary_key"`
	CreatedAt time.Time  `gorm:"not null"`
	Level     uint8      `gorm:"not null" sql:"index"`
	Source    string     `gorm:"size:32;not null" sql:"index"`
	Log       string     `gorm:"size:16000;not null"`
	DeletedAt *time.Time `sql:"index"`
}

type mProxyClient struct {
	ID      uint64 `gorm:"primary_key"`
	Tag     string `gorm:"size:32;not null;unique"`
	Mode    string `gorm:"size:32;not null"`
	Network string `gorm:"size:32;not null"`
	Address string `gorm:"size:1024;not null"`
	Options string `gorm:"size:1048576;not null"`
	Model
}

type mDNSServer struct {
	ID       uint64 `gorm:"primary_key"`
	Tag      string `gorm:"size:32;not null;unique"`
	Method   string `gorm:"size:32;not null"`
	Address  string `gorm:"size:2048;not null"`
	SkipTest bool   `gorm:"not null"`
	Model
}

type mTimeSyncer struct {
	ID       uint64 `gorm:"primary_key"`
	Tag      string `gorm:"size:32;not null;unique"`
	Mode     string `gorm:"size:32;not null"`
	Config   string `gorm:"size:16000;not null"`
	SkipTest bool   `gorm:"not null"`
	Model
}

type mBoot struct {
	ID       uint64 `gorm:"primary_key"`
	Tag      string `gorm:"size:32;not null;unique"`
	Mode     string `gorm:"size:32;not null"`
	Config   string `gorm:"size:16000;not null"`
	Interval uint32 `gorm:"not null"`
	Enable   bool   `gorm:"not null"`
	Model
}

type mListener struct {
	ID      uint64 `gorm:"primary_key"`
	Tag     string `gorm:"size:32;not null;unique"`
	Mode    string `gorm:"size:32;not null"`
	Timeout uint32 `gorm:"not null"`
	Config  string `gorm:"size:16000;not null"`
	Model
}

type mNode struct {
	ID          uint64     `gorm:"primary_key"`
	GUID        []byte     `gorm:"type:binary(52);not null" sql:"index"`
	PublicKey   []byte     `gorm:"type:binary(32);not null"`
	SessionKey  []byte     `gorm:"type:binary(32);not null"`
	IsBootstrap bool       `gorm:"not null"`
	CreatedAt   time.Time  `gorm:"not null"`
	DeletedAt   *time.Time `sql:"index"`
}

type mNodeListener struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"type:binary(52);not null" sql:"index"`
	Tag       string     `gorm:"size:32;not null"`
	Mode      string     `gorm:"size:32;not null"`
	Network   string     `gorm:"size:32;not null"`
	Address   string     `gorm:"size:2048;not null"`
	CreatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type mBeacon struct {
	ID         uint64     `gorm:"primary_key"`
	GUID       []byte     `gorm:"type:binary(52);not null" sql:"index"`
	PublicKey  []byte     `gorm:"type:binary(32);not null"`
	SessionKey []byte     `gorm:"type:binary(32);not null"`
	CreatedAt  time.Time  `gorm:"not null"`
	DeletedAt  *time.Time `sql:"index"`
}

type mBeaconListener struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"type:binary(52);not null" sql:"index"`
	Tag       string     `gorm:"size:32;not null"`
	Mode      string     `gorm:"size:32;not null"`
	Network   string     `gorm:"size:32;not null"`
	Address   string     `gorm:"size:2048;not null"`
	CreatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type mBeaconMessage struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"not null;type:binary(52)"`
	Message   []byte     `gorm:"not null;type:MEDIUMBLOB"`
	CreatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

// 52 = internal/guid/guid.go  guid.Size
// beacon & node log
type mRoleLog struct {
	ID        uint64     `gorm:"primary_key"`
	CreatedAt time.Time  `gorm:"not null"`
	GUID      []byte     `gorm:"type:binary(52);not null" sql:"index"`
	Level     uint8      `gorm:"not null"`
	Source    string     `gorm:"size:32;not null"`
	Log       string     `gorm:"size:16000;not null"`
	DeletedAt *time.Time `sql:"index"`
}

type mTrustNode struct {
	Mode    string `json:"mode"`
	Network string `json:"network"`
	Address string `json:"address"`
}

func getStructureName(v interface{}) string {
	s := reflect.TypeOf(v).String()
	ss := strings.Split(s, ".")
	return ss[len(ss)-1]
}

// InitializeDatabase is used to initialize database
func InitializeDatabase(config *Config) error {
	cfg := config.Database

	// connect database
	db, err := gorm.Open(cfg.Dialect, cfg.DSN)
	if err != nil {
		return errors.Wrapf(err, "failed to connect %s server", cfg.Dialect)
	}
	err = db.DB().Ping()
	if err != nil {
		return errors.Wrapf(err, "failed to ping %s server", cfg.Dialect)
	}
	// not add s
	db.SingularTable(true)
	db.LogMode(false)
	defer func() { _ = db.Close() }()
	tables := []*struct {
		name  string
		model interface{}
	}{
		{
			name:  tableCtrlLog,
			model: &mCtrlLog{},
		},
		{
			model: &mProxyClient{},
		},
		{
			model: &mDNSServer{},
		},
		{
			model: &mTimeSyncer{},
		},
		{
			model: &mBoot{},
		},
		{
			model: &mListener{},
		},
		{
			model: &mNode{},
		},
		{
			model: &mNodeListener{},
		},
		{
			name:  tableNodeLog,
			model: &mRoleLog{},
		},
		{
			model: &mBeacon{},
		},
		{
			model: &mBeaconMessage{},
		},
		{
			model: &mBeaconListener{},
		},
		{
			name:  tableBeaconLog,
			model: &mRoleLog{},
		},
	}
	l := len(tables)
	// because of foreign key, drop tables by inverted order
	for i := l - 1; i > -1; i-- {
		n := tables[i].name
		m := tables[i].model
		if n == "" {
			err = db.DropTableIfExists(m).Error
			if err != nil {
				table := gorm.ToTableName(getStructureName(m))
				return errors.Wrapf(err, "failed to drop table %s", table)
			}
		} else {
			err = db.Table(n).DropTableIfExists(m).Error
			if err != nil {
				table := gorm.ToTableName(getStructureName(m))
				return errors.Wrapf(err, "failed to drop table %s", table)
			}
		}
	}
	// create tables
	for i := 0; i < l; i++ {
		n := tables[i].name
		m := tables[i].model
		if n == "" {
			err = db.CreateTable(m).Error
			if err != nil {
				table := gorm.ToTableName(getStructureName(m))
				return errors.Wrapf(err, "failed to create table %s", table)
			}
		} else {
			err = db.Table(n).CreateTable(m).Error
			if err != nil {
				return errors.Wrapf(err, "failed to create table %s", n)
			}
		}
	}
	// add node foreign key
	addErr := func(table string, err error) error {
		return errors.Wrapf(err, "failed to add %s foreign key", table)
	}
	table := gorm.ToTableName(getStructureName(&mNode{}))
	err = db.Model(&mNodeListener{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(table, err)
	}
	err = db.Table(tableNodeLog).Model(&mRoleLog{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(table, err)
	}
	// add beacon foreign key
	table = gorm.ToTableName(getStructureName(&mBeacon{}))
	err = db.Model(&mBeaconMessage{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(table, err)
	}
	err = db.Model(&mBeaconListener{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(table, err)
	}
	err = db.Table(tableBeaconLog).Model(&mRoleLog{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(table, err)
	}
	return nil
}
