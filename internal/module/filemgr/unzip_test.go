package filemgr

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

const (
	testUnZipDir = "testdata/unzip/" // unzip test root path

	testUnZipFileZip  = testUnZipDir + "unzip_file.zip"  // source zip file include a file
	testUnZipDirZip   = testUnZipDir + "unzip_dir.zip"   // source zip file include a directory
	testUnZipMultiZip = testUnZipDir + "unzip_multi.zip" // source zip file include a file and directory
	testUnZipDst      = testUnZipDir + "dst"             // store extracted file

	// destination path
	testUnZipDstFile = testUnZipDst + "/file1.dat" // testdata/unzip/dst/file1.dat
	testUnZipDstDir  = testUnZipDst + "/dir"       // testdata/unzip/dst/dir

	// resource path
	testUnZipSrcFile = testUnZipDir + "file1.dat" // testdata/unzip/file1.dat
	testUnZipSrcDir  = testUnZipDir + "dir"       // testdata/unzip/dir

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

func testCreateUnZipMultiZip(t *testing.T) {
	testCreateUnZipSrcFile(t)
	testCreateUnZipSrcDir(t)
	err := Zip(Cancel, testUnZipMultiZip, testUnZipSrcFile, testUnZipSrcDir)
	require.NoError(t, err)
}

func testRemoveUnZipDir(t *testing.T) {
	err := os.RemoveAll(testUnZipDir)
	require.NoError(t, err)
}

func TestUnZip(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("file", func(t *testing.T) {
		testCreateUnZipFileZip(t)
		defer testRemoveUnZipDir(t)

		err := UnZip(Cancel, testUnZipFileZip, testUnZipDst)
		require.NoError(t, err)

		testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
	})

	t.Run("directory", func(t *testing.T) {
		testCreateUnZipDirZip(t)
		defer testRemoveUnZipDir(t)

		err := UnZip(Cancel, testUnZipDirZip, testUnZipDst)
		require.NoError(t, err)

		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
	})

	t.Run("multi", func(t *testing.T) {
		testCreateUnZipMultiZip(t)
		defer testRemoveUnZipDir(t)

		err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst)
		require.NoError(t, err)

		testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
	})

	t.Run("select", func(t *testing.T) {
		t.Run("only file", func(t *testing.T) {
			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst, "file1.dat")
			require.NoError(t, err)

			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
			testIsNotExist(t, testUnZipDstDir)
		})

		t.Run("only dir", func(t *testing.T) {
			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst, "dir")
			require.NoError(t, err)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsNotExist(t, testUnZipDstFile)
		})

		t.Run("all", func(t *testing.T) {
			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst, "file1.dat", "dir")
			require.NoError(t, err)

			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		})

		t.Run("repeat", func(t *testing.T) {
			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst, "dir", "file1.dat", "dir")
			require.NoError(t, err)

			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		})

		t.Run("not exist", func(t *testing.T) {
			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst, "not exist")
			require.EqualError(t, err, "\"not exist\" doesn't exist in zip file")
		})
	})
}
