package filemgr

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

const (
	testZipDir     = "testdata/zip/"             // zip test root path
	testZipDst     = testZipDir + "zip_test.zip" // destination zip file path
	testZipSrcFile = testZipDir + "file1.dat"    // path is a file
	testZipSrcDir  = testZipDir + "dir"          // path is a directory

	// files in the test directory
	testZipSrcFile1 = testZipSrcDir + "/afile1.dat"  // testdata/zip/dir/afile1.dat
	testZipSrcDir1  = testZipSrcDir + "/dir1"        // testdata/zip/dir/dir1
	testZipSrcFile2 = testZipSrcDir1 + "/afile2.dat" // testdata/zip/dir/dir1/afile2.dat
	testZipSrcDir2  = testZipSrcDir1 + "/dir2"       // testdata/zip/dir/dir1/dir2
	testZipSrcDir3  = testZipSrcDir + "/dir3"        // testdata/zip/dir/dir3
	testZipSrcDir4  = testZipSrcDir3 + "/dir4"       // testdata/zip/dir/dir3/dir4
	testZipSrcFile3 = testZipSrcDir4 + "/file3.dat"  // testdata/zip/dir/dir3/dir4/file3.dat
	testZipSrcFile4 = testZipSrcDir3 + "/file4.dat"  // testdata/zip/dir/dir3/file4.dat
	testZipSrcFile5 = testZipSrcDir + "/file5.dat"   // testdata/zip/dir/file5.dat
)

func testCreateZipSrcFile(t *testing.T) {
	testCreateFile(t, testZipSrcFile)
}

func testCreateZipSrcDir(t *testing.T) {
	err := os.MkdirAll(testZipSrcDir, 0750)
	require.NoError(t, err)

	testCreateFile(t, testZipSrcFile1)
	err = os.Mkdir(testZipSrcDir1, 0750)
	require.NoError(t, err)
	testCreateFile2(t, testZipSrcFile2)
	err = os.Mkdir(testZipSrcDir2, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testZipSrcDir3, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testZipSrcDir4, 0750)
	require.NoError(t, err)
	testCreateFile(t, testZipSrcFile3)
	testCreateFile2(t, testZipSrcFile4)
	testCreateFile2(t, testZipSrcFile5)
}

func testRemoveZipDir(t *testing.T) {
	err := os.RemoveAll(testZipDir)
	require.NoError(t, err)
}

func testCheckZipWithFile(t *testing.T) {
	zipFile, err := zip.OpenReader(testZipDst)
	require.NoError(t, err)
	defer func() { _ = zipFile.Close() }()

	require.Len(t, zipFile.File, 1)
	rc, err := zipFile.File[0].Open()
	require.NoError(t, err)

	data, err := ioutil.ReadAll(rc)
	require.NoError(t, err)
	require.Equal(t, testsuite.Bytes(), data)

	err = rc.Close()
	require.NoError(t, err)
}

func testCheckZipWithDir(t *testing.T) {
	zipFile, err := zip.OpenReader(testZipDst)
	require.NoError(t, err)
	defer func() { _ = zipFile.Close() }()

	require.Len(t, zipFile.File, 10)
	for i, item := range [...]*struct {
		name  string
		data  []byte
		isDir bool
	}{
		{testZipSrcDir, nil, true},
		{testZipSrcFile1, testsuite.Bytes(), false},
		{testZipSrcDir1, nil, true},
		{testZipSrcFile2, bytes.Repeat(testsuite.Bytes(), 2), false},
		{testZipSrcDir2, nil, true},
		{testZipSrcDir3, nil, true},
		{testZipSrcDir4, nil, true},
		{testZipSrcFile3, testsuite.Bytes(), false},
		{testZipSrcFile4, bytes.Repeat(testsuite.Bytes(), 2), false},
		{testZipSrcFile5, bytes.Repeat(testsuite.Bytes(), 2), false},
	} {
		file := zipFile.File[i]
		// check is dir
		require.Equal(t, item.isDir, file.FileInfo().IsDir())
		// check name
		expectName := strings.ReplaceAll(item.name, testZipDir, "")
		if item.isDir {
			expectName += "/"
		}
		require.Equal(t, expectName, file.Name)
		// check file data
		if item.isDir {
			require.Equal(t, file.FileInfo().Size(), int64(0))
			continue
		}
		rc, err := file.Open()
		require.NoError(t, err)

		data, err := ioutil.ReadAll(rc)
		require.NoError(t, err)
		require.Equal(t, item.data, data)

		err = rc.Close()
		require.NoError(t, err)
	}
}

