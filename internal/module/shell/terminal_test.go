package shell

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestTrimPrefixSpace(t *testing.T) {
	testdata := []struct {
		input  string
		except string
	}{
		{input: "a  ", except: "a  "},
		{input: "  a", except: "a"},
		{input: "   ", except: ""},
		{input: "  a ", except: "a "},
	}
	for i := 0; i < len(testdata); i++ {
		require.Equal(t, testdata[i].except, trimPrefixSpace(testdata[i].input))
	}
}

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

	terminal, err := NewTerminal()
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 512)
		for {
			n, err := terminal.Read(buf)
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
		"\n",
		"  \n",

		// about change directory
		"  cd\n",
		"  cd ..\n",
		"cd shell\n",
		`cd "doesn't exist"` + "\n",
	}
	for _, cmd := range command {
		switch cmd {
		case "interrupt":
			err = terminal.Interrupt()
			require.NoError(t, err)
			time.Sleep(1 * time.Second)
		case "ping":
			_, err = terminal.Write([]byte("ping 8.8.8.8 -t\n"))
			require.NoError(t, err)
			time.Sleep(4 * time.Second)
		default:
			_, err = terminal.Write([]byte(cmd))
			require.NoError(t, err)
			time.Sleep(250 * time.Millisecond)
		}
	}

	err = terminal.Close()
	require.NoError(t, err)

	wg.Wait()

	// for test output
	fmt.Println()
}
