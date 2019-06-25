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

// [2006-01-02 15:04:05] [INFO] <mysql> test log
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

// [2006-01-02 15:04:05] [INFO] <gorm> test log
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

type ctrl_logger struct {
	db *database
	l  logger.Level
}

func new_ctrl_logger(ctx *CONTROLLER) (*ctrl_logger, error) {
	l, err := logger.Parse(ctx.config.Log_Level)
	if err != nil {
		return nil, err
	}
	return &ctrl_logger{db: ctx.database, l: l}, nil
}

func (this *ctrl_logger) Printf(l logger.Level, src string, format string, log ...interface{}) {
	if l < this.l {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	log_str := fmt.Sprintf(format, log...)
	buffer.WriteString(log_str)
	fmt.Println(buffer.String())
	_ = this.db.Insert_Ctrl_Log(l, src, log_str)
}

func (this *ctrl_logger) Print(l logger.Level, src string, log ...interface{}) {
	if l < this.l {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	log_str := fmt.Sprint(log...)
	buffer.WriteString(log_str)
	fmt.Println(buffer.String())
	_ = this.db.Insert_Ctrl_Log(l, src, log_str)
}

func (this *ctrl_logger) Println(l logger.Level, src string, log ...interface{}) {
	if l < this.l {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	log_str := fmt.Sprintln(log...)
	log_str = log_str[:len(log_str)-1] // delete "\n"
	buffer.WriteString(log_str)
	fmt.Println(buffer.String())
	_ = this.db.Insert_Ctrl_Log(l, src, log_str)
}
