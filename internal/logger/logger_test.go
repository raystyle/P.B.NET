package logger

import (
	"fmt"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

const (
	testPrefixF  = "test format %s %s"
	testPrefix   = "test print"
	testPrefixLn = "test println"
	testSrc      = "test src"
	testLog1     = "test"
	testLog2     = "log"
)

func TestParse(t *testing.T) {
	for _, testdata := range []struct {
		name  string
		level Level
	}{
		{"debug", Debug},
		{"info", Info},
		{"warning", Warning},
		{"error", Error},
		{"exploit", Exploit},
		{"fatal", Fatal},
		{"off", Off},
	} {
		t.Run(testdata.name, func(t *testing.T) {
			l, err := Parse(testdata.name)
			require.NoError(t, err)
			require.Equal(t, l, testdata.level)
		})
	}

	t.Run("invalid level", func(t *testing.T) {
		l, err := Parse("invalid level")
		require.Error(t, err)
		require.Equal(t, l, Debug)
	})
}

func TestPrefix(t *testing.T) {
	for lv := Level(0); lv < Off; lv++ {
		fmt.Println(Prefix(time.Now(), lv, testSrc).String())
	}
	// unknown level
	fmt.Println(Prefix(time.Now(), Level(153), testSrc).String())
}

func TestLogger(t *testing.T) {
	Common.Printf(Debug, testSrc, testPrefixF, testLog1, testLog2)
	Common.Print(Debug, testSrc, testPrefix, testLog1, testLog2)
	Common.Println(Debug, testSrc, testPrefixLn, testLog1, testLog2)

	Test.Printf(Debug, testSrc, testPrefixF, testLog1, testLog2)
	Test.Print(Debug, testSrc, testPrefix, testLog1, testLog2)
	Test.Println(Debug, testSrc, testPrefixLn, testLog1, testLog2)

	Discard.Printf(Debug, testSrc, testPrefixF, testLog1, testLog2)
	Discard.Print(Debug, testSrc, testPrefix, testLog1, testLog2)
	Discard.Println(Debug, testSrc, testPrefixLn, testLog1, testLog2)
}

func TestMultiLogger(t *testing.T) {
	logger := NewMultiLogger(Debug, os.Stdout)

	logger.Printf(Debug, testSrc, testPrefixF, testLog1, testLog2)
	logger.Print(Debug, testSrc, testPrefix, testLog1, testLog2)
	logger.Println(Debug, testSrc, testPrefixLn, testLog1, testLog2)

	t.Run("low level", func(t *testing.T) {
		err := logger.SetLevel(Info)
		require.NoError(t, err)

		logger.Printf(Debug, testSrc, testPrefixF, testLog1, testLog2)
		logger.Print(Debug, testSrc, testPrefix, testLog1, testLog2)
		logger.Println(Debug, testSrc, testPrefixLn, testLog1, testLog2)
	})

	t.Run("invalid level", func(t *testing.T) {
		err := logger.SetLevel(Level(123))
		require.EqualError(t, err, "invalid logger level: 123")
	})

	err := logger.Close()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, logger)
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
	HijackLogWriter(Error, "test", Test, log.Llongfile)
	log.Println("Println")
}

func TestConn(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		listener, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)

		conn, err := net.Dial("tcp", listener.Addr().String())
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()
		fmt.Println(Conn(conn))

		err = listener.Close()
		require.NoError(t, err)
	})

	t.Run("mock", func(t *testing.T) {
		conn := testsuite.NewMockConnWithReadError()
		fmt.Println(Conn(conn))
		_ = conn.Close()
	})
}
