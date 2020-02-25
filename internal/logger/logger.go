package logger

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

// Level is the log level
type Level = uint8

// about level
const (
	Debug Level = iota
	Info
	Warning
	Error
	Exploit
	Fatal
	Off
)

// TimeLayout is used to provide a parameter to time.Time.Format().
const TimeLayout = "2006-01-02 15:04:05"

// Logger is a common logger.
type Logger interface {
	Printf(lv Level, src, format string, log ...interface{})
	Print(lv Level, src string, log ...interface{})
	Println(lv Level, src string, log ...interface{})
}

// Parse is used to parse logger level from string.
func Parse(level string) (Level, error) {
	lv := Level(0)
	switch level {
	case "debug":
		lv = Debug
	case "info":
		lv = Info
	case "warning":
		lv = Warning
	case "error":
		lv = Error
	case "exploit":
		lv = Exploit
	case "fatal":
		lv = Fatal
	case "off":
		lv = Off
	default:
		return lv, fmt.Errorf("unknown logger level: %s", level)
	}
	return lv, nil
}

// Prefix is used to print time, level and source to a buffer.
//
// time + level + source + log
// source usually like: class name + "-" + instance tag
//
// [2018-11-27 00:00:00] [info] <main> controller is running
// [2018-11-27 00:00:00] [info] <socks5-test> start listener
func Prefix(time time.Time, level Level, src string) *bytes.Buffer {
	var lv string
	switch level {
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
	buf := bytes.Buffer{}
	buf.WriteString("[")
	buf.WriteString(time.Local().Format(TimeLayout))
	buf.WriteString("] [")
	buf.WriteString(lv)
	buf.WriteString("] <")
	buf.WriteString(src)
	buf.WriteString("> ")
	return &buf
}

var (
	// Common is a common logger, some tools need it.
	Common Logger = new(common)

	// Test is used to go test.
	Test Logger = new(test)

	// Discard is used to discard log in object test.
	Discard Logger = new(discard)
)

// [2020-01-21 12:36:41] [debug] <test src> test-format test log
type common struct{}

func (common) Printf(lv Level, src, format string, log ...interface{}) {
	output := Prefix(time.Now(), lv, src)
	_, _ = fmt.Fprintf(output, format, log...)
	fmt.Println(output)
}

func (common) Print(lv Level, src string, log ...interface{}) {
	output := Prefix(time.Now(), lv, src)
	_, _ = fmt.Fprint(output, log...)
	fmt.Println(output)
}

func (common) Println(lv Level, src string, log ...interface{}) {
	output := Prefix(time.Now(), lv, src)
	_, _ = fmt.Fprintln(output, log...)
	fmt.Print(output)
}

// [Test] [2020-01-21 12:36:41] [debug] <test src> test-format test log
type test struct{}

var testPrefix = []byte("[Test] ")

func writePrefix(lv Level, src string) *bytes.Buffer {
	output := new(bytes.Buffer)
	output.Write(testPrefix)
	_, _ = io.Copy(output, Prefix(time.Now(), lv, src))
	return output
}

func (test) Printf(lv Level, src, format string, log ...interface{}) {
	output := writePrefix(lv, src)
	_, _ = fmt.Fprintf(output, format, log...)
	fmt.Println(output)
}

func (test) Print(lv Level, src string, log ...interface{}) {
	output := writePrefix(lv, src)
	_, _ = fmt.Fprint(output, log...)
	fmt.Println(output)
}

func (test) Println(lv Level, src string, log ...interface{}) {
	output := writePrefix(lv, src)
	_, _ = fmt.Fprintln(output, log...)
	fmt.Print(output)
}

type discard struct{}

func (discard) Printf(_ Level, _, _ string, _ ...interface{}) {}

func (discard) Print(_ Level, _ string, _ ...interface{}) {}

func (discard) Println(_ Level, _ string, _ ...interface{}) {}

// pWriter is used to print with a prefix
type pWriter struct {
	w      io.Writer
	prefix []byte
}

func (p pWriter) Write(b []byte) (n int, err error) {
	n = len(b)
	_, err = p.w.Write(append(p.prefix, b...))
	return
}

// NewWriterWithPrefix is used to print prefix before each log.
// it used for role test
func NewWriterWithPrefix(w io.Writer, prefix string) io.Writer {
	return pWriter{
		w:      w,
		prefix: []byte(fmt.Sprintf("[%s] ", prefix)),
	}
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

// Wrap is for go internal logger like http.Server.ErrorLog.
func Wrap(lv Level, src string, logger Logger) *log.Logger {
	w := &writer{
		level:  lv,
		src:    src,
		logger: logger,
	}
	return log.New(w, "", 0)
}

// HijackLogWriter is used to hijack all packages that use log.Print().
func HijackLogWriter(logger Logger) {
	log.SetFlags(log.Llongfile)
	w := &writer{
		level:  Error,
		src:    "pkg-log",
		logger: logger,
	}
	log.SetOutput(w)
}

// Conn is used to print connection information.
// local:  tcp 127.0.0.1:123
// remote: tcp 127.0.0.1:124
func Conn(conn net.Conn) *bytes.Buffer {
	b := bytes.Buffer{}
	_, _ = fmt.Fprintf(&b, "local:  %s %s\nremote: %s %s ",
		conn.LocalAddr().Network(), conn.LocalAddr(),
		conn.RemoteAddr().Network(), conn.RemoteAddr())
	return &b
}

const (
	// post data length in one line
	bodyLineLength = 64

	// <security> prevent too big resp.Body
	maxBodyLength = 1024
)

// HTTPRequest is used to print http.Request.
//
// client: 127.0.0.1:1234
// POST /index HTTP/1.1
// Host: github.com
// Accept: text/html
// Connection: keep-alive
// User-Agent: Mozilla
//
// post data...
// post data...
func HTTPRequest(r *http.Request) *bytes.Buffer {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintf(buf, "client: %s\n", r.RemoteAddr)
	// request
	_, _ = fmt.Fprintf(buf, "%s %s %s", r.Method, r.RequestURI, r.Proto)
	// host
	_, _ = fmt.Fprintf(buf, "\nHost: %s", r.Host)
	// header
	for k, v := range r.Header {
		_, _ = fmt.Fprintf(buf, "\n%s: %s", k, v[0])
	}
	if r.Body != nil {
		rawBody := new(bytes.Buffer)
		defer func() {
			r.Body = ioutil.NopCloser(io.MultiReader(rawBody, r.Body))
		}()
		// start print
		buffer := make([]byte, bodyLineLength)
		// check body
		n, err := io.ReadFull(r.Body, buffer)
		if err != nil {
			if n == 0 { // no body
				return buf
			}
			// 0 < data size < bodyLineLength
			_, _ = fmt.Fprintf(buf, "\n\n%s", buffer[:n])
			rawBody.Write(buffer[:n])
			return buf
		}
		// new line and write data
		_, _ = fmt.Fprintf(buf, "\n\n%s", buffer)
		rawBody.Write(buffer)
		for {
			if rawBody.Len() > maxBodyLength {
				break
			}
			n, err := io.ReadFull(r.Body, buffer)
			if err != nil {
				// write last line
				if n != 0 {
					_, _ = fmt.Fprintf(buf, "\n%s", buffer[:n])
					rawBody.Write(buffer[:n])
				}
				break
			}
			_, _ = fmt.Fprintf(buf, "\n%s", buffer)
			rawBody.Write(buffer)
		}
	}
	return buf
}
