package logger

import (
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	testSrc  = "test src"
	testLog1 = "test"
	testLog2 = "log"
)

func TestParse(t *testing.T) {
	l, err := Parse("debug")
	require.NoError(t, err)
	require.Equal(t, l, Debug)
	l, err = Parse("info")
	require.NoError(t, err)
	require.Equal(t, l, Info)
	l, err = Parse("warning")
	require.NoError(t, err)
	require.Equal(t, l, Warning)
	l, err = Parse("error")
	require.NoError(t, err)
	require.Equal(t, l, Error)
	l, err = Parse("exploit")
	require.NoError(t, err)
	require.Equal(t, l, Exploit)
	l, err = Parse("fatal")
	require.NoError(t, err)
	require.Equal(t, l, Fatal)
	l, err = Parse("off")
	require.NoError(t, err)
	require.Equal(t, l, Off)
	l, err = Parse("invalid level")
	require.Error(t, err)
	require.Equal(t, l, Debug)
}

func TestPrefix(t *testing.T) {
	for lv := Level(0); lv < Off; lv++ {
		fmt.Println(Prefix(time.Now(), lv, testSrc).String())
	}
	// unknown level
	fmt.Println(Prefix(time.Now(), Level(153), testSrc).String())
}

func TestLogger(t *testing.T) {
	Common.Printf(Debug, testSrc, "test-format %s %s", testLog1, testLog2)
	Common.Print(Debug, testSrc, "test-print", testLog1, testLog2)
	Common.Println(Debug, testSrc, "test-println", testLog1, testLog2)

	Test.Printf(Debug, testSrc, "test-format %s %s", testLog1, testLog2)
	Test.Print(Debug, testSrc, "test-print", testLog1, testLog2)
	Test.Println(Debug, testSrc, "test-println", testLog1, testLog2)

	Discard.Printf(Debug, testSrc, "test-format %s %s", testLog1, testLog2)
	Discard.Print(Debug, testSrc, "test-print", testLog1, testLog2)
	Discard.Println(Debug, testSrc, "test-println", testLog1, testLog2)
}

func TestNewWriterWithPrefix(t *testing.T) {
	w := NewWriterWithPrefix(os.Stdout, "prefix")
	_, err := w.Write([]byte("test\n"))
	require.NoError(t, err)
}

func TestWrap(t *testing.T) {
	l := Wrap(Debug, "test wrap", Test)
	l.Println("Println")
}

func TestHijackLogWriter(t *testing.T) {
	HijackLogWriter(Test)
	log.Println("Println")
}

func TestConn(t *testing.T) {
	conn, err := net.Dial("tcp", "ds.vm3.test-ipv6.com:80")
	require.NoError(t, err)
	fmt.Println(Conn(conn))
	_ = conn.Close()
}
