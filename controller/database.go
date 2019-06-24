package controller

import (
	"github.com/jinzhu/gorm"

	_ "github.com/go-sql-driver/mysql"
)

type database struct {
	ctx *CONTROLLER
	db  *gorm.DB
}

func new_database(ctx *CONTROLLER) (*database, error) {
	d := &database{
		ctx: ctx,
	}
	return d, nil
}

func (this *database) Connect() error {
	db, err := gorm.Open("mysql", "test.db")
	if err != nil {
		return err
	}

	return nil
}
