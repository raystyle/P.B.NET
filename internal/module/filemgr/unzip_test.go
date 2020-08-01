package filemgr

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testUnZipDir = "testdata/unzip/" // unzip test root path

	testUnZipFileZip = testUnZipDir + "unzip_file.zip" // source zip file include a file
	testUnZipDirZip  = testUnZipDir + "unzip_dir.zip"  // source zip file include a directory
	testUnZipDst     = testUnZipDir + "dst"            // store extracted file

	// resource path
	testUnZipSrcFile = testUnZipDir + "file1.dat"
	testUnZipSrcDir  = testUnZipDir + "dir"

	// files in the test directory
	testUnZipSrcFile1 = testUnZipSrcDir + "/afile1.dat"  // testdata/unzip/dir/afile1.dat
	testUnZipSrcDir1  = testUnZipSrcDir + "/dir1"        // testdata/unzip/dir/dir1
	testUnZipSrcFile2 = testUnZipSrcDir1 + "/afile2.dat" // testdata/unzip/dir/dir1/afile2.dat
	testUnZipSrcDir2  = testUnZipSrcDir1 + "/dir2"       // testdata/unzip/dir/dir1/dir2
	testUnZipSrcDir3  = testUnZipSrcDir + "/dir3"        // testdata/unzip/dir/dir3
	testUnZipSrcDir4  = testUnZipSrcDir3 + "/dir4"       // testdata/unzip/dir/dir3/dir4
	testUnZipSrcFile3 = testUnZipSrcDir4 + "/file3.dat"  // testdata/unzip/dir/dir3/dir4/file3.dat
	testUnZipSrcFile4 = testUnZipSrcDir3 + "/file4.dat"  // testdata/unzip/dir/dir3/file4.dat
	testUnZipSrcFile5 = testUnZipSrcDir + "/file5.dat"   // testdata/unzip/dir/file5.dat
)

func testCreateUnZipSrcFile(t *testing.T) {
	testCreateFile(t, testUnZipSrcFile)
}

func testCreateUnZipSrcDir(t *testing.T) {
	err := os.MkdirAll(testUnZipSrcDir, 0750)
	require.NoError(t, err)

	testCreateFile(t, testUnZipSrcFile1)
	err = os.Mkdir(testUnZipSrcDir1, 0750)
	require.NoError(t, err)
	testCreateFile2(t, testUnZipSrcFile2)
	err = os.Mkdir(testUnZipSrcDir2, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testUnZipSrcDir3, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testUnZipSrcDir4, 0750)
	require.NoError(t, err)
	testCreateFile(t, testUnZipSrcFile3)
	testCreateFile2(t, testUnZipSrcFile4)
	testCreateFile2(t, testUnZipSrcFile5)
}

func testCreateUnZipFileZip(t *testing.T) {
	testCreateUnZipSrcFile(t)
	err := Zip(Cancel, testUnZipFileZip, testUnZipSrcFile)
	require.NoError(t, err)
}

func testCreateUnZipDirZip(t *testing.T) {
	testCreateUnZipSrcDir(t)
	err := Zip(Cancel, testUnZipDirZip, testUnZipSrcDir)
	require.NoError(t, err)
}

func testRemoveUnzipDir(t *testing.T) {
	err := os.RemoveAll(testUnZipDir)
	require.NoError(t, err)
}
