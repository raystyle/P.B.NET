package node

import (
	"bytes"
	"fmt"
	"os"

	"project/internal/logger"
)

func (node *NODE) Printf(l logger.Level, src, format string, log ...interface{}) {
	if l < node.logLv {
		return
	}
	b := logger.Prefix(l, src)
	if b == nil {
		return
	}
	_, _ = fmt.Fprintf(b, format, log...)
	node.printLog(b)
}

func (node *NODE) Print(l logger.Level, src string, log ...interface{}) {
	if l < node.logLv {
		return
	}
	b := logger.Prefix(l, src)
	if b == nil {
		return
	}
	_, _ = fmt.Fprint(b, log...)
	node.printLog(b)
}

func (node *NODE) Println(l logger.Level, src string, log ...interface{}) {
	if l < node.logLv {
		return
	}
	b := logger.Prefix(l, src)
	if b == nil {
		return
	}
	_, _ = fmt.Fprintln(b, log...)
	b.Truncate(b.Len() - 1) // delete "\n"
	node.printLog(b)
}

func (node *NODE) printLog(b *bytes.Buffer) {
	// send to controller

	// print console
	b.WriteString("\n")
	_, _ = b.WriteTo(os.Stdout)
}
