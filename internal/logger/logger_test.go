package logger

import (
	"testing"
)

func Test_Logger(t *testing.T) {
	Test.Printf(DEBUG, "test src", "test format %s", "str")
	Test.Print(DEBUG, "test src", "test print log")
	Test.Println(DEBUG, "test src", "test println log")
	Discard.Printf(DEBUG, "test src", "test format %s", "str")
	Discard.Print(DEBUG, "test src", "test print log")
	Discard.Println(DEBUG, "test src", "test println log")
}
