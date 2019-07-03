package controller

import (
	"bytes"
	"fmt"
	"os"

	"github.com/pkg/errors"

	"project/internal/logger"
)

type db_logger struct {
	db   string // "mysql"
	file *os.File
}

func new_db_logger(db, path string) (*db_logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 664)
	if err != nil {
		return nil, errors.Wrapf(err, "create %s logger failed", db)
	}
	return &db_logger{db: db, file: f}, nil
}

// [2006-01-02 15:04:05] [INFO] <mysql> test log
func (this *db_logger) Print(log ...interface{}) {
	buffer := logger.Prefix(logger.INFO, this.db)
	buffer.WriteString(fmt.Sprintln(log...))
	_, _ = this.file.Write(buffer.Bytes())
	fmt.Print(buffer.String())
}

func (this *db_logger) Close() {
	_ = this.file.Close()
}

type gorm_logger struct {
	file *os.File
}

func new_gorm_logger(path string) (*gorm_logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 664)
	if err != nil {
		return nil, errors.Wrap(err, "create gorm logger failed")
	}
	return &gorm_logger{file: f}, nil
}

// [2006-01-02 15:04:05] [INFO] <gorm> test log
func (this *gorm_logger) Print(log ...interface{}) {
	buffer := logger.Prefix(logger.INFO, "gorm")
	buffer.WriteString(fmt.Sprintln(log...))
	_, _ = this.file.Write(buffer.Bytes())
	fmt.Print(buffer.String())
}

func (this *gorm_logger) Close() {
	_ = this.file.Close()
}

func (this *CTRL) Printf(l logger.Level, src string, format string, log ...interface{}) {
	if l < this.log_lv {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	log_str := fmt.Sprintf(format, log...)
	buffer.WriteString(log_str)
	this.print_log(l, src, log_str, buffer)
}

func (this *CTRL) Print(l logger.Level, src string, log ...interface{}) {
	if l < this.log_lv {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	log_str := fmt.Sprint(log...)
	buffer.WriteString(log_str)
	this.print_log(l, src, log_str, buffer)
}

func (this *CTRL) Println(l logger.Level, src string, log ...interface{}) {
	if l < this.log_lv {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	log_str := fmt.Sprintln(log...)
	log_str = log_str[:len(log_str)-1] // delete "\n"
	buffer.WriteString(log_str)
	this.print_log(l, src, log_str, buffer)
}

func (this *CTRL) print_log(l logger.Level, src, log string, b *bytes.Buffer) {
	fmt.Println(b.String())
	m := &m_ctrl_log{
		Level:  l,
		Source: src,
		Log:    log,
	}
	this.db.Create(m)
}
