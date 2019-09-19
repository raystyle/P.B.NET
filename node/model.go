package node

import (
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/xnet"
	"project/internal/xreflect"
)

// different table have the same model
const (
	tableCNMessage = "cn_message" // CN = Controller -> Node
	tableNCMessage = "nc_message" // NC = Node -> Controller
	tableCBMessage = "cb_message" // CB = Controller -> Beacon
	tableBCMessage = "bc_message" // BC = Beacon -> Controller
)

// node sync and used to controller recovery
type mNode struct {
	GUID         []byte `gorm:"primary_key;type:blob(52)"`
	PublicKey    []byte `gorm:"type:blob(32);not null"`
	KexPublicKey []byte `gorm:"type:blob(32);not null"`
}

type mBeacon struct {
	GUID         []byte `gorm:"primary_key;type:blob(52)"`
	PublicKey    []byte `gorm:"type:blob(32);not null"`
	KexPublicKey []byte `gorm:"type:blob(32);not null"`
}

type mNodeSyncer struct {
	GUID     []byte `gorm:"primary_key;type:blob(52)"`
	CtrlSend uint64 `gorm:"not null;column:controller_send"`
	NodeRecv uint64 `gorm:"not null;column:node_receive"`
	NodeSend uint64 `gorm:"not null;column:node_send"`
	CtrlRecv uint64 `gorm:"not null;column:controller_receive"`
}

type mBeaconSyncer struct {
	GUID       []byte `gorm:"primary_key;type:blob(52)"`
	CtrlSend   uint64 `gorm:"not null;column:controller_send"`
	BeaconRecv uint64 `gorm:"not null;column:beacon_receive"`
	BeaconSend uint64 `gorm:"not null;column:beacon_send"`
	CtrlRecv   uint64 `gorm:"not null;column:controller_receive"`
}

// mMessage is role <-> role message
// Max Message see internal/protocol/message.go
type mMessage struct {
	ID        uint64 `gorm:"primary_key"`
	RoleGUID  []byte `gorm:"not null;type:blob(52)"`
	Index     uint64 `gorm:"not null"`
	GUID      []byte `gorm:"not null;type:blob(52)"`
	Message   []byte `gorm:"not null;type:blob(2097152)"`
	Hash      []byte `gorm:"not null;type:blob(32)"`
	Signature []byte `gorm:"not null;type:blob(64)"`
}

// for role query
// Node GUID
type mNodeListener struct {
	GUID    []byte    `gorm:"primary_key;type:blob(52)"`
	Mode    xnet.Mode `gorm:"not null"`
	Network string    `gorm:"not null"`
	Address string    `gorm:"not null"`
}

func createDatabase(cfg *Config) error {
	dsnFormat := "file:%s?_auth&_auth_user=%s&_auth_pass=%s&_auth_crypt=sha256"
	dsn := fmt.Sprintf(dsnFormat, cfg.DBFilePath, cfg.DBUsername, cfg.DBPassword)
	db, err := gorm.Open("sqlite3", dsn)
	if err != nil {
		return err
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
			model: &mNode{},
		},
		{
			model: &mNodeSyncer{},
		},
		{
			name:  tableCNMessage,
			model: &mMessage{},
		},
		{
			name:  tableNCMessage,
			model: &mMessage{},
		},
		{
			model: &mBeacon{},
		},
		{
			model: &mBeaconSyncer{},
		},
		{
			name:  tableCBMessage,
			model: &mMessage{},
		},
		{
			name:  tableBCMessage,
			model: &mMessage{},
		},
		{
			model: &mNodeListener{},
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
	err = db.Table(tableCNMessage).Model(&mMessage{}).AddForeignKey("role_guid",
		table+"(guid)", "CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(table, err)
	}
	err = db.Table(tableNCMessage).Model(&mMessage{}).AddForeignKey("role_guid",
		table+"(guid)", "CASCADE", "CASCADE").Error
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
	err = db.Table(tableCBMessage).Model(&mMessage{}).AddForeignKey("role_guid",
		table+"(guid)", "CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(table, err)
	}
	err = db.Table(tableBCMessage).Model(&mMessage{}).AddForeignKey("role_guid",
		table+"(guid)", "CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(table, err)
	}
	return nil

}
