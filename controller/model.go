package controller

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/config"
	"project/internal/xnet"
	"project/internal/xreflect"
)

// different table have the same model
const (
	tableLog       = "log"
	tableNodeLog   = "node_log"
	tableBeaconLog = "beacon_log"
)

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
	ID     uint64 `gorm:"primary_key"`
	Tag    string `gorm:"size:32;not null;unique"`
	Mode   string `gorm:"size:32;not null"`
	Config string `gorm:"size:16000;not null"`
	Model
}

type mDNSServer struct {
	ID      uint64 `gorm:"primary_key"`
	Tag     string `gorm:"size:32;not null;unique"`
	Method  string `gorm:"size:32;not null"`
	Address string `gorm:"size:2048;not null"`
	Model
}

type mTimeSyncer struct {
	ID     uint64 `gorm:"primary_key"`
	Tag    string `gorm:"size:32;not null;unique"`
	Mode   string `gorm:"size:32;not null"`
	Config string `gorm:"size:16000;not null"`
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

func (ml *mListener) Configure() *config.Listener {
	l := &config.Listener{
		Tag:    ml.Tag,
		Mode:   ml.Mode,
		Config: []byte(ml.Config),
	}
	l.Timeout = time.Duration(ml.Timeout) * time.Second
	return l
}

type mNode struct {
	GUID        []byte     `gorm:"primary_key;type:binary(52)"`
	PublicKey   []byte     `gorm:"type:binary(32);not null"`
	SessionKey  []byte     `gorm:"type:binary(32);not null"`
	IsBootstrap bool       `gorm:"not null"`
	CreatedAt   time.Time  `gorm:"not null"`
	DeletedAt   *time.Time `sql:"index"`
}

type mNodeSyncer struct {
	GUID      []byte     `gorm:"primary_key;type:binary(52)"`
	CtrlSend  uint64     `gorm:"not null;column:controller_send"`
	NodeRecv  uint64     `gorm:"not null;column:node_receive"`
	NodeSend  uint64     `gorm:"not null;column:node_send"`
	CtrlRecv  uint64     `gorm:"not null;column:controller_receive"`
	UpdatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type mNodeListener struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"type:binary(52);not null" sql:"index"`
	Tag       string     `gorm:"size:32;not null"`
	Mode      xnet.Mode  `gorm:"size:32;not null"`
	Network   string     `gorm:"size:32;not null"`
	Address   string     `gorm:"size:2048;not null"`
	CreatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type mBeacon struct {
	GUID       []byte     `gorm:"primary_key;type:binary(52)"`
	PublicKey  []byte     `gorm:"type:binary(32);not null"`
	SessionKey []byte     `gorm:"type:binary(32);not null"`
	CreatedAt  time.Time  `gorm:"not null"`
	DeletedAt  *time.Time `sql:"index"`
}

type mBeaconSyncer struct {
	GUID       []byte     `gorm:"primary_key;type:binary(52)"`
	CtrlSend   uint64     `gorm:"not null;column:controller_send"`
	BeaconRecv uint64     `gorm:"not null;column:beacon_receive"`
	BeaconSend uint64     `gorm:"not null;column:beacon_send"`
	CtrlRecv   uint64     `gorm:"not null;column:controller_receive"`
	UpdatedAt  time.Time  `gorm:"not null"`
	DeletedAt  *time.Time `sql:"index"`
}

type mBeaconListener struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"type:binary(52);not null" sql:"index"`
	Tag       string     `gorm:"size:32;not null"`
	Mode      xnet.Mode  `gorm:"size:32;not null"`
	Network   string     `gorm:"size:32;not null"`
	Address   string     `gorm:"size:2048;not null"`
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
	Mode    xnet.Mode `json:"mode"`
	Network string    `json:"network"`
	Address string    `json:"address"`
}

// InitDatabase is used to initialize database
// if first use this project
func InitDatabase(cfg *Config) error {
	// connect database
	db, err := gorm.Open(cfg.Dialect, cfg.DSN)
	if err != nil {
		return errors.Wrapf(err, "connect %s server failed", cfg.Dialect)
	}
	err = db.DB().Ping()
	if err != nil {
		return errors.Wrapf(err, "ping %s server failed", cfg.Dialect)
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
			name:  tableLog,
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
			model: &mNodeSyncer{},
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
			model: &mBeaconSyncer{},
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
				table := gorm.ToTableName(xreflect.StructName(m))
				return errors.Wrapf(err, "drop table %s failed", table)
			}
		} else {
			err = db.Table(n).DropTableIfExists(m).Error
			if err != nil {
				table := gorm.ToTableName(xreflect.StructName(m))
				return errors.Wrapf(err, "drop table %s failed", table)
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
				table := gorm.ToTableName(xreflect.StructName(m))
				return errors.Wrapf(err, "create table %s failed", table)
			}
		} else {
			err = db.Table(n).CreateTable(m).Error
			if err != nil {
				return errors.Wrapf(err, "create table %s failed", n)
			}
		}
	}
	// add node foreign key
	addErr := func(table string, err error) error {
		return errors.Wrapf(err, "add %s foreign key failed", table)
	}
	table := gorm.ToTableName(xreflect.StructName(&mNode{}))
	err = db.Model(&mNodeSyncer{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(table, err)
	}
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
	table = gorm.ToTableName(xreflect.StructName(&mBeacon{}))
	err = db.Model(&mBeaconSyncer{}).AddForeignKey("guid", table+"(guid)",
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
