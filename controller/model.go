package controller

import (
	"time"
)

type Model struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
}

type m_proxy_client struct {
	ID     uint64 `gorm:"primary_key"`
	Tag    string `gorm:"size:32;not null" sql:"index"`
	Mode   string `gorm:"size:32;not null"`
	Config string `gorm:"size:8192;not null"`
	Model
}

type m_dns_client struct {
	ID      uint64 `gorm:"primary_key"`
	Tag     string `gorm:"size:32;not null" sql:"index"`
	Method  string `gorm:"size:32;not null"`
	Address string `gorm:"size:2048;not null"`
	Model
}

type m_timesync struct {
	ID     uint64 `gorm:"primary_key"`
	Tag    string `gorm:"size:32;not null" sql:"index"`
	Mode   string `gorm:"size:32;not null"`
	Config string `gorm:"size:16000;not null"`
	Model
}

type m_bootstrap struct {
	ID       uint64        `gorm:"primary_key"`
	Tag      string        `gorm:"size:32;not null" sql:"index"`
	Mode     string        `gorm:"size:32;not null"`
	Config   string        `gorm:"size:16000;not null"`
	Interval time.Duration `gorm:"not null"`
	Enable   bool          `gorm:"not null"`
	Model
}

// GUID type: binary(guid.Size)
type m_listener struct {
	ID     uint64 `gorm:"primary_key"`
	GUID   []byte `gorm:"type:binary(52);not null"`
	Tag    string `gorm:"size:32;not null"`
	Mode   string `gorm:"size:32;not null"`
	Config string `gorm:"size:16000;not null"`
	Model
}
