package node

import (
	"bytes"
	"fmt"
	"time"

	"project/internal/logger"
)

const time_layout = "2006-01-02 15:04:05"

type log struct {
	ctx   *NODE
	level logger.Level
}

func new_log(ctx *NODE) *log {
	return &log{
		ctx:   ctx,
		level: ctx.config.Log_level,
	}
}

func (this *log) Printf(level logger.Level, src, format string, log ...interface{}) {
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

func (this *log) Println(level logger.Level, src string, log ...interface{}) {
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
func (this *log) prefix(level logger.Level, src string) *bytes.Buffer {
	buffer := &bytes.Buffer{}
	buffer.WriteString("[")
	buffer.WriteString(time.Now().Local().Format(time_layout))
	buffer.WriteString("] [")
	switch level {
	case logger.DEBUG:
		buffer.WriteString("DEBUG")
	case logger.INFO:
		buffer.WriteString("INFO")
	case logger.WARNING:
		buffer.WriteString("WARNING")
	case logger.ERROR:
		buffer.WriteString("ERROR")
	case logger.EXPLOIT:
		buffer.WriteString("EXPLOIT")
	case logger.FATAL:
		buffer.WriteString("FATAL")
	default:
		return nil
	}
	buffer.WriteString("] <")
	buffer.WriteString(src)
	buffer.WriteString("> ")
	return buffer
}

func (this *log) print(log string) {
	fmt.Println(log)
}
