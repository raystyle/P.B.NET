package logger

import (
	"bytes"
	"fmt"
	"log"
	"time"
)

type Level uint8

const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	EXPLOIT
	FATAL
	OFF
)

const time_layout = "2006-01-02 15:04:05"

var (
	Test = &test{}
)

type Logger interface {
	Printf(level Level, src string, format string, log ...interface{})
	Println(level Level, src string, log ...interface{})
}

// for go internal logger like http.Server.ErrorLog
func Wrap(level Level, src string, logger Logger) *log.Logger {
	w := &writer{
		level:  level,
		src:    src,
		logger: logger,
	}
	return log.New(w, "", 0)
}

func Parse(level string) (Level, error) {
	l := Level(0)
	switch level {
	case "debug":
		l = DEBUG
	case "info":
		l = INFO
	case "warning":
		l = WARNING
	case "error":
		l = ERROR
	case "exploit":
		l = EXPLOIT
	case "fatal":
		l = FATAL
	case "off":
		l = OFF
	default:
		return l, fmt.Errorf("invalid level: %s", level)
	}
	return l, nil
}

type writer struct {
	level  Level
	src    string
	logger Logger
}

func (this *writer) Write(p []byte) (int, error) {
	this.logger.Println(this.level, this.src, string(p))
	return len(p), nil
}

type test struct{}

func (this *test) Printf(level Level, src string, format string, log ...interface{}) {
	buffer := this.prefix(level, src)
	if buffer == nil {
		return
	}
	buffer.WriteString(fmt.Sprintf(format, log...))
	fmt.Println(buffer.String())
}

func (this *test) Println(level Level, src string, log ...interface{}) {
	buffer := this.prefix(level, src)
	if buffer == nil {
		return
	}
	buffer.WriteString(fmt.Sprint(log...))
	fmt.Println(buffer.String())
}

// time + level + source + log
// source usually like class name + "-" + instance tag
// [2006-01-02 15:04:05] [INFO] <http proxy-test> start http proxy server
func (this *test) prefix(level Level, src string) *bytes.Buffer {
	buffer := &bytes.Buffer{}
	buffer.WriteString("[")
	buffer.WriteString(time.Now().Local().Format(time_layout))
	buffer.WriteString("] [")
	switch level {
	case DEBUG:
		buffer.WriteString("DEBUG")
	case INFO:
		buffer.WriteString("INFO")
	case WARNING:
		buffer.WriteString("WARNING")
	case ERROR:
		buffer.WriteString("ERROR")
	case EXPLOIT:
		buffer.WriteString("EXPLOIT")
	case FATAL:
		buffer.WriteString("FATAL")
	default:
		return nil
	}
	buffer.WriteString("] <")
	buffer.WriteString(src)
	buffer.WriteString("> ")
	return buffer
}
