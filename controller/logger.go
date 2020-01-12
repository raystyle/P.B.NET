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
	dialect string // "mysql"
	file    *os.File
	writer  io.Writer
}

func newDatabaseLogger(dialect, path string, writer io.Writer) (*dbLogger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create %s logger", dialect)
	}
	return &dbLogger{
		dialect: dialect,
		file:    file,
		writer:  io.MultiWriter(file, writer),
	}, nil
}

// [2018-11-27 00:00:00] [info] <mysql> test log
func (lg *dbLogger) Print(log ...interface{}) {
	buf := logger.Prefix(logger.Info, lg.dialect)
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = buf.WriteTo(lg.writer)
}

func (lg *dbLogger) Close() {
	_ = lg.file.Close()
}

type gormLogger struct {
	file   *os.File
	writer io.Writer
}

func newGormLogger(path string, writer io.Writer) (*gormLogger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create gorm logger")
	}
	return &gormLogger{
		file:   file,
		writer: io.MultiWriter(file, writer),
	}, nil
}

// [2018-11-27 00:00:00] [info] <gorm> test log
func (lg *gormLogger) Print(log ...interface{}) {
	buf := logger.Prefix(logger.Info, "gorm")
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = buf.WriteTo(lg.writer)
}

func (lg *gormLogger) Close() {
	_ = lg.file.Close()
}

type gLogger struct {
	ctx *CTRL

	level  logger.Level
	file   *os.File
	writer io.Writer
}

func newLogger(ctx *CTRL, config *Config) (*gLogger, error) {
	cfg := config.Logger
	lv, err := logger.Parse(cfg.Level)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	return &gLogger{
		ctx:    ctx,
		level:  lv,
		file:   file,
		writer: io.MultiWriter(file, cfg.Writer),
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
	_ = lg.ctx.database.InsertCtrlLog(&mCtrlLog{
		Level:  lv,
		Source: src,
		Log:    log,
	})
	_, _ = b.WriteTo(lg.writer)
}

func (lg *gLogger) Close() {
	_ = lg.file.Close()
	lg.ctx = nil
}
