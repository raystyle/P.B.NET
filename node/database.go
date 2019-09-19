package node

import (
	_ "github.com/mattn/go-sqlite3"

	_ "project/internal/gorm"
)

type db struct {
}

func newDB(ctx *NODE, cfg *Config) (*db, error) {

	return new(db), nil
}

func (db *db) SelectNodeMessage(guid []byte, index uint64) (*mMessage, error) {

	return nil, nil
}
