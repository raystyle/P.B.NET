package filemgr

import (
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
	testMoveDir = "testdata/move/" // move test root path

	testMoveSrcFile = testMoveDir + "file1.dat" // source path is a file
	testMoveSrcDir  = testMoveDir + "dir"       // source path is a directory

	// destination path
	testMoveDst     = testMoveDir + "dst"        // store extracted file
	testMoveDstFile = testMoveDst + "/file1.dat" // testdata/move/dst/file1.dat
	testMoveDstDir  = testMoveDst + "/dir"       // testdata/move/dst/dir

	// src files in directory
	testMoveSrcFile1 = testMoveSrcDir + "/afile1.dat"  // testdata/move/dir/afile1.dat
	testMoveSrcDir1  = testMoveSrcDir + "/dir1"        // testdata/move/dir/dir1
	testMoveSrcFile2 = testMoveSrcDir1 + "/afile2.dat" // testdata/move/dir/dir1/afile2.dat
	testMoveSrcDir2  = testMoveSrcDir1 + "/dir2"       // testdata/move/dir/dir1/dir2
	testMoveSrcDir3  = testMoveSrcDir + "/dir3"        // testdata/move/dir/dir3
	testMoveSrcDir4  = testMoveSrcDir3 + "/dir4"       // testdata/move/dir/dir3/dir4
	testMoveSrcFile3 = testMoveSrcDir4 + "/file3.dat"  // testdata/move/dir/dir3/dir4/file3.dat
	testMoveSrcFile4 = testMoveSrcDir3 + "/file4.dat"  // testdata/move/dir/dir3/file4.dat
	testMoveSrcFile5 = testMoveSrcDir + "/file5.dat"   // testdata/move/dir/file5.dat
)

func testCreateMoveSrcFile(t *testing.T) {
	testCreateFile(t, testMoveSrcFile)
}

func testCreateMoveSrcDir(t *testing.T) {
	err := os.MkdirAll(testMoveSrcDir, 0750)
	require.NoError(t, err)

	testCreateFile(t, testMoveSrcFile1)
	err = os.Mkdir(testMoveSrcDir1, 0750)
	require.NoError(t, err)
	testCreateFile2(t, testMoveSrcFile2)
	err = os.Mkdir(testMoveSrcDir2, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testMoveSrcDir3, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testMoveSrcDir4, 0750)
	require.NoError(t, err)
	testCreateFile(t, testMoveSrcFile3)
	testCreateFile2(t, testMoveSrcFile4)
	testCreateFile2(t, testMoveSrcFile5)
}

func testCreateMoveSrcMulti(t *testing.T) {
	testCreateMoveSrcFile(t)
	testCreateMoveSrcDir(t)
}

func testCheckMoveDstFile(t *testing.T) {
	testIsNotExist(t, testMoveSrcFile)

	file, err := os.Open(testMoveDstFile)
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	require.NoError(t, err)

	require.Equal(t, int64(testsuite.TestDataSize), stat.Size())
	require.Equal(t, false, stat.IsDir())

	data, err := ioutil.ReadAll(file)
	require.NoError(t, err)
	require.Equal(t, testsuite.Bytes(), data)
}

