package logger

import (
	"testing"
)

func Test_Logger(t *testing.T) {
	Test.Printf(DEBUG, "test src", "test format %s", "test log")
	Test.Print(DEBUG, "test src", "test print", "test log")
	Test.Println(DEBUG, "test src", "test println", "test log")
	Discard.Printf(DEBUG, "test src", "test format %s", "test log")
	Discard.Print(DEBUG, "test src", "test print", "test log")
	Discard.Println(DEBUG, "test src", "test println", "test log")
}
