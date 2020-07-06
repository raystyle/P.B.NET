package filemgr

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopy(t *testing.T) {
	s, err := os.Stat("copy.go")
	require.NoError(t, err)

	fmt.Println(s.Mode().Perm())

	fmt.Println(filepath.Abs("../../../internal/testsuite"))
	fmt.Println(filepath.Abs("../../../internal/testsuite/"))

	fmt.Println(filepath.Clean("\\mock.go"))
	fmt.Println(filepath.Clean("/mock.go"))

	err = filepath.Walk("E:\\OneDrive\\P.B.NET\\internal\\testsuite", func(path string, info os.FileInfo, err error) error {
		fmt.Println(filepath.Clean(path), info.IsDir(), info.Name(), err)

		return err
	})
	fmt.Println("walk err", err)
}