func testCheckMoveDstDir(t *testing.T) {
	testIsNotExist(t, testMoveSrcDir)

	fileData1 := testsuite.Bytes()
	fileData2 := bytes.Repeat(testsuite.Bytes(), 2)

	for _, item := range [...]*struct {
		path  string
		size  int
		isDir bool
		data  []byte
	}{
		{testMoveSrcFile1, testsuite.TestDataSize, false, fileData1},
		{testMoveSrcDir1, 0, true, nil},
		{testMoveSrcFile2, testsuite.TestDataSize * 2, false, fileData2},
		{testMoveSrcDir2, 0, true, nil},
		{testMoveSrcDir3, 0, true, nil},
		{testMoveSrcDir4, 0, true, nil},
		{testMoveSrcFile3, testsuite.TestDataSize, false, fileData1},
		{testMoveSrcFile4, testsuite.TestDataSize * 2, false, fileData2},
		{testMoveSrcFile5, testsuite.TestDataSize * 2, false, fileData2},
	} {
		path := strings.ReplaceAll(item.path, testMoveSrcDir, testMoveDstDir)

		file, err := os.Open(path)
		require.NoError(t, err)

		stat, err := file.Stat()
		require.NoError(t, err)

		require.Equal(t, int64(item.size), stat.Size())
		require.Equal(t, item.isDir, stat.IsDir())

		if !item.isDir {
			data, err := ioutil.ReadAll(file)
			require.NoError(t, err)
			require.Equal(t, item.data, data)
		}

		err = file.Close()
		require.NoError(t, err)
	}
}

func testCheckMoveDstMulti(t *testing.T) {
	testCheckMoveDstFile(t)
	testCheckMoveDstDir(t)
}

func testRemoveMoveDir(t *testing.T) {
	err := os.RemoveAll(testMoveDir)
	require.NoError(t, err)
}

func TestMove(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("fast mode", func(t *testing.T) {
		t.Run("file", func(t *testing.T) {
			testCreateMoveSrcFile(t)
			defer testRemoveMoveDir(t)

			err := Move(Cancel, testMoveDst, testMoveSrcFile)
			require.NoError(t, err)

			testCheckMoveDstFile(t)
		})

		t.Run("directory", func(t *testing.T) {
			testCreateMoveSrcDir(t)
			defer testRemoveMoveDir(t)

			err := Move(Cancel, testMoveDst, testMoveSrcDir)
			require.NoError(t, err)

			testCheckMoveDstDir(t)
		})

		t.Run("multi", func(t *testing.T) {
			t.Run("file first", func(t *testing.T) {
				testCreateMoveSrcMulti(t)
				defer testRemoveMoveDir(t)

				err := Move(Cancel, testMoveDst, testMoveSrcFile, testMoveSrcDir)
				require.NoError(t, err)

				testCheckMoveDstMulti(t)
			})

			t.Run("directory first", func(t *testing.T) {
				testCreateMoveSrcMulti(t)
				defer testRemoveMoveDir(t)

				err := Move(Cancel, testMoveDst, testMoveSrcDir, testMoveSrcFile)
				require.NoError(t, err)

				testCheckMoveDstMulti(t)
			})
		})
	})

	t.Run("common mode", func(t *testing.T) {
		// force in common mode
		patch := func(string) string {
			return ""
		}
		pg := monkey.Patch(filepath.VolumeName, patch)
		defer pg.Unpatch()

		t.Run("file", func(t *testing.T) {
			testCreateMoveSrcFile(t)
			defer testRemoveMoveDir(t)

			err := Move(Cancel, testMoveDst, testMoveSrcFile)
			require.NoError(t, err)

			testCheckMoveDstFile(t)
		})

		t.Run("directory", func(t *testing.T) {
			testCreateMoveSrcDir(t)
			defer testRemoveMoveDir(t)

			err := Move(Cancel, testMoveDst, testMoveSrcDir)
			require.NoError(t, err)

			testCheckMoveDstDir(t)
		})

		t.Run("multi", func(t *testing.T) {
			t.Run("file first", func(t *testing.T) {
				testCreateMoveSrcMulti(t)
				defer testRemoveMoveDir(t)

				err := Move(Cancel, testMoveDst, testMoveSrcFile, testMoveSrcDir)
				require.NoError(t, err)

				testCheckMoveDstMulti(t)
			})

			t.Run("directory first", func(t *testing.T) {
				testCreateMoveSrcMulti(t)
				defer testRemoveMoveDir(t)

				err := Move(Cancel, testMoveDst, testMoveSrcDir, testMoveSrcFile)
				require.NoError(t, err)

				testCheckMoveDstMulti(t)
			})
		})
	})
}

func TestMoveWithContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateMoveSrcDir(t)
		defer testRemoveMoveDir(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := MoveWithContext(ctx, Cancel, testMoveDst, testMoveSrcDir)
		require.NoError(t, err)

		testCheckMoveDstDir(t)
	})

	t.Run("cancel", func(t *testing.T) {
		testCreateMoveSrcDir(t)
		defer testRemoveMoveDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()
		err := MoveWithContext(ctx, Cancel, testMoveDst, testMoveSrcDir)
		require.Equal(t, context.Canceled, err)

		testIsNotExist(t, testMoveDst)
	})
}

func TestMoveWithNotice(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("failed to collect", func(t *testing.T) {
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
			err := Move(ec, testMoveDst, path)
			require.Equal(t, ErrUserCanceled, err)

			require.Equal(t, 1, count)

			testIsNotExist(t, path)
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
			err := Move(ec, testMoveDst, path)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsNotExist(t, path)
		})
	})

	t.Run("mkdir-stat", func(t *testing.T) {
		target, err := filepath.Abs(testMoveDstDir + "/dir1")
		require.NoError(t, err)

		var pg *monkey.PatchGuard
		patch := func(name string) (os.FileInfo, error) {
			if name == target {
				return nil, monkey.Error
			}
			pg.Unpatch()
			defer pg.Restore()
			return stat(name)
		}
		pg = monkey.Patch(stat, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer pg.Restore()

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlMoveFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckMoveDstMulti(t)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlMoveFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}

			err := Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsExist(t, testMoveDst)
			testIsNotExist(t, target)
			testCheckMoveDstFile(t)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlMoveFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}

			err := Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testIsExist(t, testMoveDst)
			testIsNotExist(t, target)
			testIsNotExist(t, testMoveDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlMoveFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}

			err := Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.EqualError(t, errors.Cause(err), "unknown failed to move operation code: 0")

			require.Equal(t, 1, count)

			testIsExist(t, testMoveDst)
			testIsNotExist(t, target)
			testIsNotExist(t, testMoveDstFile)
		})
	})

	t.Run("mkdir-SameDirFile", func(t *testing.T) {
		t.Run("retry", func(t *testing.T) {
			// create same name file with directory
			target, err := filepath.Abs(testMoveDstDir + "/dir1")
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				err = os.Remove(target)
				require.NoError(t, err)
				return ErrCtrlOpRetry
			}

			err = Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckMoveDstMulti(t)
		})

		t.Run("skip", func(t *testing.T) {
			// create same name file with directory
			target, err := filepath.Abs(testMoveDstDir + "/dir1")
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpSkip
			}

			err = Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsExist(t, testMoveDst)
			testIsExist(t, target)
			testIsExist(t, testMoveDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			// create same name file with directory
			target, err := filepath.Abs(testMoveDstDir + "/dir1")
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpCancel
			}

			err = Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testIsExist(t, testMoveDst)
			testIsExist(t, target)
			testIsNotExist(t, testMoveDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			// create same name file with directory
			target, err := filepath.Abs(testMoveDstDir + "/dir1")
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpInvalid
			}

			err = Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.EqualError(t, errors.Cause(err), "unknown same dir file operation code: 0")

			require.Equal(t, 1, count)

			testIsExist(t, testMoveDst)
			testIsExist(t, target)
			testIsNotExist(t, testMoveDstFile)
		})
	})

	t.Run("mkdir-os.MkdirAll", func(t *testing.T) {
		target, err := filepath.Abs(testMoveDstDir + "/dir1")
		require.NoError(t, err)

		var pg *monkey.PatchGuard
		patch := func(name string, perm os.FileMode) error {
			if name == target {
				return monkey.Error
			}
			pg.Unpatch()
			defer pg.Restore()
			return os.Mkdir(name, perm)
		}
		pg = monkey.Patch(os.Mkdir, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer pg.Restore()

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlMoveFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckMoveDstMulti(t)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlMoveFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err := Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsExist(t, testMoveDstDir)
			testIsNotExist(t, target)
			testCheckMoveDstFile(t)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlMoveFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err := Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testIsExist(t, testMoveDst)
			testIsNotExist(t, target)
			testIsNotExist(t, testMoveDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateMoveSrcMulti(t)
			defer testRemoveMoveDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlMoveFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err := Move(ec, testMoveDst, testMoveSrcFile, testMoveSrcDir)
			require.EqualError(t, errors.Cause(err), "unknown failed to move operation code: 0")

			require.Equal(t, 1, count)

			testIsExist(t, testMoveDst)
			testIsNotExist(t, target)
			testIsNotExist(t, testMoveDstFile)
		})
	})

}