func testCheckZipWithMulti(t *testing.T) {
	zipFile, err := zip.OpenReader(testZipDst)
	require.NoError(t, err)
	defer func() { _ = zipFile.Close() }()

	require.Len(t, zipFile.File, 10+1)
	for i, item := range [...]*struct {
		name  string
		data  []byte
		isDir bool
	}{
		{testZipSrcDir, nil, true},
		{testZipSrcFile1, testsuite.Bytes(), false},
		{testZipSrcDir1, nil, true},
		{testZipSrcFile2, bytes.Repeat(testsuite.Bytes(), 2), false},
		{testZipSrcDir2, nil, true},
		{testZipSrcDir3, nil, true},
		{testZipSrcDir4, nil, true},
		{testZipSrcFile3, testsuite.Bytes(), false},
		{testZipSrcFile4, bytes.Repeat(testsuite.Bytes(), 2), false},
		{testZipSrcFile5, bytes.Repeat(testsuite.Bytes(), 2), false},
		{testZipSrcFile, testsuite.Bytes(), false},
	} {
		file := zipFile.File[i]
		// check name
		expectName := strings.ReplaceAll(item.name, testZipDir, "")
		if item.isDir {
			expectName += "/"
		}
		require.Equal(t, expectName, file.Name)
		// check is dir
		require.Equal(t, item.isDir, file.FileInfo().IsDir())
		// check file data
		if item.isDir {
			require.Equal(t, file.FileInfo().Size(), int64(0))
			continue
		}
		rc, err := file.Open()
		require.NoError(t, err)

		data, err := ioutil.ReadAll(rc)
		require.NoError(t, err)
		require.Equal(t, item.data, data)

		err = rc.Close()
		require.NoError(t, err)
	}
}

func TestZip(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("file", func(t *testing.T) {
		testCreateZipSrcFile(t)
		defer testRemoveZipDir(t)

		err := Zip(Cancel, testZipDst, testZipSrcFile)
		require.NoError(t, err)

		testCheckZipWithFile(t)
	})

	t.Run("directory", func(t *testing.T) {
		testCreateZipSrcDir(t)
		defer testRemoveZipDir(t)

		err := Zip(Cancel, testZipDst, testZipSrcDir)
		require.NoError(t, err)

		testCheckZipWithDir(t)
	})

	t.Run("multi", func(t *testing.T) {
		t.Run("file first", func(t *testing.T) {
			testCreateZipSrcFile(t)
			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			err := Zip(Cancel, testZipDst, testZipSrcFile, testZipSrcDir)
			require.NoError(t, err)

			testCheckZipWithMulti(t)
		})

		t.Run("directory first", func(t *testing.T) {
			testCreateZipSrcDir(t)
			testCreateZipSrcFile(t)
			defer testRemoveZipDir(t)

			err := Zip(Cancel, testZipDst, testZipSrcDir, testZipSrcFile)
			require.NoError(t, err)

			testCheckZipWithMulti(t)
		})
	})

	t.Run("empty path", func(t *testing.T) {
		err := Zip(Cancel, testZipDst)
		require.Error(t, err)
	})

	t.Run("path doesn't exist", func(t *testing.T) {
		const path = "not exist"

		t.Run("cancel", func(t *testing.T) {
			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCollectFailed, typ)
				require.Error(t, err)
				t.Log(err)
				count++
				return ErrCtrlOpCancel
			}
			err := Zip(ec, testZipDst, path)
			require.Equal(t, ErrUserCanceled, err)

			testIsNotExist(t, testZipDst)

			require.Equal(t, 1, count)
		})

		t.Run("skip", func(t *testing.T) {
			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCollectFailed, typ)
				require.Error(t, err)
				t.Log(err)
				count++
				return ErrCtrlOpSkip
			}
			err := Zip(ec, testZipDst, path)
			require.NoError(t, err)

			// it will a create a empty zip file
			err = os.Remove(testZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)
		})
	})
}

func TestZipWithContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateZipSrcDir(t)
		defer testRemoveZipDir(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := ZipWithContext(ctx, Cancel, testZipDst, testZipSrcDir)
		require.NoError(t, err)

		testCheckZipWithDir(t)
	})

	t.Run("cancel", func(t *testing.T) {
		testCreateZipSrcDir(t)
		defer testRemoveZipDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()
		err := ZipWithContext(ctx, Cancel, testZipDst, testZipSrcDir)
		require.Equal(t, context.Canceled, err)

		testIsNotExist(t, testZipDst)
	})
}

