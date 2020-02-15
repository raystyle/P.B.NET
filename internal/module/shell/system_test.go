package shell

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/axgle/mahonia"
	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestSystem(t *testing.T) {
	mahonia.NewDecoder("GBK")

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	system, err := NewSystem("", nil, "")
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 16)
		for {
			n, err := system.Read(buf)
			if err != nil {
				return
			}
			fmt.Print(string(buf[:n]))
		}
	}()

	time.Sleep(time.Second)

	_, err = system.Write([]byte("cmd\n"))
	require.NoError(t, err)
	_, err = system.Write([]byte("whoami\n"))
	require.NoError(t, err)
	_, err = system.Write([]byte("exit\n"))
	require.NoError(t, err)
	_, err = system.Write([]byte("whoami\n\n"))
	require.NoError(t, err)

	time.Sleep(time.Second)

	err = system.Close()
	require.NoError(t, err)

	wg.Wait()

	// for test output
	fmt.Println()
}
