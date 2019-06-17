package node

import (
	"bytes"
	"fmt"
	"time"

	log "project/internal/logger"
)

const time_layout = "2006-01-02 15:04:05"

type logger struct {
	ctx   *NODE
	level log.Level
}

func new_logger(ctx *NODE) (*logger, error) {
	l, err := log.Parse(ctx.config.Log_level)
	if err != nil {
		return nil, err
	}
	return &logger{ctx: ctx, level: l}, nil
}

func (this *logger) Printf(level log.Level, src, format string, log ...interface{}) {
	if level < this.level {
		return
	}
	buffer := this.prefix(level, src)
	if buffer == nil {
		return
	}
	buffer.WriteString(fmt.Sprintf(format, log...))
	this.print(buffer.String())
}

func (this *logger) Println(level log.Level, src string, log ...interface{}) {
	if level < this.level {
		return
	}
	buffer := this.prefix(level, src)
	if buffer == nil {
		return
	}
	buffer.WriteString(fmt.Sprint(log...))
	this.print(buffer.String())
}

// time + level + source + log
// source usually like class name + "-" + instance tag
// [2006-01-02 15:04:05] [INFO] <timesync> start http proxy server
func (this *logger) prefix(level log.Level, src string) *bytes.Buffer {
	buffer := &bytes.Buffer{}
	buffer.WriteString("[")
	buffer.WriteString(time.Now().Local().Format(time_layout))
	buffer.WriteString("] [")
	switch level {
	case log.DEBUG:
		buffer.WriteString("DEBUG")
	case log.INFO:
		buffer.WriteString("INFO")
	case log.WARNING:
		buffer.WriteString("WARNING")
	case log.ERROR:
		buffer.WriteString("ERROR")
	case log.EXPLOIT:
		buffer.WriteString("EXPLOIT")
	case log.FATAL:
		buffer.WriteString("FATAL")
	default:
		return nil
	}
	buffer.WriteString("] <")
	buffer.WriteString(src)
	buffer.WriteString("> ")
	return buffer
}

func (this *logger) print(log string) {
	fmt.Println(log)
}
