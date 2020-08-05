package filemgr

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
