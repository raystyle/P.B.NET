package logger

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testSrc = "test src"
	testLog = "test log"
)

func TestLogger(t *testing.T) {
	Test.Printf(DEBUG, testSrc, "test format %s", testLog)
	Test.Print(DEBUG, testSrc, "test print", testLog)
	Test.Println(DEBUG, testSrc, "test println", testLog)
	Discard.Printf(DEBUG, testSrc, "test format %s", testLog)
	Discard.Print(DEBUG, testSrc, "test print", testLog)
	Discard.Println(DEBUG, testSrc, "test println", testLog)
}

func TestParse(t *testing.T) {
	l, err := Parse("debug")
	require.NoError(t, err)
	require.Equal(t, l, DEBUG)
	l, err = Parse("info")
	require.NoError(t, err)
	require.Equal(t, l, INFO)
	l, err = Parse("warning")
	require.NoError(t, err)
	require.Equal(t, l, WARNING)
	l, err = Parse("error")
	require.NoError(t, err)
	require.Equal(t, l, ERROR)
	l, err = Parse("exploit")
	require.NoError(t, err)
	require.Equal(t, l, EXPLOIT)
	l, err = Parse("fatal")
	require.NoError(t, err)
	require.Equal(t, l, FATAL)
	l, err = Parse("off")
	require.NoError(t, err)
	require.Equal(t, l, OFF)
	l, err = Parse("invalid level")
	require.Error(t, err)
	require.Equal(t, l, DEBUG)
}

func TestPrefix(t *testing.T) {
	for l := Level(0); l < OFF; l++ {
		t.Log(Prefix(l, testSrc).String())
	}
}

func TestWrap(t *testing.T) {
	l := Wrap(DEBUG, "test wrap", Test)
	l.Println("println")
}
