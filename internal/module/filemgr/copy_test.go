package filemgr

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"project/internal/module/task"
	"project/internal/patch/monkey"
	"project/internal/system"
	"project/internal/testsuite"
)

const (
	testCopyDir = "testdata/copy/" // copy test root path

	testCopySrcFile = testCopyDir + "file1.dat" // source path is a file
	testCopySrcDir  = testCopyDir + "dir"       // source path is a directory

	// destination path
	testCopyDst     = testCopyDir + "dst"        // store copied files
	testCopyDstFile = testCopyDst + "/file1.dat" // testdata/copy/dst/file1.dat
	testCopyDstDir  = testCopyDst + "/dir"       // testdata/copy/dst/dir

	// src files in directory
	testCopySrcFile1 = testCopySrcDir + "/afile1.dat"  // testdata/copy/dir/afile1.dat
	testCopySrcDir1  = testCopySrcDir + "/dir1"        // testdata/copy/dir/dir1
	testCopySrcFile2 = testCopySrcDir1 + "/afile2.dat" // testdata/copy/dir/dir1/afile2.dat
	testCopySrcDir2  = testCopySrcDir1 + "/dir2"       // testdata/copy/dir/dir1/dir2
	testCopySrcDir3  = testCopySrcDir + "/dir3"        // testdata/copy/dir/dir3
	testCopySrcDir4  = testCopySrcDir3 + "/dir4"       // testdata/copy/dir/dir3/dir4
	testCopySrcFile3 = testCopySrcDir4 + "/file3.dat"  // testdata/copy/dir/dir3/dir4/file3.dat
	testCopySrcFile4 = testCopySrcDir3 + "/file4.dat"  // testdata/copy/dir/dir3/file4.dat
	testCopySrcFile5 = testCopySrcDir + "/file5.dat"   // testdata/copy/dir/file5.dat
)

func testCreateCopySrcFile(t *testing.T) {
	testCreateFile(t, testCopySrcFile)
}

func testCreateCopySrcDir(t *testing.T) {
	err := os.MkdirAll(testCopySrcDir, 0750)
	require.NoError(t, err)

	testCreateFile(t, testCopySrcFile1)
	err = os.Mkdir(testCopySrcDir1, 0750)
	require.NoError(t, err)
	testCreateFile2(t, testCopySrcFile2)
	err = os.Mkdir(testCopySrcDir2, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testCopySrcDir3, 0750)
	require.NoError(t, err)
	err = os.Mkdir(testCopySrcDir4, 0750)
	require.NoError(t, err)
	testCreateFile(t, testCopySrcFile3)
	testCreateFile2(t, testCopySrcFile4)
	testCreateFile2(t, testCopySrcFile5)
}

func testCreateCopySrcMulti(t *testing.T) {
	testCreateCopySrcFile(t)
	testCreateCopySrcDir(t)
}

func testCheckCopyDstFile(t *testing.T) {
	testCompareFile(t, testCopySrcFile, testCopyDstFile)
}

func testCheckCopyDstDir(t *testing.T) {
	testCompareDirectory(t, testCopySrcDir, testCopyDstDir)
}

func testCheckCopyDstMulti(t *testing.T) {
	testCheckCopyDstFile(t)
	testCheckCopyDstDir(t)
}

func testRemoveCopyDir(t *testing.T) {
	err := os.RemoveAll(testCopyDir)
	require.NoError(t, err)
}

