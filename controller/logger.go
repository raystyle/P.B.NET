package controller

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/xpanic"
)

type dbLogger struct {
	ctx *Ctrl

	dialect string // "mysql"
	file    *os.File
	writer  io.Writer

	rwm sync.RWMutex
}

func newDatabaseLogger(ctx *Ctrl, dialect, path string, writer io.Writer) (*dbLogger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0600) // #nosec
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create %s logger", dialect)
	}
	return &dbLogger{
		ctx:     ctx,
		dialect: dialect,
		file:    file,
		writer:  io.MultiWriter(file, writer),
	}, nil
}

// [2018-11-27 00:00:00] [info] <mysql> test log
func (lg *dbLogger) Print(log ...interface{}) {
	lg.rwm.RLock()
	defer lg.rwm.RUnlock()
	if lg.ctx == nil {
		return
	}
	buf := logger.Prefix(lg.ctx.global.Now(), logger.Info, lg.dialect)
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = buf.WriteTo(lg.writer)
}

func (lg *dbLogger) Close() {
	_ = lg.file.Close()
	lg.rwm.Lock()
	defer lg.rwm.Unlock()
	lg.ctx = nil
}

type gormLogger struct {
	ctx *Ctrl

	file   *os.File
	writer io.Writer

	rwm sync.RWMutex
}

func newGormLogger(ctx *Ctrl, path string, writer io.Writer) (*gormLogger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND, 0600) // #nosec
	if err != nil {
		return nil, errors.Wrap(err, "failed to create gorm logger")
	}
	return &gormLogger{
		ctx:    ctx,
		file:   file,
		writer: io.MultiWriter(file, writer),
	}, nil
}

// [2018-11-27 00:00:00] [info] <gorm> test log
func (lg *gormLogger) Print(log ...interface{}) {
	lg.rwm.RLock()
	defer lg.rwm.RUnlock()
	if lg.ctx == nil {
		return
	}
	buf := logger.Prefix(lg.ctx.global.Now(), logger.Info, "gorm")
	_, _ = fmt.Fprintln(buf, log...)
	_, _ = buf.WriteTo(lg.writer)
}

func (lg *gormLogger) Close() {
	_ = lg.file.Close()
	lg.rwm.Lock()
	defer lg.rwm.Unlock()
	lg.ctx = nil
}

type gLogger struct {
	ctx *Ctrl

	level  logger.Level
	file   *os.File
	writer io.Writer

	rwm sync.RWMutex
}

func newLogger(ctx *Ctrl, config *Config) (*gLogger, error) {
	cfg := config.Logger
	lv, err := logger.Parse(cfg.Level)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_APPEND, 0600) // #nosec
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
	lg.rwm.RLock()
	defer lg.rwm.RUnlock()
	if lv < lg.level || lg.ctx == nil {
		return
	}
	buf := logger.Prefix(lg.ctx.global.Now().Local(), lv, src)
	// log with level and src
	logStr := fmt.Sprintf(format, log...)
	buf.WriteString(logStr)
	buf.WriteString("\n")
	lg.writeLog(lv, src, logStr, buf)
}

func (lg *gLogger) Print(lv logger.Level, src string, log ...interface{}) {
	lg.rwm.RLock()
	defer lg.rwm.RUnlock()
	if lv < lg.level || lg.ctx == nil {
		return
	}
	buf := logger.Prefix(lg.ctx.global.Now().Local(), lv, src)
	// log with level and src
	logStr := fmt.Sprint(log...)
	buf.WriteString(logStr)
	buf.WriteString("\n")
	lg.writeLog(lv, src, logStr, buf)
}

func (lg *gLogger) Println(lv logger.Level, src string, log ...interface{}) {
	lg.rwm.RLock()
	defer lg.rwm.RUnlock()
	if lv < lg.level || lg.ctx == nil {
		return
	}
	buf := logger.Prefix(lg.ctx.global.Now().Local(), lv, src)
	// log with level and src
	logStr := fmt.Sprintln(log...)
	buf.WriteString(logStr)
	lg.writeLog(lv, src, logStr[:len(logStr)-1], buf) // delete "\n"
}

// SetLevel is used to set log level that need print.
func (lg *gLogger) SetLevel(lv logger.Level) error {
	if lv > logger.Off {
		return errors.Errorf("invalid logger level %d", lv)
	}
	lg.rwm.Lock()
	defer lg.rwm.Unlock()
	lg.level = lv
	return nil
}

func (lg *gLogger) Close() {
	_ = lg.file.Close()
	lg.rwm.Lock()
	defer lg.rwm.Unlock()
	lg.ctx = nil
}

// string log don't include time, level and source, it will also save to the database.
func (lg *gLogger) writeLog(lv logger.Level, src, log string, b *bytes.Buffer) {
	defer func() {
		if r := recover(); r != nil {
			_, _ = xpanic.Print(r, "gLogger.writeLog").WriteTo(lg.writer)
		}
	}()
	_, _ = b.WriteTo(lg.writer)
	err := lg.ctx.database.InsertLog(&mLog{
		Level:  lv,
		Source: src,
		Log:    []byte(log),
	})
	if err != nil {
		buf := logger.Prefix(lg.ctx.global.Now().Local(), logger.Error, "logger")
		_, _ = buf.WriteTo(lg.writer)
		_, _ = fmt.Fprintln(lg.writer, err)
	}
}
