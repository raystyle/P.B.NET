package filemgr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/system"
	"project/internal/testsuite"
)

const (
	testUnZipDir = "testdata/unzip/" // unzip test root path

	testUnZipFileZip  = testUnZipDir + "unzip_file.zip"  // source zip file include a file
	testUnZipDirZip   = testUnZipDir + "unzip_dir.zip"   // source zip file include a directory
	testUnZipMultiZip = testUnZipDir + "unzip_multi.zip" // source zip file include a file and directory

	// destination path
	testUnZipDst     = testUnZipDir + "dst"        // store extracted file
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

		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
	})
}

func TestUnZipWithContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateUnZipMultiZip(t)
		defer testRemoveUnZipDir(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := UnZipWithContext(ctx, Cancel, testUnZipMultiZip, testUnZipDst)
		require.NoError(t, err)

		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
	})

	t.Run("cancel", func(t *testing.T) {
		testCreateUnZipMultiZip(t)
		defer testRemoveUnZipDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()
		err := UnZipWithContext(ctx, Cancel, testUnZipMultiZip, testUnZipDst)
		require.Equal(t, context.Canceled, err)

		testIsNotExist(t, testUnZipDstDir)
		testIsNotExist(t, testUnZipDstFile)
	})
}

func TestUnZipWithNotice(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("mkdir-stat", func(t *testing.T) {
		target, err := filepath.Abs(testUnZipDstDir + "/dir1")
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

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsExist(t, testUnZipDstDir)
			testIsNotExist(t, target)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testIsExist(t, testUnZipDstDir)
			testIsNotExist(t, target)
			testIsNotExist(t, testUnZipDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.EqualError(t, err, "unknown failed to unzip operation code: 0")

			require.Equal(t, 1, count)

			testIsExist(t, testUnZipDstDir)
			testIsNotExist(t, target)
			testIsNotExist(t, testUnZipDstFile)
		})
	})

	t.Run("mkdir-SameDirFile", func(t *testing.T) {
		t.Run("retry", func(t *testing.T) {
			// create same name file with directory
			target, err := filepath.Abs(testUnZipDstDir + "/dir1")
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				err = os.Remove(target)
				require.NoError(t, err)
				return ErrCtrlOpRetry
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("skip", func(t *testing.T) {
			// create same name file with directory
			target, err := filepath.Abs(testUnZipDstDir + "/dir1")
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpSkip
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsExist(t, testUnZipDstDir)
			testIsExist(t, target)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			// create same name file with directory
			target, err := filepath.Abs(testUnZipDstDir + "/dir1")
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpCancel
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testIsExist(t, testUnZipDstDir)
			testIsExist(t, target)
			testIsNotExist(t, testUnZipDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			// create same name file with directory
			target, err := filepath.Abs(testUnZipDstDir + "/dir1")
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpInvalid
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.EqualError(t, err, "unknown same dir file operation code: 0")

			require.Equal(t, 1, count)

			testIsExist(t, testUnZipDstDir)
			testIsExist(t, target)
			testIsNotExist(t, testUnZipDstFile)
		})
	})

	t.Run("mkdir-os.MkdirAll", func(t *testing.T) {
		target, err := filepath.Abs(testUnZipDstDir + "/dir1")
		require.NoError(t, err)

		var pg *monkey.PatchGuard
		patch := func(name string, perm os.FileMode) error {
			if name == target {
				return monkey.Error
			}
			pg.Unpatch()
			defer pg.Restore()
			return os.MkdirAll(name, perm)
		}
		pg = monkey.Patch(os.MkdirAll, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsExist(t, testUnZipDstDir)
			testIsNotExist(t, target)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testIsExist(t, testUnZipDstDir)
			testIsNotExist(t, target)
			testIsNotExist(t, testUnZipDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.EqualError(t, err, "unknown failed to unzip operation code: 0")

			require.Equal(t, 1, count)

			testIsExist(t, testUnZipDstDir)
			testIsNotExist(t, target)
			testIsNotExist(t, testUnZipDstFile)
		})
	})

	t.Run("checkDst-stat", func(t *testing.T) {
		target, err := filepath.Abs(testUnZipDstFile)
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

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsNotExist(t, testUnZipDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsNotExist(t, testUnZipDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.EqualError(t, err, "unknown failed to unzip operation code: 0")

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsNotExist(t, testUnZipDstFile)
		})
	})

	t.Run("checkDst-SameFileDir", func(t *testing.T) {
		t.Run("retry", func(t *testing.T) {
			// create same name directory with file
			target, err := filepath.Abs(testUnZipDstFile)
			require.NoError(t, err)
			err = os.MkdirAll(target, 0750)
			require.NoError(t, err)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				require.NoError(t, err)
				count++
				err = os.Remove(target)
				require.NoError(t, err)
				return ErrCtrlOpRetry
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("skip", func(t *testing.T) {
			// create same name directory with file
			target, err := filepath.Abs(testUnZipDstFile)
			require.NoError(t, err)
			err = os.MkdirAll(target, 0750)
			require.NoError(t, err)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpSkip
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsExist(t, testUnZipDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			// create same name directory with file
			target, err := filepath.Abs(testUnZipDstFile)
			require.NoError(t, err)
			err = os.MkdirAll(target, 0750)
			require.NoError(t, err)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpCancel
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsExist(t, testUnZipDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			// create same name directory with file
			target, err := filepath.Abs(testUnZipDstFile)
			require.NoError(t, err)
			err = os.MkdirAll(target, 0750)
			require.NoError(t, err)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpInvalid
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.EqualError(t, err, "unknown same file dir operation code: 0")

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsExist(t, testUnZipDstFile)
		})
	})

	t.Run("checkDst-SameFile", func(t *testing.T) {
		t.Run("replace", func(t *testing.T) {
			// create same name file
			target, err := filepath.Abs(testUnZipDstFile)
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFile, typ)
				require.NoError(t, err)
				count++
				err = os.Remove(target)
				require.NoError(t, err)
				return ErrCtrlOpReplace
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("skip", func(t *testing.T) {
			// create same name file
			target, err := filepath.Abs(testUnZipDstFile)
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpSkip
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsExist(t, testUnZipDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			// create same name file
			target, err := filepath.Abs(testUnZipDstFile)
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpCancel
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsExist(t, testUnZipDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			// create same name file
			target, err := filepath.Abs(testUnZipDstFile)
			require.NoError(t, err)
			testCreateFile(t, target)

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpInvalid
			}

			err = UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.EqualError(t, err, "unknown same file operation code: 0")

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsExist(t, testUnZipDstFile)
		})
	})

	t.Run("extractFile-system.OpenFile", func(t *testing.T) {
		target, err := filepath.Abs(testUnZipDstFile)
		require.NoError(t, err)

		var pg *monkey.PatchGuard
		patch := func(name string, flag int, perm os.FileMode) (*os.File, error) {
			if name == target {
				return nil, monkey.Error
			}
			pg.Unpatch()
			defer pg.Restore()
			return system.OpenFile(name, flag, perm)
		}
		pg = monkey.Patch(system.OpenFile, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsNotExist(t, testUnZipDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsNotExist(t, testUnZipDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlUnZipFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}

			err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
			require.EqualError(t, err, "unknown failed to unzip operation code: 0")

			require.Equal(t, 1, count)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsNotExist(t, testUnZipDstFile)
		})
	})
}

func TestUnZipWithRetry(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	target, err := filepath.Abs(testUnZipDstFile)
	require.NoError(t, err)

	var pg *monkey.PatchGuard
	patch := func(name string, aTime, mTime time.Time) error {
		if name == target {
			return monkey.Error
		}
		pg.Unpatch()
		defer pg.Restore()
		return os.Chtimes(name, aTime, mTime)
	}
	pg = monkey.Patch(os.Chtimes, patch)
	defer pg.Unpatch()

	t.Run("retry", func(t *testing.T) {
		defer pg.Restore()

		testCreateUnZipMultiZip(t)
		defer testRemoveUnZipDir(t)

		count := 0
		ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
			require.Equal(t, ErrCtrlUnZipFailed, typ)
			monkey.IsMonkeyError(t, err)
			count++
			pg.Unpatch()
			return ErrCtrlOpRetry
		}

		err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
		require.NoError(t, err)

		require.Equal(t, 1, count)

		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
	})

	t.Run("skip", func(t *testing.T) {
		defer pg.Restore()

		testCreateUnZipMultiZip(t)
		defer testRemoveUnZipDir(t)

		count := 0
		ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
			require.Equal(t, ErrCtrlUnZipFailed, typ)
			monkey.IsMonkeyError(t, err)
			count++
			pg.Unpatch()
			return ErrCtrlOpSkip
		}

		err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
		require.NoError(t, err)

		require.Equal(t, 1, count)

		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		testIsNotExist(t, testUnZipDstFile)
	})

	t.Run("user cancel", func(t *testing.T) {
		defer pg.Restore()

		testCreateUnZipMultiZip(t)
		defer testRemoveUnZipDir(t)

		count := 0
		ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
			require.Equal(t, ErrCtrlUnZipFailed, typ)
			monkey.IsMonkeyError(t, err)
			count++
			pg.Unpatch()
			return ErrCtrlOpCancel
		}

		err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
		require.Equal(t, ErrUserCanceled, errors.Cause(err))

		require.Equal(t, 1, count)

		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		testIsNotExist(t, testUnZipDstFile)
	})

	t.Run("unknown operation", func(t *testing.T) {
		defer pg.Restore()

		testCreateUnZipMultiZip(t)
		defer testRemoveUnZipDir(t)

		count := 0
		ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
			require.Equal(t, ErrCtrlUnZipFailed, typ)
			monkey.IsMonkeyError(t, err)
			count++
			pg.Unpatch()
			return ErrCtrlOpInvalid
		}

		err := UnZip(ec, testUnZipMultiZip, testUnZipDst)
		require.EqualError(t, err, "unknown failed to unzip operation code: 0")

		require.Equal(t, 1, count)

		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		testIsNotExist(t, testUnZipDstFile)
	})
}

func TestUnZipTask_Prepare(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()
}

func TestUnZipTask_Process(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("select", func(t *testing.T) {
		t.Run("only file", func(t *testing.T) {
			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst, "file1.dat")
			require.NoError(t, err)

			testIsNotExist(t, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("only directory", func(t *testing.T) {
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

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("repeat directory", func(t *testing.T) {
			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst, "dir", "file1.dat", "dir")
			require.NoError(t, err)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
		})

		t.Run("repeat file in directory", func(t *testing.T) {
			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst, "dir", "dir/afile1.dat")
			require.NoError(t, err)

			testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
			testIsNotExist(t, testUnZipDstFile)
		})

		t.Run("not exist", func(t *testing.T) {
			testCreateUnZipMultiZip(t)
			defer testRemoveUnZipDir(t)

			err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst, "not exist")
			require.EqualError(t, err, "\"not exist\" doesn't exist in zip file")

			testIsNotExist(t, testUnZipDstDir)
			testIsNotExist(t, testUnZipDstFile)
		})
	})

	t.Run("destination directory already exists", func(t *testing.T) {
		testCreateUnZipMultiZip(t)
		defer testRemoveUnZipDir(t)

		err := os.MkdirAll(testUnZipDstDir, 0750)
		require.NoError(t, err)

		err = UnZip(Cancel, testUnZipMultiZip, testUnZipDst)
		require.NoError(t, err)

		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
	})
}

func TestUnZipTask_Progress(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateUnZipMultiZip(t)
		defer testRemoveUnZipDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		ut := NewUnZipTask(Cancel, nil, testUnZipMultiZip, testUnZipDst)

		done := make(chan struct{})
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				fmt.Println("progress:", ut.Progress())
				fmt.Println("detail:", ut.Detail())
				fmt.Println()
				select {
				case <-done:
					return
				case <-time.After(250 * time.Millisecond):
				}
			}
		}()

		err := ut.Start()
		require.NoError(t, err)

		close(done)
		wg.Wait()

		fmt.Println("progress:", ut.Progress())
		fmt.Println("detail:", ut.Detail())

		rut := ut.Task().(*unZipTask)
		testsuite.IsDestroyed(t, ut)
		testsuite.IsDestroyed(t, rut)

		testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
		testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
	})

	t.Run("current > total", func(t *testing.T) {
		task := NewUnZipTask(Cancel, nil, testUnZipMultiZip, testUnZipDst)
		ut := task.Task().(*unZipTask)

		ut.current.SetUint64(1000)
		ut.total.SetUint64(10)

		t.Log(task.Progress())
	})

	t.Run("too long value", func(t *testing.T) {
		task := NewUnZipTask(Cancel, nil, testUnZipMultiZip, testUnZipDst)
		ut := task.Task().(*unZipTask)

		ut.current.SetUint64(1)
		ut.total.SetUint64(7)

		t.Log(task.Progress())
	})

	t.Run("invalid value", func(t *testing.T) {
		patch := func(s string, bitSize int) (float64, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(strconv.ParseFloat, patch)
		defer pg.Unpatch()

		task := NewUnZipTask(Cancel, nil, testUnZipMultiZip, testUnZipDst)
		ut := task.Task().(*unZipTask)

		ut.current.SetUint64(1)
		ut.total.SetUint64(7)

		t.Log(task.Progress())
	})

	t.Run("too long progress", func(t *testing.T) {
		task := NewUnZipTask(Cancel, nil, testUnZipMultiZip, testUnZipDst)
		ut := task.Task().(*unZipTask)

		// 3% -> 2.98%
		ut.current.SetUint64(3)
		ut.total.SetUint64(100)

		t.Log(task.Progress())
	})
}

func TestUnZipTask_Watcher(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	testCreateUnZipMultiZip(t)
	defer testRemoveUnZipDir(t)

	pg1 := testPatchTaskCanceled()
	defer pg1.Unpatch()

	pg2 := testPatchMultiTaskWatcher()
	defer pg2.Unpatch()

	err := UnZip(Cancel, testUnZipMultiZip, testUnZipDst)
	require.NoError(t, err)

	testCompareDirectory(t, testUnZipSrcDir, testUnZipDstDir)
	testCompareFile(t, testUnZipSrcFile, testUnZipDstFile)
}