func TestCopy(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("file", func(t *testing.T) {
		testCreateCopySrcFile(t)
		defer testRemoveCopyDir(t)

		err := Copy(Cancel, testCopyDst, testCopySrcFile)
		require.NoError(t, err)

		testCheckCopyDstFile(t)
	})

	t.Run("directory", func(t *testing.T) {
		testCreateCopySrcDir(t)
		defer testRemoveCopyDir(t)

		err := Copy(Cancel, testCopyDst, testCopySrcDir)
		require.NoError(t, err)

		testCheckCopyDstDir(t)
	})

	t.Run("multi", func(t *testing.T) {
		t.Run("file first", func(t *testing.T) {
			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			err := Copy(Cancel, testCopyDst, testCopySrcFile, testCopySrcDir)
			require.NoError(t, err)

			testCheckCopyDstMulti(t)
		})

		t.Run("directory first", func(t *testing.T) {
			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			err := Copy(Cancel, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			testCheckCopyDstMulti(t)
		})
	})
}

func TestCopyWithContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateCopySrcDir(t)
		defer testRemoveCopyDir(t)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := CopyWithContext(ctx, Cancel, testCopyDst, testCopySrcDir)
		require.NoError(t, err)

		testCheckCopyDstDir(t)
	})

	t.Run("cancel", func(t *testing.T) {
		testCreateCopySrcDir(t)
		defer testRemoveCopyDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()
		err := CopyWithContext(ctx, Cancel, testCopyDst, testCopySrcDir)
		require.Equal(t, context.Canceled, err)

		testIsNotExist(t, testCopyDst)
	})
}

