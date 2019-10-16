package node

import (
	"bytes"
	"fmt"
	"os"

	"project/internal/logger"
)

type xLogger struct {
	ctx   *NODE
	level logger.Level
}

func newLogger(ctx *NODE, level string) (*xLogger, error) {
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
	// log with level and src
	logStr := fmt.Sprintf(format, log...)
	buffer.WriteString(logStr)
	buffer.WriteString("\n")
	lg.writeLog(lv, src, logStr, buffer)
}

func (lg *xLogger) Print(lv logger.Level, src string, log ...interface{}) {
	if lv < lg.level {
		return
	}
	buffer := logger.Prefix(lv, src)
	// log with level and src
	logStr := fmt.Sprint(log...)
	buffer.WriteString(logStr)
	buffer.WriteString("\n")
	lg.writeLog(lv, src, logStr, buffer)
}

func (lg *xLogger) Println(lv logger.Level, src string, log ...interface{}) {
	if lv < lg.level {
		return
	}
	buffer := logger.Prefix(lv, src)
	// log with level and src
	logStr := fmt.Sprintln(log...)
	buffer.WriteString(logStr)
	lg.writeLog(lv, src, logStr[:len(logStr)-1], buffer) // delete "\n"
}

// log don't include time level src, for database
func (lg *xLogger) writeLog(lv logger.Level, src, log string, b *bytes.Buffer) {
	// send to controller

	// print to console
	_, _ = b.WriteTo(os.Stdout)
}
