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

func TestSplitShellcode(t *testing.T) {
	shellcode := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9}
	splitSize := len(shellcode) / 2

	t.Log(splitSize)

	// secondStage first copy for hide special header
	firstStage := shellcode[:splitSize]
	secondStage := shellcode[splitSize:]

	// first size must one byte for pass some AV
	nextSize := 1
	l := len(secondStage)
	for i := 0; i < l; {
		if i+nextSize > l {
			nextSize = l - i
		}

		t.Log("bytes:", secondStage[i:i+nextSize])
		t.Log("address:", splitSize+i)

		i += nextSize
		nextSize = 4 // set random
	}

	nextSize = 1
	l = len(firstStage)
	for i := 0; i < l; {
		if i+nextSize > l {
			nextSize = l - i
		}

		t.Log("bytes:", firstStage[i:i+nextSize])
		t.Log("address:", i)

		i += nextSize
		nextSize = 4 // random
	}

	// b [5]
	// addr 4
	// b [6 7 8 9]
	// addr 5
	// b [1]
	// addr 0
	// b [2 3 4]
	// addr 1
}

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

	t.Run("wait and clean", func(t *testing.T) {
		cp := make([]byte, len(shellcode))
		copy(cp, shellcode)

		err = InjectShellcode(pid, cp, 0, true, true)
		require.NoError(t, err)
	})

	t.Run("wait", func(t *testing.T) {
		cp := make([]byte, len(shellcode))
		copy(cp, shellcode)

		err = InjectShellcode(pid, cp, 8, true, false)
		require.NoError(t, err)
	})

	t.Run("not wait", func(t *testing.T) {
		cp := make([]byte, len(shellcode))
		copy(cp, shellcode)

		err = InjectShellcode(pid, cp, 16, false, false)
		require.NoError(t, err)

		time.Sleep(3 * time.Second)
	})

	err = cmd.Process.Kill()
	require.NoError(t, err)

	// exit status 1
	err = cmd.Wait()
	require.Error(t, err)
}