func TestMoveTask_Prepare(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()
}

func TestMoveTask_Process(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("destination directory already exists", func(t *testing.T) {
		testCreateMoveSrcMulti(t)
		defer testRemoveMoveDir(t)

		err := os.MkdirAll(testMoveDstDir, 0750)
		require.NoError(t, err)

		err = Move(Cancel, testMoveDst, testMoveSrcFile, testMoveSrcDir)
		require.NoError(t, err)

		testCheckMoveDstMulti(t)
	})
}

func TestMoveTask_Progress(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateMoveSrcDir(t)
		defer testRemoveMoveDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		mt := NewMoveTask(Cancel, nil, testMoveDst, testMoveSrcDir)

		done := make(chan struct{})
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				fmt.Println("progress:", mt.Progress())
				fmt.Println("detail:", mt.Detail())
				fmt.Println()
				select {
				case <-done:
					return
				case <-time.After(250 * time.Millisecond):
				}
			}
		}()

		err := mt.Start()
		require.NoError(t, err)

		close(done)
		wg.Wait()

		fmt.Println("progress:", mt.Progress())
		fmt.Println("detail:", mt.Detail())

		rmt := mt.Task().(*moveTask)
		testsuite.IsDestroyed(t, mt)
		testsuite.IsDestroyed(t, rmt)

		testCheckMoveDstDir(t)
	})

	t.Run("current > total", func(t *testing.T) {
		task := NewMoveTask(Cancel, nil, testMoveDst, testMoveSrcDir)
		mt := task.Task().(*moveTask)

		mt.current.SetUint64(1000)
		mt.total.SetUint64(10)

		t.Log(task.Progress())
	})

	t.Run("too long value", func(t *testing.T) {
		task := NewMoveTask(Cancel, nil, testMoveDst, testMoveSrcDir)
		mt := task.Task().(*moveTask)

		mt.current.SetUint64(1)
		mt.total.SetUint64(7)

		t.Log(task.Progress())
	})

	t.Run("invalid value", func(t *testing.T) {
		patch := func(s string, bitSize int) (float64, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(strconv.ParseFloat, patch)
		defer pg.Unpatch()

		task := NewMoveTask(Cancel, nil, testMoveDst, testMoveSrcDir)
		mt := task.Task().(*moveTask)

		mt.current.SetUint64(1)
		mt.total.SetUint64(7)

		t.Log(task.Progress())
	})

	t.Run("too long progress", func(t *testing.T) {
		task := NewMoveTask(Cancel, nil, testMoveDst, testMoveSrcDir)
		mt := task.Task().(*moveTask)

		// 3% -> 2.98%
		mt.current.SetUint64(3)
		mt.total.SetUint64(100)

		t.Log(task.Progress())
	})
}

func TestMoveTask_Watcher(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	pg1 := testPatchTaskCanceled()
	defer pg1.Unpatch()

	pg2 := testPatchMultiTaskWatcher()
	defer pg2.Unpatch()

	testCreateMoveSrcDir(t)
	defer testRemoveMoveDir(t)

	err := Move(Cancel, testMoveDst, testMoveSrcDir)
	require.NoError(t, err)

	testCheckMoveDstDir(t)
}
