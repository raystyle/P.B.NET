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

// [2006-01-02 15:04:05] [info] <mysql> test log
func (l *dbLogger) Print(log ...interface{}) {
	buffer := logger.Prefix(logger.Info, l.db)
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

// [2006-01-02 15:04:05] [info] <gorm> test log
func (l *gormLogger) Print(log ...interface{}) {
	buffer := logger.Prefix(logger.Info, "gorm")
	_, _ = fmt.Fprintln(buffer, log...)
	_, _ = l.file.Write(buffer.Bytes())
	_, _ = buffer.WriteTo(os.Stdout)
}

func (l *gormLogger) Close() {
	_ = l.file.Close()
}

type xLogger struct {
	ctx   *CTRL
	level logger.Level
}

func newLogger(ctx *CTRL, level string) (*xLogger, error) {
	// init logger
	lv, err := logger.Parse(level)
	if err != nil {
		return nil, err
	}
	return &xLogger{
		ctx:   ctx,
		level: lv,
	}, nil
}

func (lg *xLogger) Printf(lv logger.Level, src string, format string, log ...interface{}) {
	if lv < lg.level {
		return
	}
	buffer := logger.Prefix(lv, src)
	_, _ = fmt.Fprintf(buffer, format, log...)
	lg.printLog(lv, src, buffer)
}

func (lg *xLogger) Print(lv logger.Level, src string, log ...interface{}) {
	if lv < lg.level {
		return
	}
	buffer := logger.Prefix(lv, src)
	_, _ = fmt.Fprint(buffer, log...)
	lg.printLog(lv, src, buffer)
}

func (lg *xLogger) Println(lv logger.Level, src string, log ...interface{}) {
	if lv < lg.level {
		return
	}
	buffer := logger.Prefix(lv, src)
	_, _ = fmt.Fprintln(buffer, log...)
	buffer.Truncate(buffer.Len() - 1) // delete "\n"
	lg.printLog(lv, src, buffer)
}

// log don't include time level src, for database
func (lg *xLogger) printLog(lv logger.Level, src string, b *bytes.Buffer) {
	// write to database
	m := mCtrlLog{
		Level:  lv,
		Source: src,
		Log:    b.String(),
	}
	_ = lg.ctx.db.InsertCtrlLog(&m)
	// print console
	b.WriteString("\n")
	_, _ = b.WriteTo(os.Stdout)
}
