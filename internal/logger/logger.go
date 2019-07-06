package logger

import (
	"bytes"
	"fmt"
	"log"
	"time"
)

type Level = uint8

const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	EXPLOIT
	FATAL
	OFF
)

const Time_Layout = "2006-01-02 15:04:05"

var (
	Test    = new(test)
	Discard = new(discard)
)

type Logger interface {
	Printf(l Level, src string, format string, log ...interface{})
	Print(l Level, src string, log ...interface{})
	Println(l Level, src string, log ...interface{})
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

// time + level + source + log
// source usually like class name + "-" + instance tag
// [2006-01-02 15:04:05] [INFO] <http proxy-test> start http proxy server
func Prefix(l Level, src string) *bytes.Buffer {
	lv := ""
	switch l {
	case DEBUG:
		lv = "DEBUG"
	case INFO:
		lv = "INFO"
	case WARNING:
		lv = "WARNING"
	case ERROR:
		lv = "ERROR"
	case EXPLOIT:
		lv = "EXPLOIT"
	case FATAL:
		lv = "FATAL"
	}
	buffer := &bytes.Buffer{}
	buffer.WriteString("[")
	buffer.WriteString(time.Now().Local().Format(Time_Layout))
	buffer.WriteString("] [")
	buffer.WriteString(lv)
	buffer.WriteString("] <")
	buffer.WriteString(src)
	buffer.WriteString("> ")
	return buffer
}

// for go internal logger like http.Server.ErrorLog
func Wrap(l Level, src string, logger Logger) *log.Logger {
	w := &writer{
		level:  l,
		src:    src,
		logger: logger,
	}
	return log.New(w, "", 0)
}

type writer struct {
	level  Level
	src    string
	logger Logger
}

func (this *writer) Write(p []byte) (int, error) {
	this.logger.Println(this.level, this.src, string(p[:len(p)-1]))
	return len(p), nil
}

type test struct{}

func (this *test) Printf(l Level, src string, format string, log ...interface{}) {
	b := Prefix(l, src)
	_, _ = fmt.Fprintf(b, format, log...)
	fmt.Println(b.String())
}

func (this *test) Print(l Level, src string, log ...interface{}) {
	b := Prefix(l, src)
	_, _ = fmt.Fprint(b, log...)
	fmt.Println(b.String())
}

func (this *test) Println(l Level, src string, log ...interface{}) {
	b := Prefix(l, src)
	_, _ = fmt.Fprintln(b, log...)
	fmt.Print(b.String())
}

type discard struct{}

func (this *discard) Printf(l Level, src string, format string, log ...interface{}) {}

func (this *discard) Print(l Level, src string, log ...interface{}) {}

func (this *discard) Println(l Level, src string, log ...interface{}) {}
