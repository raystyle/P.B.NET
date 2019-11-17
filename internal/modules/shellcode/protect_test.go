// +build windows

package shellcode

import (
	"encoding/hex"
	"io/ioutil"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVirtualProtect(t *testing.T) {
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
		return
	}

	t.Logf("use %s shellcode\n", runtime.GOARCH)
	defer func() { _ = file.Close() }()
	shellcode, err := ioutil.ReadAll(hex.NewDecoder(file))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		cp := make([]byte, len(shellcode))
		copy(cp, shellcode)
		require.NoError(t, VirtualProtect(cp))
	}

	// no data
	err = VirtualProtect(nil)
	require.EqualError(t, err, "no data")
}
