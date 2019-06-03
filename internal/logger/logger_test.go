package logger

import "testing"

func Test_Println(t *testing.T) {
	Test.Println(DEBUG, "test src", "test log")
}
