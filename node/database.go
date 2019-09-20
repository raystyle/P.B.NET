package node

import (
	"fmt"
	"os"

	"github.com/jinzhu/gorm"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"

	_ "project/internal/gorm"
	"project/internal/random"
)

type db struct {
	ctx  *NODE
	db   *gorm.DB
	path string
}

func newDB(ctx *NODE, cfg *Config) (*db, error) {
	// connect database
	path := cfg.DBFilePath
	if path == "" {
		path = os.TempDir() + "/" + random.String(8)
	}
	dsn := fmt.Sprintf("file:%s", path)
	gormDB, err := gorm.Open("sqlite3", dsn)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	err = gormDB.DB().Ping()
	if err != nil {
		return nil, errors.Wrap(err, "ping sqlite3 failed")
	}
	// connection
	gormDB.DB().SetMaxOpenConns(1)
	gormDB.DB().SetMaxIdleConns(1)
	// not add s
	gormDB.SingularTable(true)
	// close log
	gormDB.LogMode(false)
	// check is new database
	if !gormDB.HasTable(new(mNode)) {
		err = initDatabase(gormDB)
		if err != nil {
			return nil, err
		}
	}
	db := db{
		ctx:  ctx,
		db:   gormDB,
		path: path,
	}
	return &db, nil
}

func (db *db) SelectNodeMessage(guid []byte, index uint64) (*mMessage, error) {

	return nil, nil
}
