package controller

import (
	"bytes"
	"fmt"
	"io"
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
		return nil, errors.Wrapf(err, "failed to create %s logger", db)
	}
	return &dbLogger{db: db, file: f}, nil
}

// [2006-01-02 15:04:05] [info] <mysql> test log
func (l *dbLogger) Print(log ...interface{}) {
	buf := logger.Prefix(logger.Info, l.db)
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = l.file.Write(buf.Bytes())
	_, _ = buf.WriteTo(os.Stdout)
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
		return nil, errors.Wrap(err, "failed to create gorm logger")
	}
	return &gormLogger{file: f}, nil
}

// [2006-01-02 15:04:05] [info] <gorm> test log
func (l *gormLogger) Print(log ...interface{}) {
	buf := logger.Prefix(logger.Info, "gorm")
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = l.file.Write(buf.Bytes())
	_, _ = buf.WriteTo(os.Stdout)
}

func (l *gormLogger) Close() {
	_ = l.file.Close()
}

type gLogger struct {
	ctx *CTRL

	level  logger.Level
	writer io.Writer
}

func newLogger(ctx *CTRL, config *Config) (*gLogger, error) {
	cfg := config.Logger
	lv, err := logger.Parse(cfg.Level)
	if err != nil {
		return nil, err
	}
	return &gLogger{
		ctx:    ctx,
		level:  lv,
		writer: cfg.Writer,
	}, nil
}

func (lg *gLogger) Printf(lv logger.Level, src, format string, log ...interface{}) {
	if lv < lg.level {
		return
	}
	buf := logger.Prefix(lv, src)
	// log with level and src
	logStr := fmt.Sprintf(format, log...)
	buf.WriteString(logStr)
	buf.WriteString("\n")
	lg.writeLog(lv, src, logStr, buf)
}

func (lg *gLogger) Print(lv logger.Level, src string, log ...interface{}) {
	if lv < lg.level {
		return
	}
	buf := logger.Prefix(lv, src)
	// log with level and src
	logStr := fmt.Sprint(log...)
	buf.WriteString(logStr)
	buf.WriteString("\n")
	lg.writeLog(lv, src, logStr, buf)
}

func (lg *gLogger) Println(lv logger.Level, src string, log ...interface{}) {
	if lv < lg.level {
		return
	}
	buf := logger.Prefix(lv, src)
	// log with level and src
	logStr := fmt.Sprintln(log...)
	buf.WriteString(logStr)
	lg.writeLog(lv, src, logStr[:len(logStr)-1], buf) // delete "\n"
}

// string log don't include time level src, for database
func (lg *gLogger) writeLog(lv logger.Level, src, log string, b *bytes.Buffer) {
	_ = lg.ctx.db.InsertCtrlLog(&mCtrlLog{
		Level:  lv,
		Source: src,
		Log:    log,
	})
	_, _ = b.WriteTo(lg.writer)
}

func (lg *gLogger) Close() {
	lg.ctx = nil
}

// asdadsadadasd
