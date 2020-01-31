package node

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"project/internal/logger"
	"project/internal/xpanic"
)

type gLogger struct {
	ctx *Node

	level  logger.Level
	writer io.Writer

	m sync.Mutex
}

func newLogger(ctx *Node, config *Config) (*gLogger, error) {
	cfg := config.Logger
	lv, err := logger.Parse(cfg.Level)
	if err != nil {
		return nil, err
	}
	return &gLogger{ctx: ctx, level: lv, writer: cfg.Writer}, nil
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

// Close is used to set logger.ctx = nil
func (lg *gLogger) Close() {
	lg.m.Lock()
	defer lg.m.Unlock()
	lg.ctx = nil
}

// string log not include time level src
func (lg *gLogger) writeLog(lv logger.Level, src, log string, b *bytes.Buffer) {
	defer func() {
		if r := recover(); r != nil {
			_, _ = xpanic.Print(r, "gLogger.writeLog").WriteTo(lg.writer)
		}
	}()
	lg.m.Lock()
	defer lg.m.Unlock()
	if lg.ctx == nil {
		return
	}
	// send to controller

	// print to console
	_, _ = b.WriteTo(lg.writer)
}
