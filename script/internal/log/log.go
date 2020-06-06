package log

import (
	"project/internal/logger"
)

var source string

// SetSource is used to set logger source, must call it first.
func SetSource(src string) {
	source = src
}

// Printf is used to print log with format.
func Printf(lv logger.Level, format string, log ...interface{}) {
	logger.Common.Printf(lv, source, format, log...)
}

// Println is used to print log with new line.
func Println(lv logger.Level, log ...interface{}) {
	logger.Common.Println(lv, source, log...)
}
