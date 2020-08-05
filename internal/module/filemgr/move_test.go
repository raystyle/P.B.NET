package filemgr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/testsuite"
)

const (
	testMoveDir     = "testdata/move/"          // move test root path
	testMoveDstDir  = testMoveDir + "dst"       // destination directory path
	testMoveSrcFile = testMoveDir + "file1.dat" // src is file
	testMoveSrcDir  = testMoveDir + "dir"       // src is directory

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

			err := Move(Cancel, testMoveDstDir, testMoveSrcFile)
			require.NoError(t, err)
		})

		t.Run("directory", func(t *testing.T) {
			testCreateMoveSrcDir(t)
			defer testRemoveMoveDir(t)

			err := Move(Cancel, testMoveDstDir, testMoveSrcDir)
			require.NoError(t, err)
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

			err := Move(Cancel, testMoveDstDir, testMoveSrcFile)
			require.NoError(t, err)
		})

		t.Run("directory", func(t *testing.T) {
			testCreateMoveSrcDir(t)
			defer testRemoveMoveDir(t)

			err := Move(Cancel, testMoveDstDir, testMoveSrcDir)
			require.NoError(t, err)
		})
	})
}
