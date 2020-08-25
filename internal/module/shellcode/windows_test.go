// +build windows

package shellcode

import (
	"encoding/hex"
	"io/ioutil"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func TestVirtualProtect(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var (
		file *os.File
		err  error
	)
	switch runtime.GOARCH {
	case "386":
		file, err = os.Open("testdata/windows_32.txt")
		require.NoError(t, err)
	case "amd64":
		file, err = os.Open("testdata/windows_64.txt")
		require.NoError(t, err)
	default:
		t.Skip("unsupported architecture:", runtime.GOARCH)
	}

	t.Logf("use %s shellcode\n", runtime.GOARCH)
	defer func() { _ = file.Close() }()
	shellcode, err := ioutil.ReadAll(hex.NewDecoder(file))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		cp := make([]byte, len(shellcode))
		copy(cp, shellcode)
		err = VirtualProtect(cp)
		require.NoError(t, err)
	}

	// empty data
	err = VirtualProtect(nil)
	require.EqualError(t, err, "empty data")
}

func TestCreateThread(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var (
		file *os.File
		err  error
	)
	switch runtime.GOARCH {
	case "386":
		file, err = os.Open("testdata/windows_32.txt")
		require.NoError(t, err)
	case "amd64":
		file, err = os.Open("testdata/windows_64.txt")
		require.NoError(t, err)
	default:
		t.Skip("unsupported architecture:", runtime.GOARCH)
	}

	t.Logf("use %s shellcode\n", runtime.GOARCH)
	defer func() { _ = file.Close() }()
	shellcode, err := ioutil.ReadAll(hex.NewDecoder(file))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		cp := make([]byte, len(shellcode))
		copy(cp, shellcode)
		err = CreateThread(cp)
		require.NoError(t, err)
	}

	// empty data
	err = CreateThread(nil)
	require.EqualError(t, err, "empty data")
}

func TestExecute(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	var (
		file *os.File
		err  error
	)
	switch runtime.GOARCH {
	case "386":
		file, err = os.Open("testdata/windows_32.txt")
		require.NoError(t, err)
	case "amd64":
		file, err = os.Open("testdata/windows_64.txt")
		require.NoError(t, err)
	default:
		t.Skip("unsupported architecture:", runtime.GOARCH)
	}

	t.Logf("use %s shellcode\n", runtime.GOARCH)
	defer func() { _ = file.Close() }()
	shellcode, err := ioutil.ReadAll(hex.NewDecoder(file))
	require.NoError(t, err)
	cp := make([]byte, len(shellcode))

	// must copy, because shellcode will be clean
	copy(cp, shellcode)
	err = Execute(MethodVirtualProtect, cp)
	require.NoError(t, err)

	copy(cp, shellcode)
	err = Execute(MethodCreateThread, cp)
	require.NoError(t, err)

	err = Execute("foo method", shellcode)
	require.EqualError(t, err, "unknown method: foo method")
}
