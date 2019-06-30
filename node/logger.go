package node

import (
	"bytes"
	"fmt"

	"project/internal/logger"
)

func (this *NODE) Printf(l logger.Level, src, format string, log ...interface{}) {
	if l < this.log_level {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	buffer.WriteString(fmt.Sprintf(format, log...))
	this.print_log(buffer)
}

func (this *NODE) Print(l logger.Level, src string, log ...interface{}) {
	if l < this.log_level {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	buffer.WriteString(fmt.Sprint(log...))
	this.print_log(buffer)
}

func (this *NODE) Println(l logger.Level, src string, log ...interface{}) {
	if l < this.log_level {
		return
	}
	buffer := logger.Prefix(l, src)
	if buffer == nil {
		return
	}
	log_str := fmt.Sprintln(log...)
	log_str = log_str[:len(log_str)-1] // delete "\n"
	buffer.WriteString(log_str)
	this.print_log(buffer)
}

func (this *NODE) Fatalln(log ...interface{}) {
	if logger.FATAL < this.log_level {
		return
	}
	buffer := logger.Prefix(logger.FATAL, "init")
	if buffer == nil {
		return
	}
	log_str := fmt.Sprintln(log...)
	log_str = log_str[:len(log_str)-1] // delete "\n"
	buffer.WriteString(log_str)
	this.print_log(buffer)
	this.Exit()
}

func (this *NODE) print_log(b *bytes.Buffer) {
	fmt.Println(b.String())
}
