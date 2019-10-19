package logger

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

type Level = uint8

const (
	Debug Level = iota
	Info
	Warning
	Error
	Exploit
	Fatal
	Off
)

const TimeLayout = "2006-01-02 15:04:05"

var (
	Test    = new(test)
	Discard = new(discard)
)

type Logger interface {
	Printf(l Level, src string, format string, log ...interface{})
	Print(l Level, src string, log ...interface{})
	Println(l Level, src string, log ...interface{})
}

// Parse
func Parse(level string) (Level, error) {
	l := Level(0)
	switch level {
	case "debug":
		l = Debug
	case "info":
		l = Info
	case "warning":
		l = Warning
	case "error":
		l = Error
	case "exploit":
		l = Exploit
	case "fatal":
		l = Fatal
	case "off":
		l = Off
	default:
		return l, fmt.Errorf("unknown logger level: %s", level)
	}
	return l, nil
}

// Prefix
// time + level + source + log
// source usually like class name + "-" + instance tag
// [2006-01-02 15:04:05] [info] <http proxy-test> start http proxy server
func Prefix(l Level, src string) *bytes.Buffer {
	lv := ""
	switch l {
	case Debug:
		lv = "debug"
	case Info:
		lv = "info"
	case Warning:
		lv = "warning"
	case Error:
		lv = "error"
	case Exploit:
		lv = "exploit"
	case Fatal:
		lv = "fatal"
	default:
		lv = "unknown"
	}
	buffer := bytes.Buffer{}
	buffer.WriteString("[")
	buffer.WriteString(time.Now().Local().Format(TimeLayout))
	buffer.WriteString("] [")
	buffer.WriteString(lv)
	buffer.WriteString("] <")
	buffer.WriteString(src)
	buffer.WriteString("> ")
	return &buffer
}

// Wrap is for go internal logger like http.Server.ErrorLog
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

func (w *writer) Write(p []byte) (int, error) {
	w.logger.Println(w.level, w.src, string(p[:len(p)-1]))
	return len(p), nil
}

type test struct{}

func (t *test) Printf(l Level, src string, format string, log ...interface{}) {
	b := Prefix(l, src)
	_, _ = fmt.Fprintf(b, format, log...)
	fmt.Println(b.String())
}

func (t *test) Print(l Level, src string, log ...interface{}) {
	b := Prefix(l, src)
	_, _ = fmt.Fprint(b, log...)
	fmt.Println(b.String())
}

func (t *test) Println(l Level, src string, log ...interface{}) {
	b := Prefix(l, src)
	_, _ = fmt.Fprintln(b, log...)
	fmt.Print(b.String())
}

type discard struct{}

func (d *discard) Printf(l Level, src string, format string, log ...interface{}) {}

func (d *discard) Print(l Level, src string, log ...interface{}) {}

func (d *discard) Println(l Level, src string, log ...interface{}) {}

// Conn is used to print conn info
// tcp 127.0.0.1:123 <-> tcp 127.0.0.1:124
func Conn(conn net.Conn) *bytes.Buffer {
	b := bytes.Buffer{}
	_, _ = fmt.Fprintf(&b, "conn: %s %s <-> %s %s ",
		conn.LocalAddr().Network(), conn.LocalAddr(),
		conn.RemoteAddr().Network(), conn.RemoteAddr())
	return &b
}

const postLineLength = 50

// HTTPRequest is used to print http.Request
//
// client: 127.0.0.1:1234
//
// POST /index HTTP/1.1
// Accept: text/html
// Connection: keep-alive
// User-Agent: Mozilla
//
// post data...
// post data...
func HTTPRequest(r *http.Request) string {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintf(b, "client: %s\n\n", r.RemoteAddr)
	_, _ = fmt.Fprintf(b, "%s %s %s", r.Method, r.RequestURI, r.Proto)
	for k, v := range r.Header {
		_, _ = fmt.Fprintf(b, "\n%s: %s", k, v[0])
	}
	if r.Body != nil {
		buffer := make([]byte, postLineLength)
		// check body
		n, err := io.ReadFull(r.Body, buffer)
		if err != nil {
			if n == 0 { // no body
				return b.String()
			}
			// 0 < data size < postLineLength
			_, _ = fmt.Fprintf(b, "\n\n%s", buffer[:n])
			return b.String()
		}
		// new line and write data
		_, _ = fmt.Fprintf(b, "\n\n%s", buffer)
		for {
			n, err := io.ReadFull(r.Body, buffer)
			if err != nil {
				// write last line
				if n != 0 {
					_, _ = fmt.Fprintf(b, "\n%s", buffer[:n])
				}
				break
			}
			_, _ = fmt.Fprintf(b, "\n%s", buffer)
		}
	}
	return b.String()
}
