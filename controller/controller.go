package controller

import (
	"os"

	"github.com/jinzhu/gorm"

	"project/internal/logger"
	"project/internal/protocol"
)

const (
	Name    = "P.B.NET"
	version = protocol.V1_0_0
)

type CTRL struct {
	db        *gorm.DB
	log_level logger.Level
	global    *global
}

func New(c *Config) (*CTRL, error) {
	// debug
	if c.bin_path != "" {
		err := os.Chdir(c.bin_path)
		if err != nil {
			return nil, err
		}
	}
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
	err := this.global.Start_Timesync()
	if err != nil {
		return err
	}
	this.Printf(logger.INFO, src_init, "timesync: %s", this.global.Now())
	this.Print(logger.INFO, src_init, "controller is running")
	return nil
}

func (this *CTRL) Exit() {
	this.Print(logger.INFO, src_init, "controller is stopped")
	_ = this.db.Close()
}
