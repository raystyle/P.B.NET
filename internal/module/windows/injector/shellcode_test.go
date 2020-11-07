// +build windows

package injector

import (
	"encoding/hex"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
	
	"github.com/stretchr/testify/require"
	
	"project/internal/testsuite"
)

func TestInjectShellcode(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var (
		file *os.File
		err  error
	)
	switch runtime.GOARCH {
	case "386":
		file, err = os.Open("../../shellcode/testdata/windows_32.txt")
		require.NoError(t, err)
	case "amd64":
		file, err = os.Open("../../shellcode/testdata/windows_64.txt")
		require.NoError(t, err)
	default:
		t.Skip("unsupported architecture:", runtime.GOARCH)
	}

	t.Logf("use %s shellcode\n", runtime.GOARCH)
	defer func() { _ = file.Close() }()
	shellcode, err := ioutil.ReadAll(hex.NewDecoder(file))
	require.NoError(t, err)

	cmd := exec.Command("notepad.exe")
	err = cmd.Start()
	require.NoError(t, err)

	pid := uint32(cmd.Process.Pid)
	t.Log("notepad.exe process id:", pid)

	err = InjectShellcode(pid, shellcode)
	require.NoError(t, err)

	time.Sleep(time.Minute)

	err = cmd.Wait()
	require.NoError(t, err)
}
