package controller

import (
	"time"

	"project/internal/xnet"
)

// different table has the same model
const (
	t_node_log   = "node_log"
	t_beacon_log = "beacon_log"
)

type Model struct {
	CreatedAt time.Time  `gorm:"not null"`
	UpdatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type m_ctrl_log struct {
	ID        uint64     `gorm:"primary_key"`
	CreatedAt time.Time  `gorm:"not null"`
	Level     uint8      `gorm:"not null" sql:"index"`
	Source    string     `gorm:"size:32;not null" sql:"index"`
	Log       string     `gorm:"size:16000;not null"`
	DeletedAt *time.Time `sql:"index"`
}

type m_proxy_client struct {
	ID     uint64 `gorm:"primary_key"`
	Tag    string `gorm:"size:32;not null;unique"`
	Mode   string `gorm:"size:32;not null"`
	Config string `gorm:"size:16000;not null"`
	Model
}

type m_dns_client struct {
	ID      uint64 `gorm:"primary_key"`
	Tag     string `gorm:"size:32;not null;unique"`
	Method  string `gorm:"size:32;not null"`
	Address string `gorm:"size:2048;not null"`
	Model
}

type m_timesync struct {
	ID     uint64 `gorm:"primary_key"`
	Tag    string `gorm:"size:32;not null;unique"`
	Mode   string `gorm:"size:32;not null"`
	Config string `gorm:"size:16000;not null"`
	Model
}

type m_boot struct {
	ID       uint64 `gorm:"primary_key"`
	Tag      string `gorm:"size:32;not null;unique"`
	Mode     string `gorm:"size:32;not null"`
	Config   string `gorm:"size:16000;not null"`
	Interval uint32 `gorm:"not null"`
	Enable   bool   `gorm:"not null"`
	Model
}

type m_listener struct {
	ID     uint64 `gorm:"primary_key"`
	Tag    string `gorm:"size:32;not null;unique"`
	Mode   string `gorm:"size:32;not null"`
	Config string `gorm:"size:16000;not null"`
	Model
}

type m_node struct {
	GUID      []byte     `gorm:"primary_key;type:binary(52)"`
	AES_Key   []byte     `gorm:"type:binary(48);not null"`
	Publickey []byte     `gorm:"type:binary(32);not null"`
	Bootstrap bool       `gorm:"not null"`
	CreatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type m_node_listener struct {
	ID        uint64     `gorm:"primary_key"`
	GUID      []byte     `gorm:"type:binary(52);not null" sql:"index"`
	Tag       string     `gorm:"size:32;not null"`
	Mode      xnet.Mode  `gorm:"size:32;not null"`
	Network   string     `gorm:"size:32;not null"`
	Address   string     `gorm:"size:2048;not null"`
	CreatedAt time.Time  `gorm:"not null"`
	DeletedAt *time.Time `sql:"index"`
}

type m_node_syncer struct {
	GUID      []byte    `gorm:"primary_key;type:binary(52)"`
	CTRL_Send uint64    `gorm:"not null;column:controller_send"`
	Node_Recv uint64    `gorm:"not null;column:node_receive"`
	Node_Send uint64    `gorm:"not null;column:node_send"`
	CTRL_Recv uint64    `gorm:"not null;column:controller_receive"`
	UpdatedAt time.Time `gorm:"not null"`
}

// internal/guid/guid.go  guid.Size
// beacon & node log
type m_role_log struct {
	ID        uint64     `gorm:"primary_key"`
	CreatedAt time.Time  `gorm:"not null"`
	GUID      []byte     `gorm:"type:binary(52);not null" sql:"index"`
	Level     uint8      `gorm:"not null"`
	Source    string     `gorm:"size:32;not null"`
	Log       string     `gorm:"size:16000;not null"`
	DeletedAt *time.Time `sql:"index"`
}

type m_trust_node struct {
	Mode    xnet.Mode `json:"mode"`
	Network string    `json:"network"`
	Address string    `json:"address"`
}
