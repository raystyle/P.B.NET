package filemgr

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/module/task"
	"project/internal/system"
	"project/internal/testsuite"
)

func testCreateFile(t *testing.T, name string) {
	data := testsuite.Bytes()
	err := system.WriteFile(name, data)
	require.NoError(t, err)
}

func testCreateFile2(t *testing.T, name string) {
	data := append(testsuite.Bytes(), testsuite.Bytes()...)
	err := system.WriteFile(name, data)
	require.NoError(t, err)
}

func testCompareFile(t *testing.T, a, b string) {
	aFile, err := os.Open(a)
	require.NoError(t, err)
	defer func() { _ = aFile.Close() }()
	bFile, err := os.Open(b)
	require.NoError(t, err)
	defer func() { _ = bFile.Close() }()

	// compare stat
	aStat, err := aFile.Stat()
	require.NoError(t, err)
	bStat, err := bFile.Stat()
	require.NoError(t, err)

	require.Equal(t, aStat.Size(), bStat.Size())
	require.Equal(t, aStat.Mode(), bStat.Mode())
	require.Equal(t, aStat.IsDir(), bStat.IsDir())

	if !aStat.IsDir() {
		// compare data
		aFileData, err := ioutil.ReadAll(aFile)
		require.NoError(t, err)
		bFileData, err := ioutil.ReadAll(bFile)
		require.NoError(t, err)
		require.Equal(t, aFileData, bFileData)

		// mod time is not equal about wall
		// directory stat may be changed
		const format = "2006-01-02 15:04:05"
		am := aStat.ModTime().Format(format)
		bm := bStat.ModTime().Format(format)
		require.Equal(t, am, bm)
	}
}

func testCompareDirectory(t *testing.T, a, b string) {
	aFiles := make([]string, 0, 4)
	bFiles := make([]string, 0, 4)
	err := filepath.Walk(a, func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)
		if path != a {
			aFiles = append(aFiles, path)
		}
		return nil
	})
	require.NoError(t, err)
	err = filepath.Walk(b, func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)
		if path != b {
			bFiles = append(bFiles, path)
		}
		return nil
	})
	require.NoError(t, err)

	// compare file numbers
	aFilesLen := len(aFiles)
	bFilesLen := len(bFiles)
	require.Equal(t, aFilesLen, bFilesLen)

	// compare each file
	for i := 0; i < aFilesLen; i++ {
		testCompareFile(t, aFiles[i], bFiles[i])
	}
}

type notEqualWriter struct{}

func (notEqualWriter) Write([]byte) (int, error) {
	return 0, nil
}

func TestIOCopy(t *testing.T) {
	testdata := bytes.Repeat([]byte("hello"), 100)
	add := func(int64) {}

	t.Run("common", func(t *testing.T) {
		fakeTask := task.New("fake task", nil, nil)
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)

		readBuf.Write(testdata)

		n, err := ioCopy(fakeTask, add, writeBuf, readBuf)
		require.NoError(t, err)
		require.Equal(t, int64(len(testdata)), n)

		require.Equal(t, testdata, writeBuf.Bytes())
	})
}