func TestZipWithNotice(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("mkdir-os.Stat", func(t *testing.T) {
		target, err := filepath.Abs(testZipSrcDir1)
		require.NoError(t, err)

		patch := func(name string) (os.FileInfo, error) {
			if name == target {
				return nil, monkey.Error
			}
			return os.Lstat(name)
		}
		pg := monkey.Patch(os.Stat, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckZipWithDir(t)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.EqualError(t, err, "unknown failed to zip operation code: 0")

			require.Equal(t, 1, count)
		})
	})

	t.Run("writeFile-os.Open", func(t *testing.T) {
		target, err := filepath.Abs(testZipSrcFile1)
		require.NoError(t, err)

		patch := func(name string) (*os.File, error) {
			if name == target {
				return nil, monkey.Error
			}
			return os.OpenFile(name, os.O_RDONLY, 0)
		}
		pg := monkey.Patch(os.Open, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckZipWithDir(t)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.EqualError(t, err, "unknown failed to zip operation code: 0")

			require.Equal(t, 1, count)
		})
	})

	t.Run("writeFile-srcFile.Stat", func(t *testing.T) {
		target, err := filepath.Abs(testZipSrcFile1)
		require.NoError(t, err)

		file := new(os.File)
		patch := func(file *os.File) (os.FileInfo, error) {
			if file.Name() == target {
				return nil, monkey.Error
			}
			return os.Stat(file.Name())
		}
		pg := monkey.PatchInstanceMethod(file, "Stat", patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckZipWithDir(t)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateZipSrcDir(t)
			defer testRemoveZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}

			err := Zip(ec, testZipDst, testZipSrcDir)
			require.EqualError(t, err, "unknown failed to zip operation code: 0")

			require.Equal(t, 1, count)
		})
	})
}

func TestZipTask_Progress(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateZipSrcDir(t)
		defer testRemoveZipDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		zt := NewZipTask(Cancel, nil, testZipDst, testZipSrcDir)

		done := make(chan struct{})
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				fmt.Println("progress:", zt.Progress())
				fmt.Println("detail:", zt.Detail())
				fmt.Println()
				time.Sleep(250 * time.Millisecond)
			}
		}()

		err := zt.Start()
		require.NoError(t, err)

		close(done)
		wg.Wait()

		fmt.Println("progress:", zt.Progress())
		fmt.Println("detail:", zt.Detail())

		rzt := zt.Task().(*zipTask)
		testsuite.IsDestroyed(t, zt)
		testsuite.IsDestroyed(t, rzt)

		testCheckZipWithDir(t)
	})

	t.Run("current > total", func(t *testing.T) {
		task := NewZipTask(Cancel, nil, testZipDst, testZipSrcDir)
		zt := task.Task().(*zipTask)

		zt.current.SetUint64(1000)
		zt.total.SetUint64(10)

		t.Log(task.Progress())
	})

	t.Run("too long value", func(t *testing.T) {
		task := NewZipTask(Cancel, nil, testZipDst, testZipSrcDir)
		zt := task.Task().(*zipTask)

		zt.current.SetUint64(1)
		zt.total.SetUint64(7)

		t.Log(task.Progress())
	})

	t.Run("invalid value", func(t *testing.T) {
		patch := func(s string, bitSize int) (float64, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(strconv.ParseFloat, patch)
		defer pg.Unpatch()

		task := NewZipTask(Cancel, nil, testZipDst, testZipSrcDir)
		zt := task.Task().(*zipTask)

		zt.current.SetUint64(1)
		zt.total.SetUint64(7)

		t.Log(task.Progress())
	})

	t.Run("too long progress", func(t *testing.T) {
		task := NewZipTask(Cancel, nil, testZipDst, testZipSrcDir)
		zt := task.Task().(*zipTask)

		// 3% -> 2.98%
		zt.current.SetUint64(3)
		zt.total.SetUint64(100)

		t.Log(task.Progress())
	})
}

func TestZipTask_Watcher(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	pg1 := testPatchTaskCanceled()
	defer pg1.Unpatch()

	pg2 := testPatchMultiTaskWatcher()
	defer pg2.Unpatch()

	testCreateZipSrcDir(t)
	defer testRemoveZipDir(t)

	err := Zip(Cancel, testZipDst, testZipSrcDir)
	require.NoError(t, err)

	testCheckZipWithDir(t)
}