func TestCopyWithNotice(t *testing.T) {
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
			err := Copy(ec, testCopyDst, path)
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
			err := Copy(ec, testCopyDst, path)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsNotExist(t, path)
		})
	})

	t.Run("mkdir-stat", func(t *testing.T) {
		target, err := filepath.Abs(testCopyDstDir + "/dir1")
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

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckCopyDstMulti(t)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}

			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsExist(t, testCopyDst)
			testIsNotExist(t, target)
			testCheckCopyDstFile(t)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}

			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testIsExist(t, testCopyDst)
			testIsNotExist(t, target)
			testIsNotExist(t, testCopyDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}

			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.EqualError(t, errors.Cause(err), "unknown failed to copy operation code: 0")

			require.Equal(t, 1, count)

			testIsExist(t, testCopyDst)
			testIsNotExist(t, target)
			testIsNotExist(t, testCopyDstFile)
		})
	})

	t.Run("mkdir-SameDirFile", func(t *testing.T) {
		target, err := filepath.Abs(testCopyDstDir + "/dir1")
		require.NoError(t, err)

		t.Run("retry", func(t *testing.T) {
			// create same name file with directory
			testCreateFile(t, target)

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				err = os.Remove(target)
				require.NoError(t, err)
				return ErrCtrlOpRetry
			}

			err = Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckCopyDstMulti(t)
		})

		t.Run("skip", func(t *testing.T) {
			// create same name file with directory
			testCreateFile(t, target)

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpSkip
			}

			err = Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsExist(t, testCopyDst)
			testIsExist(t, target)
			testIsExist(t, testCopyDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			// create same name file with directory
			testCreateFile(t, target)

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpCancel
			}

			err = Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testIsExist(t, testCopyDst)
			testIsExist(t, target)
			testIsNotExist(t, testCopyDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			// create same name file with directory
			testCreateFile(t, target)

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpInvalid
			}

			err = Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.EqualError(t, errors.Cause(err), "unknown same dir file operation code: 0")

			require.Equal(t, 1, count)

			testIsExist(t, testCopyDst)
			testIsExist(t, target)
			testIsNotExist(t, testCopyDstFile)
		})
	})

	t.Run("mkdir-os.Mkdir", func(t *testing.T) {
		target, err := filepath.Abs(testCopyDstDir + "/dir1")
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

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}

			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckCopyDstMulti(t)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testIsExist(t, testCopyDstDir)
			testIsNotExist(t, target)
			testCheckCopyDstFile(t)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testIsExist(t, testCopyDst)
			testIsNotExist(t, target)
			testIsNotExist(t, testCopyDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.EqualError(t, errors.Cause(err), "unknown failed to copy operation code: 0")

			require.Equal(t, 1, count)

			testIsExist(t, testCopyDst)
			testIsNotExist(t, target)
			testIsNotExist(t, testCopyDstFile)
		})
	})

	t.Run("checkDstFile-stat", func(t *testing.T) {
		target, err := filepath.Abs(testCopyDstFile)
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

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}
			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckCopyDstMulti(t)
		})

		t.Run("skip", func(t *testing.T) {
			defer pg.Restore()

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckCopyDstDir(t)
			testIsNotExist(t, testCopyDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer pg.Restore()

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testCheckCopyDstDir(t)
			testIsNotExist(t, testCopyDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer pg.Restore()

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				monkey.IsMonkeyError(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err := Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.EqualError(t, errors.Cause(err), "unknown failed to copy operation code: 0")

			require.Equal(t, 1, count)

			testCheckCopyDstDir(t)
			testIsNotExist(t, testCopyDstFile)
		})
	})

	t.Run("checkDstFile-SameFileDir", func(t *testing.T) {
		t.Run("retry", func(t *testing.T) {
			// create same name directory with file
			err := os.MkdirAll(testCopyDstFile, 0750)
			require.NoError(t, err)

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				require.NoError(t, err)
				count++
				err = os.Remove(testCopyDstFile)
				require.NoError(t, err)
				return ErrCtrlOpRetry
			}
			err = Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckCopyDstMulti(t)
		})

		t.Run("skip", func(t *testing.T) {
			// create same name directory with file
			err := os.MkdirAll(testCopyDstFile, 0750)
			require.NoError(t, err)

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpSkip
			}
			err = Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			testCheckCopyDstDir(t)
			testIsExist(t, testCopyDstFile)
		})

		t.Run("user cancel", func(t *testing.T) {
			// create same name directory with file
			err := os.MkdirAll(testCopyDstFile, 0750)
			require.NoError(t, err)

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpCancel
			}
			err = Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			testCheckCopyDstDir(t)
			testIsExist(t, testCopyDstFile)
		})

		t.Run("unknown operation", func(t *testing.T) {
			// create same name directory with file
			err := os.MkdirAll(testCopyDstFile, 0750)
			require.NoError(t, err)

			testCreateCopySrcMulti(t)
			defer testRemoveCopyDir(t)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				require.NoError(t, err)
				count++
				return ErrCtrlOpInvalid
			}
			err = Copy(ec, testCopyDst, testCopySrcDir, testCopySrcFile)
			require.EqualError(t, errors.Cause(err), "unknown same file dir operation code: 0")

			require.Equal(t, 1, count)

			testCheckCopyDstDir(t)
			testIsExist(t, testCopyDstFile)
		})
	})

	return

	const (
		srcDir   = "testdata/dir"
		srcFile1 = "file1.dat"
		srcDir2  = "dir2"
		srcFile2 = "dir2/file2.dat"
		dstDir   = "testdata/dir-dir/"
	)

	// create test directory
	err := os.MkdirAll(srcDir, 0750)
	require.NoError(t, err)
	defer func() {
		err = os.RemoveAll(srcDir)
		require.NoError(t, err)
	}()
	// create test file
	testCreateFile(t, filepath.Join(srcDir, srcFile1))
	// create dir2
	err = os.MkdirAll(filepath.Join(srcDir, srcDir2), 0750)
	require.NoError(t, err)
	// create test file 2
	testCreateFile2(t, filepath.Join(srcDir, srcFile2))

	t.Run("FailedToCollect", func(t *testing.T) {
		patch := func(string) (os.FileInfo, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(os.Lstat, patch)
		defer pg.Unpatch()

		t.Run("skip", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCollectFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsNotExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("user cancel", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCollectFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err := Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)
		})

		t.Run("unknown operation", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCollectFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err := Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)
		})
	})

	t.Run("FailedToCopyDir-copyDirFile", func(t *testing.T) {
		dirAbs, err := filepath.Abs(filepath.Join(dstDir, srcDir2))
		require.NoError(t, err)
		fileAbs, err := filepath.Abs(filepath.Join(dstDir, srcFile1))
		require.NoError(t, err)
		patch := func(name string) (os.FileInfo, error) {
			if name == dirAbs || name == fileAbs {
				return nil, monkey.Error
			}
			stat, err := os.Stat(name)
			if err != nil {
				if !os.IsNotExist(err) {
					return nil, err
				}
			}
			return stat, nil
		}
		pg := monkey.Patch(stat, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("skip", func(t *testing.T) {
			pg.Restore()
			defer pg.Unpatch()

			defer func() {
				err = os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				return ErrCtrlOpSkip
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 2, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("user cancel", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err = os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err := Copy(ec, dstDir, srcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)
		})

		t.Run("unknown operation", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err = os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err := Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)
		})
	})

	t.Run("FailedToCopyDir-mkdir-os.Stat", func(t *testing.T) {
		t.Skip()
	})

	t.Run("FailedToCopyDir-mkdir-IsDir", func(t *testing.T) {
		t.Skip()
	})

	t.Run("FailedToCopyDir-mkdir", func(t *testing.T) {
		t.Run("retry", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create root path in dst and patch os.MkdirAll
			err := os.MkdirAll(dstDir, 0750)
			require.NoError(t, err)

			patch := func(string, os.FileMode) error {
				return monkey.Error
			}
			pg := monkey.Patch(os.Mkdir, patch)
			defer pg.Unpatch()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}
			err = Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("skip", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create root path in dst and patch os.MkdirAll
			err := os.MkdirAll(dstDir, 0750)
			require.NoError(t, err)

			patch := func(string, os.FileMode) error {
				return monkey.Error
			}
			pg := monkey.Patch(os.Mkdir, patch)
			defer pg.Unpatch()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err = Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create root path in dst and patch os.MkdirAll
			err := os.MkdirAll(dstDir, 0750)
			require.NoError(t, err)

			patch := func(string, os.FileMode) error {
				return monkey.Error
			}
			pg := monkey.Patch(os.Mkdir, patch)
			defer pg.Unpatch()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err = Copy(ec, dstDir, srcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create root path in dst and patch os.MkdirAll
			err := os.MkdirAll(dstDir, 0750)
			require.NoError(t, err)

			patch := func(string, os.FileMode) error {
				return monkey.Error
			}
			pg := monkey.Patch(os.Mkdir, patch)
			defer pg.Unpatch()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err = Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})
	})

	t.Run("SameDirFile", func(t *testing.T) {
		t.Run("retry", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create same name file with src dir
			path := filepath.Join(dstDir, srcDir2)
			testCreateFile(t, path)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				count++
				err := os.Remove(path)
				require.NoError(t, err)
				return ErrCtrlOpRetry
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("skip", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create same name file with src dir
			path := filepath.Join(dstDir, srcDir2)
			testCreateFile(t, path)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				count++
				return ErrCtrlOpSkip
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create same name file with src dir
			path := filepath.Join(dstDir, srcDir2)
			testCreateFile(t, path)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				count++
				return ErrCtrlOpCancel
			}
			err := Copy(ec, dstDir, srcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create same name file with src dir
			path := filepath.Join(dstDir, srcDir2)
			testCreateFile(t, path)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameDirFile, typ)
				count++
				return ErrCtrlOpInvalid
			}
			err := Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})
	})

	t.Run("FailedToCopy-copyFile-os.Stat", func(t *testing.T) {
		srcAbs, err := filepath.Abs(filepath.Join(srcDir, srcFile1))
		require.NoError(t, err)
		patch := func(name string) (os.FileInfo, error) {
			if name == srcAbs {
				return nil, monkey.Error
			}
			return os.Lstat(name)
		}
		pg := monkey.Patch(os.Stat, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("skip", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("user cancel", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err := Copy(ec, dstDir, srcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("unknown operation", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err = Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})
	})

	t.Run("FailedToCopy-copyFile-IsDir", func(t *testing.T) {
		// create same name file and replace src file to dir and retry
		t.Run("retry", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)

				// recover src file
				src := filepath.Join(srcDir, srcFile1)
				err = os.Remove(src)
				require.NoError(t, err)
				testCreateFile(t, src)
			}()

			// create same name file with src file
			src := filepath.Join(srcDir, srcFile1)
			dst := filepath.Join(dstDir, srcFile1)
			err := os.MkdirAll(dst, 0750)
			require.NoError(t, err)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				if typ == ErrCtrlSameFileDir {
					require.NoError(t, err)

					err := os.Remove(src)
					require.NoError(t, err)
					err = os.Remove(dst)
					require.NoError(t, err)
					err = os.MkdirAll(src, 0750)
					require.NoError(t, err)
					return ErrCtrlOpRetry
				}

				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++

				// recover src file
				src := filepath.Join(srcDir, srcFile1)
				err = os.Remove(src)
				require.NoError(t, err)
				testCreateFile(t, src)
				return ErrCtrlOpRetry
			}
			err = Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("skip", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)

				// recover src file
				src := filepath.Join(srcDir, srcFile1)
				err = os.Remove(src)
				require.NoError(t, err)
				testCreateFile(t, src)
			}()

			// recover src file and create same name file with src file
			src := filepath.Join(srcDir, srcFile1)
			dst := filepath.Join(dstDir, srcFile1)
			err = os.MkdirAll(dst, 0750)
			require.NoError(t, err)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				if typ == ErrCtrlSameFileDir {
					require.NoError(t, err)

					err := os.Remove(src)
					require.NoError(t, err)
					err = os.Remove(dst)
					require.NoError(t, err)
					err = os.MkdirAll(src, 0750)
					require.NoError(t, err)
					return ErrCtrlOpRetry
				}

				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				return ErrCtrlOpSkip
			}
			err = Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)

				// recover src file
				src := filepath.Join(srcDir, srcFile1)
				err = os.Remove(src)
				require.NoError(t, err)
				testCreateFile(t, src)
			}()

			// recover src file and create same name file with src file
			src := filepath.Join(srcDir, srcFile1)
			dst := filepath.Join(dstDir, srcFile1)
			err = os.MkdirAll(dst, 0750)
			require.NoError(t, err)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				if typ == ErrCtrlSameFileDir {
					require.NoError(t, err)

					err := os.Remove(src)
					require.NoError(t, err)
					err = os.Remove(dst)
					require.NoError(t, err)
					err = os.MkdirAll(src, 0750)
					require.NoError(t, err)
					return ErrCtrlOpRetry
				}

				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				return ErrCtrlOpCancel
			}
			err = Copy(ec, dstDir, srcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)

				// recover src file
				src := filepath.Join(srcDir, srcFile1)
				err = os.Remove(src)
				require.NoError(t, err)
				testCreateFile(t, src)
			}()

			// create same name file with src file
			src := filepath.Join(srcDir, srcFile1)
			dst := filepath.Join(dstDir, srcFile1)
			err = os.MkdirAll(dst, 0750)
			require.NoError(t, err)

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				if typ == ErrCtrlSameFileDir {
					require.NoError(t, err)

					err := os.Remove(src)
					require.NoError(t, err)
					err = os.Remove(dst)
					require.NoError(t, err)
					err = os.MkdirAll(src, 0750)
					require.NoError(t, err)
					return ErrCtrlOpRetry
				}

				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				return ErrCtrlOpInvalid
			}
			err = Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})
	})

	t.Run("SameFileDir", func(t *testing.T) {
		t.Run("retry", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			path := filepath.Join(dstDir, srcFile1)
			err := os.MkdirAll(path, 0750)
			require.NoError(t, err)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				count++
				err := os.RemoveAll(path)
				require.NoError(t, err)
				return ErrCtrlOpRetry
			}
			err = Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("skip", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			path := filepath.Join(dstDir, srcFile1)
			err := os.MkdirAll(path, 0750)
			require.NoError(t, err)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				count++
				err := os.RemoveAll(path)
				require.NoError(t, err)
				return ErrCtrlOpSkip
			}
			err = Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			path := filepath.Join(dstDir, srcFile1)
			err := os.MkdirAll(path, 0750)
			require.NoError(t, err)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				count++
				err := os.RemoveAll(path)
				require.NoError(t, err)
				return ErrCtrlOpCancel
			}
			err = Copy(ec, dstDir, srcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			path := filepath.Join(dstDir, srcFile1)
			err := os.MkdirAll(path, 0750)
			require.NoError(t, err)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFileDir, typ)
				count++
				err := os.RemoveAll(path)
				require.NoError(t, err)
				return ErrCtrlOpInvalid
			}
			err = Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})
	})

	t.Run("SameFile", func(t *testing.T) {
		t.Run("replace", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create same name file
			path := filepath.Join(dstDir, srcFile1)
			testCreateFile(t, path)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFile, typ)
				count++
				return ErrCtrlOpReplace
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("skip", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create same name file
			path := filepath.Join(dstDir, srcFile1)
			testCreateFile(t, path)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFile, typ)
				count++
				return ErrCtrlOpSkip
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("user cancel", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create same name file
			path := filepath.Join(dstDir, srcFile1)
			testCreateFile(t, path)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFile, typ)
				count++
				return ErrCtrlOpCancel
			}
			err := Copy(ec, dstDir, srcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("unknown operation", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			// create same name file
			path := filepath.Join(dstDir, srcFile1)
			testCreateFile(t, path)

			count := 0
			ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlSameFile, typ)
				count++
				return ErrCtrlOpInvalid
			}
			err := Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})
	})

	t.Run("FailedToCopy-ioCopy", func(t *testing.T) {
		patch := func(*task.Task, func(int64), io.Writer, io.Reader) (int64, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(ioCopy, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("skip", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err := Copy(ec, dstDir, srcDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("user cancel", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err := Copy(ec, dstDir, srcDir)
			require.Equal(t, ErrUserCanceled, errors.Cause(err))

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})

		t.Run("unknown operation", func(t *testing.T) {
			pg.Restore()

			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err := Copy(ec, dstDir, srcDir)
			require.Error(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})
	})
}

func TestCopyWithRetry(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()
}

func TestCopyTask_Prepare(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()
}

func TestCopyTask_Process(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()
}

func TestCopyTask_Progress(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("common", func(t *testing.T) {
		testCreateCopySrcDir(t)
		defer testRemoveCopyDir(t)

		pg := testPatchTaskCanceled()
		defer pg.Unpatch()

		ct := NewCopyTask(Cancel, nil, testCopyDst, testCopySrcDir)

		done := make(chan struct{})
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				fmt.Println("progress:", ct.Progress())
				fmt.Println("detail:", ct.Detail())
				fmt.Println()
				select {
				case <-done:
					return
				case <-time.After(250 * time.Millisecond):
				}
			}
		}()

		err := ct.Start()
		require.NoError(t, err)

		close(done)
		wg.Wait()

		fmt.Println("progress:", ct.Progress())
		fmt.Println("detail:", ct.Detail())

		rct := ct.Task().(*copyTask)
		testsuite.IsDestroyed(t, ct)
		testsuite.IsDestroyed(t, rct)

		testCheckCopyDstDir(t)
	})

	t.Run("current > total", func(t *testing.T) {
		task := NewCopyTask(Cancel, nil, testCopyDst, testCopySrcDir)
		ct := task.Task().(*copyTask)

		ct.current.SetUint64(1000)
		ct.total.SetUint64(10)

		t.Log(task.Progress())
	})

	t.Run("too long value", func(t *testing.T) {
		task := NewCopyTask(Cancel, nil, testCopyDst, testCopySrcDir)
		ct := task.Task().(*copyTask)

		ct.current.SetUint64(1)
		ct.total.SetUint64(7)

		t.Log(task.Progress())
	})

	t.Run("invalid value", func(t *testing.T) {
		patch := func(s string, bitSize int) (float64, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(strconv.ParseFloat, patch)
		defer pg.Unpatch()

		task := NewCopyTask(Cancel, nil, testCopyDst, testCopySrcDir)
		ct := task.Task().(*copyTask)

		ct.current.SetUint64(1)
		ct.total.SetUint64(7)

		t.Log(task.Progress())
	})

	t.Run("too long progress", func(t *testing.T) {
		task := NewCopyTask(Cancel, nil, testCopyDst, testCopySrcDir)
		ct := task.Task().(*copyTask)

		// 3% -> 2.98%
		ct.current.SetUint64(3)
		ct.total.SetUint64(100)

		t.Log(task.Progress())
	})
}

func TestCopyTask_Watcher(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	pg1 := testPatchTaskCanceled()
	defer pg1.Unpatch()

	pg2 := testPatchMultiTaskWatcher()
	defer pg2.Unpatch()

	testCreateCopySrcDir(t)
	defer testRemoveCopyDir(t)

	err := Copy(Cancel, testCopyDst, testCopySrcDir)
	require.NoError(t, err)

	testCheckCopyDstDir(t)
}
