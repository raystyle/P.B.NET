package shellcode

import (
	"encoding/hex"
	"io/ioutil"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExecute(t *testing.T) {
	switch runtime.GOOS {
	case "windows":
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
		cp := make([]byte, len(shellcode))

		// must copy, because shellcode will be clean
		copy(cp, shellcode)
		require.NoError(t, Execute("vp", cp))

		copy(cp, shellcode)
		require.NoError(t, Execute("thread", cp))

		err = Execute("foo method", shellcode)
		require.EqualError(t, err, "unknown method: foo method")
	case "linux":
		return
	default:
		return
	}
}
