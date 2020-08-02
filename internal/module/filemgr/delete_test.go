package filemgr

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

const (
	testDeleteDir     = "testdata/delete/"          // delete test root path
	testDeleteSrcFile = testDeleteDir + "file1.dat" // src is file
	testDeleteSrcDir  = testDeleteDir + "dir"       // src is directory

	// files in the test directory
	testDeleteSrcFile1 = testDeleteSrcDir + "/afile1.dat"  // testdata/delete/dir/afile1.dat
	testDeleteSrcDir1  = testDeleteSrcDir + "/dir1"        // testdata/delete/dir/dir1
	testDeleteSrcFile2 = testDeleteSrcDir1 + "/afile2.dat" // testdata/delete/dir/dir1/afile2.dat
	testDeleteSrcDir2  = testDeleteSrcDir1 + "/dir2"       // testdata/delete/dir/dir1/dir2
	testDeleteSrcDir3  = testDeleteSrcDir + "/dir3"        // testdata/delete/dir/dir3
	testDeleteSrcDir4  = testDeleteSrcDir3 + "/dir4"       // testdata/delete/dir/dir3/dir4
	testDeleteSrcFile3 = testDeleteSrcDir4 + "/file3.dat"  // testdata/delete/dir/dir3/dir4/file3.dat
	testDeleteSrcFile4 = testDeleteSrcDir3 + "/file4.dat"  // testdata/delete/dir/dir3/file4.dat
	testDeleteSrcFile5 = testDeleteSrcDir + "/file5.dat"   // testdata/delete/dir/file5.dat
)

func testCreateDeleteSrcFile(t *testing.T) {
	testCreateFile(t, testDeleteSrcFile)
}

func testCreateDeleteSrcDir(t *testing.T) {
	err := os.MkdirAll(testDeleteSrcDir, 0750)
	require.NoError(t, err)

	testCreateFile(t, testDeleteSrcFile1)
	err = os.Mkdir(testDeleteSrcDir1, 0750)
	require.NoError(t, err)
	testCreateFile2(t, testDeleteSrcFile2)
	err = os.Mkdir(testDeleteSrcDir2, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testDeleteSrcDir3, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testDeleteSrcDir4, 0750)
	require.NoError(t, err)
	testCreateFile(t, testDeleteSrcFile3)
	testCreateFile2(t, testDeleteSrcFile4)
	testCreateFile2(t, testDeleteSrcFile5)
}

func testRemoveDeleteDir(t *testing.T) {
	err := os.RemoveAll(testDeleteDir)
	require.NoError(t, err)
}

func TestDelete(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("file", func(t *testing.T) {
		testCreateDeleteSrcFile(t)
		defer testRemoveDeleteDir(t)

		err := Delete(Cancel, testDeleteSrcFile)
		require.NoError(t, err)

		testIsNotExist(t, testDeleteSrcFile)
	})

	t.Run("directory", func(t *testing.T) {
		testCreateDeleteSrcDir(t)
		defer testRemoveDeleteDir(t)

		err := Delete(Cancel, testDeleteSrcDir)
		require.NoError(t, err)

		testIsNotExist(t, testDeleteSrcDir)
	})

	t.Run("multi", func(t *testing.T) {
		t.Run("file first", func(t *testing.T) {
			testCreateDeleteSrcFile(t)
			testCreateDeleteSrcDir(t)
			defer testRemoveDeleteDir(t)

			err := Delete(Cancel, testDeleteSrcFile, testDeleteSrcDir)
			require.NoError(t, err)

			testIsNotExist(t, testDeleteSrcFile)
			testIsNotExist(t, testDeleteSrcDir)
		})

		t.Run("directory first", func(t *testing.T) {
			testCreateDeleteSrcDir(t)
			testCreateDeleteSrcFile(t)
			defer testRemoveDeleteDir(t)

			err := Delete(Cancel, testDeleteSrcDir, testDeleteSrcFile)
			require.NoError(t, err)

			testIsNotExist(t, testDeleteSrcDir)
			testIsNotExist(t, testDeleteSrcFile)
		})
	})

	t.Run("empty path", func(t *testing.T) {
		err := Delete(Cancel)
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
			err := Delete(ec, path)
			require.Equal(t, ErrUserCanceled, err)

			testIsNotExist(t, path)
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
			err := Delete(ec, path)
			require.NoError(t, err)

			testIsNotExist(t, path)
			require.Equal(t, 1, count)
		})
	})
}

func TestDeleteWithContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateDeleteSrcDir(t)
		defer testRemoveDeleteDir(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := DeleteWithContext(ctx, Cancel, testDeleteSrcDir)
		require.NoError(t, err)

		testIsNotExist(t, testDeleteSrcDir)
	})

	t.Run("cancel", func(t *testing.T) {
		testCreateDeleteSrcDir(t)
		defer testRemoveDeleteDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()
		err := DeleteWithContext(ctx, Cancel, testDeleteSrcDir)
		require.Equal(t, context.Canceled, err)

		testIsExist(t, testDeleteSrcDir)
	})
}

func TestDeleteWithNotice(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("deleteDirFile-os.Remove", func(t *testing.T) {
		patch := func(string) error {
			return monkey.Error
		}
		pg := monkey.Patch(os.Remove, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer pg.Restore()

			testCreateDeleteSrcDir(t)
			defer testRemoveDeleteDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlDeleteFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}
			err := Delete(ec, testDeleteSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)
			testIsNotExist(t, testDeleteSrcDir)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateDeleteSrcDir(t)
			defer testRemoveDeleteDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlDeleteFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err := Delete(ec, testDeleteSrcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)
			testIsExist(t, testDeleteSrcDir)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateDeleteSrcDir(t)
			defer testRemoveDeleteDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlDeleteFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err := Delete(ec, testDeleteSrcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)
			testIsExist(t, testDeleteSrcDir)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateDeleteSrcDir(t)
			defer testRemoveDeleteDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlDeleteFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err := Delete(ec, testDeleteSrcDir)
			errStr := "failed to delete directory: unknown failed to delete operation code: 0"
			require.EqualError(t, err, errStr)

			require.Equal(t, 1, count)
			testIsExist(t, testDeleteSrcDir)
		})
	})
}

func TestDelete_File(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()
}

func TestDelete_Directory(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()
}

func TestDelete_Multi(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()
}

func TestDeleteTask_Progress(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateDeleteSrcDir(t)
		defer testRemoveDeleteDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		dt := NewDeleteTask(Cancel, nil, testDeleteSrcDir)

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
				fmt.Println("progress:", dt.Progress())
				fmt.Println("detail:", dt.Detail())
				fmt.Println()
				time.Sleep(250 * time.Millisecond)
			}
		}()

		err := dt.Start()
		require.NoError(t, err)

		close(done)
		wg.Wait()

		fmt.Println("progress:", dt.Progress())
		fmt.Println("detail:", dt.Detail())

		rdt := dt.Task().(*deleteTask)
		testsuite.IsDestroyed(t, dt)
		testsuite.IsDestroyed(t, rdt)

		testIsNotExist(t, testDeleteSrcDir)
	})

	t.Run("current > total", func(t *testing.T) {
		task := NewDeleteTask(Cancel, nil, testDeleteSrcDir)
		dt := task.Task().(*deleteTask)

		dt.current.SetUint64(1000)
		dt.total.SetUint64(10)

		t.Log(dt.Progress())
	})

	t.Run("too long value", func(t *testing.T) {
		task := NewDeleteTask(Cancel, nil, testDeleteSrcDir)
		dt := task.Task().(*deleteTask)

		dt.current.SetUint64(1)
		dt.total.SetUint64(7)

		t.Log(dt.Progress())
	})

	t.Run("invalid value", func(t *testing.T) {
		patch := func(s string, bitSize int) (float64, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(strconv.ParseFloat, patch)
		defer pg.Unpatch()

		task := NewDeleteTask(Cancel, nil, testDeleteSrcDir)
		dt := task.Task().(*deleteTask)

		dt.current.SetUint64(1)
		dt.total.SetUint64(7)

		t.Log(dt.Progress())
	})

	t.Run("too long progress", func(t *testing.T) {
		task := NewDeleteTask(Cancel, nil, testDeleteSrcDir)
		dt := task.Task().(*deleteTask)

		// 3% -> 2.98%
		dt.current.SetUint64(3)
		dt.total.SetUint64(100)

		t.Log(dt.Progress())
	})
}

func TestDeleteTask_Watcher(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	pg1 := testPatchTaskCanceled()
	defer pg1.Unpatch()

	pg2 := testPatchMultiTaskWatcher()
	defer pg2.Unpatch()

	testCreateDeleteSrcDir(t)
	defer testRemoveDeleteDir(t)

	err := Delete(Cancel, testDeleteSrcDir)
	require.NoError(t, err)

	testIsNotExist(t, testDeleteSrcDir)
}
