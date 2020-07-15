package filemgr

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

const (
	testMoveDir = "testdata/move/"

	// src is file
	testMoveSrcFile = testMoveDir + "file1.dat"

	// src is directory
	//
	// testdata/dir
	// testdata/dir/afile1.dat
	// testdata/dir/dir1
	// testdata/dir/dir1/afile2.dat
	// testdata/dir/dir1/dir2
	// testdata/dir/dir3
	// testdata/dir/dir3/dir4
	// testdata/dir/dir3/dir4/file3.dat
	// testdata/dir/dir3/file4.dat
	// testdata/dir/file5.dat

	testMoveSrcDir = testMoveDir + "dir"
	testMoveDstDir = testMoveDir + "dir-dir"

	testMoveSrcFile1 = testMoveSrcDir + "/afile1.dat"
	testMoveSrcDir1  = testMoveSrcDir + "/dir1"
	testMoveSrcFile2 = testMoveSrcDir1 + "/afile2.dat"
	testMoveSrcDir2  = testMoveSrcDir1 + "/dir2"
	testMoveSrcDir3  = testMoveSrcDir + "/dir3"
	testMoveSrcDir4  = testMoveSrcDir3 + "/dir4"
	testMoveSrcFile3 = testMoveSrcDir4 + "/file3.dat"
	testMoveSrcFile4 = testMoveSrcDir3 + "/file4.dat"
	testMoveSrcFile5 = testMoveSrcDir + "/file5.dat"
)

func testCreateMoveSrcFile(t *testing.T) {
	testCreateFile(t, testMoveSrcFile)
}

func testRemoveMoveSrcFile(t *testing.T) {
	err := os.Remove(testMoveSrcFile)
	require.NoError(t, err)
}

func testCreateMoveSrcDir(t *testing.T) {

}

func TestMove(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("src is file", func(t *testing.T) {
		t.Run("to file path", func(t *testing.T) {

		})

		t.Run("to directory path", func(t *testing.T) {

		})
	})

	t.Run("src is directory", func(t *testing.T) {
		t.Run("to directory path", func(t *testing.T) {

		})

		t.Run("to file path", func(t *testing.T) {

		})
	})
}
