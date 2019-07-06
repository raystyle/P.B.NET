package node

import (
	"bytes"
	"fmt"
	"os"

	"project/internal/logger"
)

func (this *NODE) Printf(l logger.Level, src, format string, log ...interface{}) {
	if l < this.log_lv {
		return
	}
	b := logger.Prefix(l, src)
	if b == nil {
		return
	}
	_, _ = fmt.Fprintf(b, format, log...)
	this.print_log(b)
}

func (this *NODE) Print(l logger.Level, src string, log ...interface{}) {
	if l < this.log_lv {
		return
	}
	b := logger.Prefix(l, src)
	if b == nil {
		return
	}
	_, _ = fmt.Fprint(b, log...)
	this.print_log(b)
}

func (this *NODE) Println(l logger.Level, src string, log ...interface{}) {
	if l < this.log_lv {
		return
	}
	b := logger.Prefix(l, src)
	if b == nil {
		return
	}
	_, _ = fmt.Fprintln(b, log...)
	b.Truncate(b.Len() - 1) // delete "\n"
	this.print_log(b)
}

func (this *NODE) print_log(b *bytes.Buffer) {
	// send to controller

	// print console
	b.WriteString("\n")
	_, _ = b.WriteTo(os.Stdout)
}
