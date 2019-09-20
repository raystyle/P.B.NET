package node

import (
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/xnet"
	"project/internal/xreflect"
)

// different table have the same model
const (
	tableMessageCN = "message_cn" // CN = Controller -> Node
	tableMessageNC = "message_nc" // NC = Node -> Controller
	tableMessageCB = "message_cb" // CB = Controller -> Beacon
	tableMessageBC = "message_bc" // BC = Beacon -> Controller
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
	RoleGUID  []byte `gorm:"not null;type:blob(52)" sql:"index"`
	Index     uint64 `gorm:"not null" sql:"index"`
	GUID      []byte `gorm:"not null;type:blob(52)"`
	Message   []byte `gorm:"not null;type:blob(2097152)"`
	Hash      []byte `gorm:"not null;type:blob(32)"`
	Signature []byte `gorm:"not null;type:blob(64)"`
}

// for role query
// Node GUID
// don't save to the database
type mNodeListener struct {
	GUID    []byte
	Mode    xnet.Mode
	Network string
	Address string
}

func initDatabase(db *gorm.DB) error {
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
			name:  tableMessageCN,
			model: &mMessage{},
		},
		{
			name:  tableMessageNC,
			model: &mMessage{},
		},
		{
			model: &mBeacon{},
		},
		{
			model: &mBeaconSyncer{},
		},
		{
			name:  tableMessageCB,
			model: &mMessage{},
		},
		{
			name:  tableMessageBC,
			model: &mMessage{},
		},
	}
	// create tables
	for i := 0; i < len(tables); i++ {
		n := tables[i].name
		m := tables[i].model
		if n == "" {
			err := db.CreateTable(m).Error
			if err != nil {
				table := gorm.ToTableName(xreflect.StructName(m))
				return errors.Wrapf(err, "create table %s failed", table)
			}
		} else {
			err := db.Table(n).CreateTable(m).Error
			if err != nil {
				return errors.Wrapf(err, "create table %s failed", n)
			}
		}
	}
	return nil
}
