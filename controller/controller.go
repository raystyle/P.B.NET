package controller

import (
	"github.com/jinzhu/gorm"

	"project/internal/logger"
)

type CTRL struct {
	db        *gorm.DB
	log_level logger.Level
	global    *global
}

func New(c *Config) (*CTRL, error) {
	db, err := connect_database(c)
	if err != nil {
		return nil, err
	}
	// init logger
	l, err := logger.Parse(c.Log_Level)
	if err != nil {
		return nil, err
	}
	ctrl := &CTRL{
		db:        db,
		log_level: l,
	}
	// init global
	g, err := new_global(ctrl, c)
	if err != nil {
		return nil, err
	}
	ctrl.global = g
	return ctrl, nil
}

func (this *CTRL) Main() error {

	return nil
}

func (this *CTRL) Exit() error {
	_ = this.db.Close()
	return nil
}
