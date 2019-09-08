package controller

import (
	"time"

	"project/internal/config"
	"project/internal/xnet"
)

// different table has the same model
const (
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
	AESKey      []byte     `gorm:"type:binary(48);not null"`
	IsBootstrap bool       `gorm:"not null"`
	CreatedAt   time.Time  `gorm:"not null"`
	DeletedAt   *time.Time `sql:"index"`
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

type mNodeSyncer struct {
	GUID      []byte    `gorm:"primary_key;type:binary(52)"`
	CtrlSend  uint64    `gorm:"not null;column:controller_send"`
	NodeRecv  uint64    `gorm:"not null;column:node_receive"`
	NodeSend  uint64    `gorm:"not null;column:node_send"`
	CtrlRecv  uint64    `gorm:"not null;column:controller_receive"`
	UpdatedAt time.Time `gorm:"not null"`
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
