package controller

import (
	"time"
)

type Base_Model struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
}

type proxy_client struct {
	ID     uint64 `gorm:"primary_key"`
	Tag    string `gorm:"size:32" sql:"index"`
	Mode   string `gorm:"size:32"`
	Config string `gorm:"size:8192"`
	Base_Model
}
