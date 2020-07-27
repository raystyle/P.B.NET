package filemgr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirToZipFile(t *testing.T) {
	t.Skip()

	err := DirToZipFile("../filemgr/", "testdata/filemgr.zip")
	require.NoError(t, err)

	err = DirToZipFile("../filemgr/*", "testdata/filemgr.zip")
	require.NoError(t, err)
}

func TestZipFileToDir(t *testing.T) {
	t.Skip()

	const dir = "testdata/zip-file"
	err := ZipFileToDir("testdata/file.zip", dir)
	require.NoError(t, err)

	// err = os.RemoveAll(dir)
	// require.NoError(t, err)
}
