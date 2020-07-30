package filemgr

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

const (
	testZipDir     = "testdata/zip/"             // zip test root path
	testZipFile    = testZipDir + "zip_test.zip" // destination zip file path
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

}

func testCheckZipWithDir(t *testing.T) {

}

func TestZip(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("file", func(t *testing.T) {
		testCreateZipSrcFile(t)
		defer testRemoveZipDir(t)

		err := Zip(SkipAll, testZipFile, testZipSrcFile)
		require.NoError(t, err)

		testCheckZipWithFile(t)
	})

	t.Run("directory", func(t *testing.T) {
		testCreateZipSrcDir(t)
		defer testRemoveZipDir(t)

		err := Zip(SkipAll, testZipFile, testZipSrcDir)
		require.NoError(t, err)

		testCheckZipWithDir(t)
	})
}
