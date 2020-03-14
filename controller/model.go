package controller

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/security"
	"project/internal/xreflect"
)

// set gorm.TheNamingStrategy.Table.
// gorm custom name: table name delete "m"
// table "mProxyClient" -> "m_proxy_client" -> "proxy_client"
func init() {
	n := gorm.TheNamingStrategy.Table
	gorm.TheNamingStrategy.Table = func(name string) string {
		return n(name)[2:]
	}
}

// different table with the same model.
const (
	tableCtrlLog   = "log"
	tableNodeLog   = "node_log"
	tableBeaconLog = "beacon_log"
)

// 32 = guid.Size in internal/guid/guid.go

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
	Source    string     `gorm:"not null;size:128" sql:"index"`
	Log       []byte     `gorm:"not null;type:mediumblob"`
	DeletedAt *time.Time `sql:"index"`
}

type mProxyClient struct {
	ID      uint64 `gorm:"primary_key"`
	Tag     string `gorm:"not null;size:128;unique"`
	Mode    string `gorm:"not null;size:32"`
	Network string `gorm:"not null;size:128"`
	Address string `gorm:"not null;size:4096"`
	Options string `gorm:"not null;size:1048576"`
	Model
}

type mDNSServer struct {
	ID       uint64 `gorm:"primary_key"`
	Tag      string `gorm:"not null;size:128;unique"`
	Method   string `gorm:"not null;size:32"`
	Address  string `gorm:"not null;size:4096"`
	SkipTest bool   `gorm:"not null"`
	Model
}

type mTimeSyncer struct {
	ID       uint64 `gorm:"primary_key"`
	Tag      string `gorm:"not null;size:128;unique"`
	Mode     string `gorm:"not null;size:32"`
	Config   string `gorm:"not null;size:1048576"`
	SkipTest bool   `gorm:"not null"`
	Model
}

type mBoot struct {
	ID       uint64 `gorm:"primary_key"`
	Tag      string `gorm:"not null;size:128;unique"`
	Mode     string `gorm:"not null;size:32"`
	Config   string `gorm:"not null;size:1048576"`
	Interval uint32 `gorm:"not null"`
	Enable   bool   `gorm:"not null"`
	Model
}

type mListener struct {
	ID      uint64 `gorm:"primary_key"`
	Tag     string `gorm:"not null;size:128;unique"`
	Mode    string `gorm:"not null;size:32"`
	Timeout uint32 `gorm:"not null"`
	Config  string `gorm:"not null;size:1048576"`
	Model
}

type mNode struct {
	ID           uint64          `gorm:"primary_key"`
	GUID         []byte          `gorm:"not null;type:binary(32);unique" sql:"index"`
	PublicKey    []byte          `gorm:"not null;type:binary(32)"`
	KexPublicKey []byte          `gorm:"not null;type:binary(32)"`
	SessionKey   *security.Bytes `gorm:"-"` // when first query, it will be calculated
	IsBootstrap  bool            `gorm:"not null"`
	CreatedAt    time.Time       `gorm:"not null"`
	DeletedAt    *time.Time      `sql:"index"`
}

