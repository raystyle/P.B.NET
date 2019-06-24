package controller

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"project/internal/logger"
)

type db_logger struct {
	db   string // "mysql"
	file *os.File
}

func new_db_logger(db, path string) (*db_logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 664)
	if err != nil {
		return nil, err
	}
	return &db_logger{db: db, file: f}, nil
}

// [2006-01-02 15:04:05] [WARNING] <mysql> test log
func (this *db_logger) Print(log ...interface{}) {
	_, _ = this.file.Write(print_log(this.db, log...).Bytes())
}

type gorm_logger struct {
	file *os.File
}

func new_gorm_logger(path string) (*gorm_logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 664)
	if err != nil {
		return nil, err
	}
	return &gorm_logger{file: f}, nil
}

func (this *gorm_logger) Print(log ...interface{}) {
	_, _ = this.file.Write(print_log("gorm", log...).Bytes())
}

func print_log(src string, log ...interface{}) *bytes.Buffer {
	b := &bytes.Buffer{}
	b.WriteString("[")
	b.WriteString(time.Now().Local().Format(logger.Time_Layout))
	b.WriteString("] [INFO] <")
	b.WriteString(src)
	b.WriteString("> ")
	b.WriteString(fmt.Sprintln(log...))
	fmt.Print(b.String())
	return b
}
