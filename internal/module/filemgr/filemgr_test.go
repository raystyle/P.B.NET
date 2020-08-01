package filemgr

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/module/task"
	"project/internal/patch/monkey"
	"project/internal/system"
	"project/internal/testsuite"
)

func testCreateFile(t *testing.T, name string) {
	data := testsuite.Bytes()
	err := system.WriteFile(name, data)
	require.NoError(t, err)
}

func testCreateFile2(t *testing.T, name string) {
	data := bytes.Repeat(testsuite.Bytes(), 2)
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

func testIsExist(t *testing.T, path string) {
	exist, err := system.IsExist(path)
	require.NoError(t, err)
	require.True(t, exist)
}

func testIsNotExist(t *testing.T, path string) {
	notExist, err := system.IsNotExist(path)
	require.NoError(t, err)
	require.True(t, notExist)
}

func testPatchTaskCanceled() *monkey.PatchGuard {
	t := new(task.Task)
	var pg *monkey.PatchGuard
	patch := func(task *task.Task) bool {
		time.Sleep(200 * time.Millisecond)
		pg.Unpatch()
		defer pg.Restore()
		return task.Canceled()
	}
	pg = monkey.PatchInstanceMethod(t, "Canceled", patch)
	return pg
}

func testPatchMultiTaskWatcher() *monkey.PatchGuard {
	patch := func(duration time.Duration) *time.Ticker {
		panic(monkey.Panic)
	}
	return monkey.Patch(time.NewTicker, patch)
}

func TestIsRoot(t *testing.T) {
	for _, path := range [...]string{
		"/", "\\", "C:\\", "\\\\host\\share",
	} {
		require.True(t, isRoot(path))
	}
	require.False(t, isRoot("C:\\test.dat"))
}

const mockTaskName = "mock task"

type mockTask struct{}

func (mockTask) Prepare(context.Context) error {
	return nil
}

func (mockTask) Process(context.Context, *task.Task) error {
	return nil
}

func (mockTask) Progress() string {
	return "99%"
}

func (mockTask) Detail() string {
	return ""
}

func (mockTask) Clean() {
}

type notEqualWriter struct{}

func (notEqualWriter) Write([]byte) (int, error) {
	return 0, nil
}

func TestIOCopy(t *testing.T) {
	testdata := bytes.Repeat([]byte("hello"), 100)
	add := func(int64) {}

	t.Run("common", func(t *testing.T) {
		mt := task.New(mockTaskName, nil, nil)
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)

		readBuf.Write(testdata)

		n, err := ioCopy(mt, add, writeBuf, readBuf)
		require.NoError(t, err)
		require.Equal(t, int64(len(testdata)), n)

		require.Equal(t, testdata, writeBuf.Bytes())
	})

	t.Run("cancel", func(t *testing.T) {
		mt := task.New(mockTaskName, new(mockTask), nil)
		mt.Cancel()
		readBuf := new(bytes.Buffer)
		writeBuf := new(bytes.Buffer)

		readBuf.Write(testdata)

		n, err := ioCopy(mt, add, writeBuf, readBuf)
		require.Equal(t, context.Canceled, err)
		require.Zero(t, n)
	})

	t.Run("failed to read", func(t *testing.T) {
		mt := task.New(mockTaskName, nil, nil)
		reader := testsuite.NewMockConnWithReadError()
		writer := new(bytes.Buffer)

		n, err := ioCopy(mt, add, writer, reader)
		require.Error(t, err)
		require.Equal(t, int64(0), n)
	})

	t.Run("failed to write", func(t *testing.T) {
		mt := task.New(mockTaskName, nil, nil)
		reader := new(bytes.Buffer)
		reader.Write([]byte{1, 2, 3})
		writer := testsuite.NewMockConnWithWriteError()

		n, err := ioCopy(mt, add, writer, reader)
		require.Error(t, err)
		require.Equal(t, int64(0), n)
	})

	t.Run("written not equal", func(t *testing.T) {
		mt := task.New(mockTaskName, nil, nil)
		reader := new(bytes.Buffer)
		reader.Write([]byte{1, 2, 3})
		writer := new(notEqualWriter)

		n, err := ioCopy(mt, add, writer, reader)
		require.Error(t, err)
		require.Equal(t, int64(0), n)
	})
}

func TestStartTask(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("cancel before start", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := startTask(ctx, nil, mockTaskName)
		require.Equal(t, context.Canceled, err)
	})

	t.Run("unexpected progress", func(t *testing.T) {
		mt := task.New(mockTaskName, new(mockTask), nil)

		err := startTask(context.Background(), mt, mockTaskName)
		require.EqualError(t, err, "unexpected progress: 99%")
	})

	t.Run("panic in created goroutine", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		mt := task.New(mockTaskName, new(mockTask), nil)

		tt := new(task.Task)
		patch1 := func(interface{}) bool {
			panic(monkey.Panic)
		}
		pg1 := monkey.PatchInstanceMethod(tt, "Cancel", patch1)
		defer pg1.Unpatch()

		patch2 := func(interface{}) error {
			cancel()
			time.Sleep(time.Second) // wait goroutine in startTask
			return nil
		}
		pg2 := monkey.PatchInstanceMethod(tt, "Start", patch2)
		defer pg2.Unpatch()

		err := startTask(ctx, mt, mockTaskName)
		require.Error(t, err)
	})
}