type mNodeListener struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"not null;type:binary(32)" sql:"index"`
	Tag       string     `gorm:"not null;size:32"`
	Mode      string     `gorm:"not null;size:32"`
	Network   string     `gorm:"not null;size:32"`
	Address   string     `gorm:"not null;size:4096"`
	CreatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type mBeacon struct {
	ID           uint64          `gorm:"primary_key"`
	GUID         []byte          `gorm:"not null;type:binary(32);unique" sql:"index"`
	PublicKey    []byte          `gorm:"not null;type:binary(32)"`
	KexPublicKey []byte          `gorm:"not null;type:binary(32)"`
	SessionKey   *security.Bytes `gorm:"-"` // when first query, it will be calculated
	CreatedAt    time.Time       `gorm:"not null"`
	DeletedAt    *time.Time      `sql:"index"`
}

type mBeaconListener struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"not null;type:binary(32)" sql:"index"`
	Tag       string     `gorm:"not null;size:32"`
	Mode      string     `gorm:"not null;size:32"`
	Network   string     `gorm:"not null;size:32"`
	Address   string     `gorm:"not null;size:4096"`
	CreatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type mBeaconMessage struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"not null;type:binary(32)" sql:"index"`
	Index     uint64     `gorm:"not null" sql:"index"`
	Hash      []byte     `gorm:"not null;type:binary(32)"`
	Deflate   byte       `gorm:"not null;type:tinyint unsigned"`
	Message   []byte     `gorm:"not null;type:mediumblob"`
	CreatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

// <security> must use new table to set message index to each Beacon,
// because use mBeaconMessage.ID maybe expose scale.
type mBeaconMessageIndex struct {
	ID    uint64 `gorm:"primary_key"`
	GUID  []byte `gorm:"not null;type:binary(32);unique" sql:"index"`
	Index uint64 `gorm:"not null"`
	Model
}

// beacon & node log
type mRoleLog struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"not null;type:binary(32)" sql:"index"`
	CreatedAt time.Time  `gorm:"not null"`
	Level     uint8      `gorm:"not null"`
	Source    string     `gorm:"not null;size:128"`
	Log       []byte     `gorm:"not null;type:mediumblob"`
	DeletedAt *time.Time `sql:"index"`
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

	// table name will not add "s"
	db.SingularTable(true)
	db.LogMode(false)
	defer func() { _ = db.Close() }()
	tables := []*struct {
		name  string
		model interface{}
	}{
		// about controller
		{name: tableCtrlLog, model: &mCtrlLog{}},
		{model: &mProxyClient{}},
		{model: &mDNSServer{}},
		{model: &mTimeSyncer{}},
		{model: &mBoot{}},
		{model: &mListener{}},

		// about node
		{model: &mNode{}},
		{model: &mNodeListener{}},
		{name: tableNodeLog, model: &mRoleLog{}},

		// about beacon
		{model: &mBeacon{}},
		{model: &mBeaconMessage{}},
		{model: &mBeaconMessageIndex{}},
		{model: &mBeaconListener{}},
		{name: tableBeaconLog, model: &mRoleLog{}},
	}
	l := len(tables)
	// because of foreign key, drop tables by inverted order
	for i := l - 1; i > -1; i-- {
		const format = "failed to drop table %s"
		name := tables[i].name
		model := tables[i].model
		if name == "" {
			err = db.DropTableIfExists(model).Error
			if err != nil {
				table := gorm.ToTableName(xreflect.GetStructureName(model))
				return errors.Wrapf(err, format, table)
			}
		} else {
			err = db.Table(name).DropTableIfExists(model).Error
			if err != nil {
				table := gorm.ToTableName(xreflect.GetStructureName(model))
				return errors.Wrapf(err, format, table)
			}
		}
	}
	// create tables
	for i := 0; i < l; i++ {
		const format = "failed to create table %s"
		name := tables[i].name
		model := tables[i].model
		if name == "" {
			err = db.CreateTable(model).Error
			if err != nil {
				table := gorm.ToTableName(xreflect.GetStructureName(model))
				return errors.Wrapf(err, format, table)
			}
		} else {
			err = db.Table(name).CreateTable(model).Error
			if err != nil {
				return errors.Wrapf(err, format, name)
			}
		}
	}
	return initializeDatabaseForeignKey(db)
}

func initializeDatabaseForeignKey(db *gorm.DB) error {
	const (
		field    = "guid"
		onDelete = "CASCADE"
		onUpdate = "CASCADE"
	)
	// add node foreign key
	for _, model := range []*gorm.DB{
		db.Model(&mNodeListener{}),
		db.Table(tableNodeLog).Model(&mRoleLog{}),
	} {
		err := model.AddForeignKey(field, "node(guid)", onDelete, onUpdate).Error
		if err != nil {
			return errors.Wrap(err, "failed to add node foreign key")
		}
	}
	// add beacon foreign key
	for _, model := range []*gorm.DB{
		db.Model(&mBeaconMessage{}),
		db.Model(&mBeaconMessageIndex{}),
		db.Model(&mBeaconListener{}),
		db.Table(tableBeaconLog).Model(&mRoleLog{}),
	} {
		err := model.AddForeignKey(field, "beacon(guid)", onDelete, onUpdate).Error
		if err != nil {
			return errors.Wrap(err, "failed to add beacon foreign key")
		}
	}
	return nil
}
