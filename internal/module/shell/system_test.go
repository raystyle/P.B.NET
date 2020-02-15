package shell

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/axgle/mahonia"
	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestSystem(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	system, err := NewSystem("", nil, "")
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := mahonia.NewDecoder("GBK").NewReader(system)
		buf := make([]byte, 512)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				return
			}
			_, err = os.Stdout.Write(buf[:n])
			require.NoError(t, err)
		}
	}()

	// wait print welcome information
	time.Sleep(1 * time.Second)

	command := []string{
		"abc\n",
		"wmic\n",
		"asd\n",
		"quit\n",
		"whoami\n",

		"print.exe\n",
		"abc\n",
		"def\n",
		"interrupt",

		"print.exe\n",
		"abc\n",
		"def\n",
		"interrupt",

		"cmd\n",
		"print.exe\n",
		"abc\n",
		"def\n",
		"interrupt",
		"whoami\n",
		"exit\n",

		"print.exe\n",
		"abc\n",
		"def\n",
		"interrupt",

		"ping",
		"interrupt",
		"interrupt",

		"whoami\n",
	}
	for _, cmd := range command {
		switch cmd {
		case "interrupt":
			err = system.Interrupt()
			require.NoError(t, err)
			time.Sleep(1 * time.Second)
		case "ping":
			_, err = system.Write([]byte("ping 8.8.8.8 -t\n"))
			require.NoError(t, err)
			time.Sleep(4 * time.Second)
		default:
			_, err = system.Write([]byte(cmd))
			require.NoError(t, err)
			time.Sleep(250 * time.Millisecond)
		}
	}

	err = system.Close()
	require.NoError(t, err)

	wg.Wait()

	// for test output
	fmt.Println()
}
