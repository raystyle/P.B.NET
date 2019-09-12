package controller

import (
	"bytes"
	"fmt"
	"os"

	"github.com/pkg/errors"

	"project/internal/logger"
)

type dbLogger struct {
	db   string // "mysql"
	file *os.File
}

func newDBLogger(db, path string) (*dbLogger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 644)
	if err != nil {
		return nil, errors.Wrapf(err, "create %s logger failed", db)
	}
	return &dbLogger{db: db, file: f}, nil
}

// [2006-01-02 15:04:05] [INFO] <mysql> test log
func (l *dbLogger) Print(log ...interface{}) {
	buffer := logger.Prefix(logger.INFO, l.db)
	_, _ = fmt.Fprintln(buffer, log...)
	_, _ = l.file.Write(buffer.Bytes())
	_, _ = buffer.WriteTo(os.Stdout)
}

func (l *dbLogger) Close() {
	_ = l.file.Close()
}

type gormLogger struct {
	file *os.File
}

func newGormLogger(path string) (*gormLogger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 644)
	if err != nil {
		return nil, errors.Wrap(err, "create gorm logger failed")
	}
	return &gormLogger{file: f}, nil
}

// [2006-01-02 15:04:05] [INFO] <gorm> test log
func (l *gormLogger) Print(log ...interface{}) {
	buffer := logger.Prefix(logger.INFO, "gorm")
	_, _ = fmt.Fprintln(buffer, log...)
	_, _ = l.file.Write(buffer.Bytes())
	_, _ = buffer.WriteTo(os.Stdout)
}

func (l *gormLogger) Close() {
	_ = l.file.Close()
}

func (ctrl *CTRL) Printf(l logger.Level, src string, format string, log ...interface{}) {
	if l < ctrl.logLv {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	logStr := fmt.Sprintf(format, log...)
	buffer.WriteString(logStr)
	ctrl.printLog(l, src, logStr, buffer)
}

func (ctrl *CTRL) Print(l logger.Level, src string, log ...interface{}) {
	if l < ctrl.logLv {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	logStr := fmt.Sprint(log...)
	buffer.WriteString(logStr)
	ctrl.printLog(l, src, logStr, buffer)
}

func (ctrl *CTRL) Println(l logger.Level, src string, log ...interface{}) {
	if l < ctrl.logLv {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	logStr := fmt.Sprintln(log...)
	logStr = logStr[:len(logStr)-1] // delete "\n"
	buffer.WriteString(logStr)
	ctrl.printLog(l, src, logStr, buffer)
}

// log don't include time level src, for database
func (ctrl *CTRL) printLog(l logger.Level, src, log string, b *bytes.Buffer) {
	// write to database
	m := &mCtrlLog{
		Level:  l,
		Source: src,
		Log:    log,
	}
	_ = ctrl.db.InsertCtrlLog(m)
	// print console
	b.WriteString("\n")
	_, _ = b.WriteTo(os.Stdout)
}
