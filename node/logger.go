package node

import (
	"bytes"
	"fmt"
	"os"

	"project/internal/logger"
)

type gLogger struct {
	ctx   *NODE
	level logger.Level
}

func newLogger(ctx *NODE, level string) (*gLogger, error) {
	lv, err := logger.Parse(level)
	if err != nil {
		return nil, err
	}
	return &gLogger{ctx: ctx, level: lv}, nil
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

// log don't include time level src, for database
func (lg *gLogger) writeLog(lv logger.Level, src, log string, b *bytes.Buffer) {
	// send to controller

	// print to console
	_, _ = b.WriteTo(os.Stdout)
}
