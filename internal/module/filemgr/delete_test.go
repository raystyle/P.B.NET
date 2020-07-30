package filemgr

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

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

		err := Delete(SkipAll, testDeleteSrcFile)
		require.NoError(t, err)

		testIsNotExist(t, testDeleteSrcFile)
	})

	t.Run("directory", func(t *testing.T) {
		testCreateDeleteSrcDir(t)
		defer testRemoveDeleteDir(t)

		err := Delete(SkipAll, testDeleteSrcDir)
		require.NoError(t, err)

		testIsNotExist(t, testDeleteSrcDir)
	})

	t.Run("multi", func(t *testing.T) {
		t.Run("file first", func(t *testing.T) {
			testCreateDeleteSrcFile(t)
			testCreateDeleteSrcDir(t)
			defer testRemoveDeleteDir(t)

			err := Delete(SkipAll, testDeleteSrcFile, testDeleteSrcDir)
			require.NoError(t, err)

			testIsNotExist(t, testDeleteSrcFile)
			testIsNotExist(t, testDeleteSrcDir)
		})

		t.Run("directory first", func(t *testing.T) {
			testCreateDeleteSrcDir(t)
			testCreateDeleteSrcFile(t)
			defer testRemoveDeleteDir(t)

			err := Delete(SkipAll, testDeleteSrcDir, testDeleteSrcFile)
			require.NoError(t, err)

			testIsNotExist(t, testDeleteSrcDir)
			testIsNotExist(t, testDeleteSrcFile)
		})
	})

	t.Run("path doesn't exist", func(t *testing.T) {
		const path = "not exist"

		count := 0
		ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
			require.Equal(t, ErrCtrlCollectFailed, typ)
			require.Error(t, err)
			count++
			return ErrCtrlOpSkip
		}
		err := Delete(ec, path)
		require.NoError(t, err)

		testIsNotExist(t, path)
		require.Equal(t, 1, count)
	})

	t.Run("failed to remove file", func(t *testing.T) {
		testCreateDeleteSrcFile(t)
		defer testRemoveDeleteDir(t)

		patch := func(string) error {
			return monkey.Error
		}
		pg := monkey.Patch(os.Remove, patch)
		defer pg.Unpatch()

		count := 0
		ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
			require.Equal(t, ErrCtrlDeleteFailed, typ)
			require.Error(t, err)
			count++
			return ErrCtrlOpSkip
		}
		err := Delete(ec, testDeleteSrcFile)
		require.NoError(t, err)

		testIsExist(t, testDeleteSrcFile)
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

	pg := testPatchTaskCanceled()
	defer pg.Unpatch()

	t.Run("common", func(t *testing.T) {
		testCreateDeleteSrcDir(t)
		defer testRemoveDeleteDir(t)

		dt := NewDeleteTask(SkipAll, nil, testDeleteSrcDir)

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
}
