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

func TestCommandLineToArgv(t *testing.T) {
	exe1 := "test"
	exe2 := `"test test"`
	testdata := []struct {
		cmd  string
		args []string
	}{
		{"net", []string{"net"}},
		{`net -a -b`, []string{"net", "-a", "-b"}},
		{`net -a -b "a"`, []string{"net", "-a", "-b", "a"}},
		{`"net net"`, []string{"net net"}},
		{`"net\net"`, []string{`net\net`}},
		{`"net\net net"`, []string{`net\net net`}},
		{`net -a \"net`, []string{"net", "-a", `"net`}},
		{`net -a ""`, []string{"net", "-a", ""}},
		{`""net""  -a  -b`, []string{"net", "-a", "-b"}},
		{`"""net""" -a`, []string{`"net"`, "-a"}},
	}
	for i := 0; i < len(testdata); i++ {
		args := CommandLineToArgv(exe1 + " " + testdata[i].cmd)
		require.Equal(t, append([]string{"test"}, testdata[i].args...), args)
	}
	for i := 0; i < len(testdata); i++ {
		args := CommandLineToArgv(exe2 + " " + testdata[i].cmd)
		require.Equal(t, append([]string{"test test"}, testdata[i].args...), args)
	}
}

func TestTerminal(t *testing.T) {
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

		"ping 8.8.8.8\n",
		"interrupt",
		"interrupt",

		"whoami\n",
	}
	for _, cmd := range command {
		switch cmd {
		case "interrupt":
			err = system.Interrupt()
			require.NoError(t, err)
		default:
			_, err = system.Write([]byte(cmd))
			require.NoError(t, err)
			time.Sleep(100 * time.Millisecond)
		}
	}

	err = system.Close()
	require.NoError(t, err)

	wg.Wait()

	// for test output
	fmt.Println()
}
