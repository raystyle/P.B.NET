package controller

import (
	"time"
)

// different table has the same model
const (
	t_ctrl_log   = "controller_log"
	t_node_log   = "node_log"
	t_beacon_log = "beacon_log"
)

type Model struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
}

type m_ctrl_log struct {
	ID        uint64 `gorm:"primary_key"`
	CreatedAt time.Time
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

type m_bootstrap struct {
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

// internal/guid/guid.go  guid.Size
// beacon & node log
// size:104 = hex(guid) = guid.Size * 2
type m_role_log struct {
	ID        uint64 `gorm:"primary_key"`
	CreatedAt time.Time
	GUID      string     `gorm:"size:104;not null"`
	Level     uint8      `gorm:"not null" sql:"index"`
	Source    string     `gorm:"size:32;not null" sql:"index"`
	Log       string     `gorm:"size:16000;not null"`
	DeletedAt *time.Time `sql:"index"`
}
