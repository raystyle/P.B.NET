package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	test_src = "test src"
	test_log = "test log"
)

func Test_Parse(t *testing.T) {
	l, err := Parse("debug")
	require.Nil(t, err, err)
	require.Equal(t, l, DEBUG)
	l, err = Parse("info")
	require.Nil(t, err, err)
	require.Equal(t, l, INFO)
	l, err = Parse("warning")
	require.Nil(t, err, err)
	require.Equal(t, l, WARNING)
	l, err = Parse("error")
	require.Nil(t, err, err)
	require.Equal(t, l, ERROR)
	l, err = Parse("exploit")
	require.Nil(t, err, err)
	require.Equal(t, l, EXPLOIT)
	l, err = Parse("fatal")
	require.Nil(t, err, err)
	require.Equal(t, l, FATAL)
	l, err = Parse("off")
	require.Nil(t, err, err)
	require.Equal(t, l, OFF)
	l, err = Parse("invalid level")
	require.NotNil(t, err, err)
	require.Equal(t, l, DEBUG)
}

func Test_Prefix(t *testing.T) {
	for l := Level(0); l < OFF; l++ {
		t.Log(Prefix(l, test_src).String())
	}
}

func Test_Wrap(t *testing.T) {
	l := Wrap(DEBUG, "test wrap", Test)
	l.Println("println")
}

func Test_Logger(t *testing.T) {
	Test.Printf(DEBUG, test_src, "test format %s", test_log)
	Test.Print(DEBUG, test_src, "test print", test_log)
	Test.Println(DEBUG, test_src, "test println", test_log)
	Discard.Printf(DEBUG, test_src, "test format %s", test_log)
	Discard.Print(DEBUG, test_src, "test print", test_log)
	Discard.Println(DEBUG, test_src, "test println", test_log)
}
