package filemgr

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func TestCopy(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("src is file", func(t *testing.T) {
		const (
			src     = "testdata/file.dat"
			dstFile = "testdata/file/file.dat"
			dstDir  = "testdata/file/"
		)

		// create test file
		testCreateFile(t, src)
		defer func() {
			err := os.Remove(src)
			require.NoError(t, err)
		}()

		t.Run("to file path", func(t *testing.T) {
			t.Run("dst doesn't exist", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				err := Copy(ReplaceAll, src, dstFile)
				require.NoError(t, err)

				testCompareFile(t, src, dstFile)
			})

			t.Run("dst already exists", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				t.Run("file", func(t *testing.T) {
					// create test file (exists)
					testCreateFile(t, dstFile)
					defer func() {
						err := os.Remove(dstFile)
						require.NoError(t, err)
					}()

					count := 0
					ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
						require.Equal(t, ErrCtrlSameFile, typ)
						count++
						return ErrCtrlOpReplace
					}
					err := Copy(ec, src, dstFile)
					require.NoError(t, err)

					testCompareFile(t, src, dstFile)
					require.Equal(t, 1, count)
				})

				t.Run("directory", func(t *testing.T) {
					// create test directory (exists)
					err := os.MkdirAll(dstFile, 0750)
					require.NoError(t, err)
					defer func() {
						err = os.RemoveAll(dstFile)
						require.NoError(t, err)
					}()

					err = Copy(ReplaceAll, src, dstFile)
					require.NoError(t, err)
				})
			})
		})

		t.Run("to directory path", func(t *testing.T) {
			t.Run("dst doesn't exist", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				err := Copy(ReplaceAll, src, dstDir)
				require.NoError(t, err)

				testCompareFile(t, src, dstFile)
			})

			t.Run("dst already exists", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				t.Run("file", func(t *testing.T) {
					// create test file(exists)
					testCreateFile(t, dstFile)
					defer func() {
						err := os.Remove(dstFile)
						require.NoError(t, err)
					}()

					count := 0
					ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
						require.Equal(t, ErrCtrlSameFile, typ)
						count++
						return ErrCtrlOpReplace
					}
					err := Copy(ec, src, dstDir)
					require.NoError(t, err)

					testCompareFile(t, src, dstFile)
					require.Equal(t, 1, count)
				})

				t.Run("directory", func(t *testing.T) {
					// create test directory (exists)
					err := os.MkdirAll(dstFile, 0750)
					require.NoError(t, err)
					defer func() {
						err = os.RemoveAll(dstFile)
						require.NoError(t, err)
					}()

					count := 0
					ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
						require.Equal(t, ErrCtrlSameFileDir, typ)
						count++
						return ErrCtrlOpSkip
					}
					err = Copy(ec, src, dstDir)
					require.NoError(t, err)

					require.Equal(t, 1, count)
				})
			})
		})
	})

	t.Run("src is directory", func(t *testing.T) {
		const (
			srcDir   = "testdata/dir"
			srcFile1 = "file1.dat"
			srcDir2  = "dir2"
			srcFile2 = "dir2/file2.dat"
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

		t.Run("to directory path", func(t *testing.T) {
			const dstDir = "testdata/dir-dir/"

			t.Run("dst doesn't exist", func(t *testing.T) {
				defer func() {
					err = os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				err = Copy(ReplaceAll, srcDir, dstDir)
				require.NoError(t, err)

				testCompareDirectory(t, srcDir, dstDir)
			})

			t.Run("dst already exists", func(t *testing.T) {
				err := os.MkdirAll(dstDir, 0750)
				require.NoError(t, err)
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				err = Copy(ReplaceAll, srcDir, dstDir)
				require.NoError(t, err)

				testCompareDirectory(t, srcDir, dstDir)
			})
		})

		t.Run("to file path", func(t *testing.T) {
			const dstDir = "testdata/dir-dir"

			t.Run("dst doesn't exist", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				err := Copy(ReplaceAll, srcDir, dstDir)
				require.NoError(t, err)

				testCompareDirectory(t, srcDir, dstDir)
			})

			t.Run("dst already exists", func(t *testing.T) {
				t.Run("file", func(t *testing.T) {
					defer func() {
						err := os.RemoveAll(dstDir)
						require.NoError(t, err)
					}()

					// create test file(exists)
					testCreateFile(t, dstDir)
					defer func() {
						err := os.Remove(dstDir)
						require.NoError(t, err)
					}()

					err := Copy(ReplaceAll, srcDir, dstDir)
					require.Error(t, err)
				})

				t.Run("directory", func(t *testing.T) {
					err := os.MkdirAll(dstDir, 0750)
					require.NoError(t, err)
					defer func() {
						err := os.RemoveAll(dstDir)
						require.NoError(t, err)
					}()

					err = Copy(ReplaceAll, srcDir, dstDir)
					require.NoError(t, err)

					testCompareDirectory(t, srcDir, dstDir)
				})
			})
		})

		t.Run("sub file exist in dst directory", func(t *testing.T) {
			const dstDir = "testdata/dir-dir/"

			t.Run("file to directory", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				// create exists same name directory
				path := filepath.Join(dstDir, srcFile1)
				err := os.MkdirAll(path, 0750)
				require.NoError(t, err)
				defer func() {
					err := os.Remove(path)
					require.NoError(t, err)
				}()

				count := 0
				ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
					require.Equal(t, ErrCtrlSameFileDir, typ)
					count++
					return ErrCtrlOpSkip
				}
				err = Copy(ec, srcDir, dstDir)
				require.NoError(t, err)

				require.Equal(t, 1, count)
			})

			// C:\test\dir2[dir] exists in D:\test\dir2[file]
			// need skip all files in C:\test\dir2
			// like skip C:\test\dir2\file2.dat
			t.Run("directory to file", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				// create exists same name file
				path := filepath.Join(dstDir, srcDir2)
				testCreateFile(t, path)
				defer func() {
					err := os.Remove(path)
					require.NoError(t, err)
				}()

				count := 0
				ec := func(_ context.Context, typ uint8, _ error, _ *SrcDstStat) uint8 {
					require.Equal(t, ErrCtrlSameDirFile, typ)
					count++
					return ErrCtrlOpSkip
				}
				err := Copy(ec, srcDir, dstDir)
				require.NoError(t, err)

				require.Equal(t, 1, count)
			})
		})
	})
}

func TestCopyWithContext(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("copy file", func(t *testing.T) {
		const (
			src = "testdata/file.dat"
			dst = "testdata/file/file.dat"
			dir = "testdata/file"
		)

		// create test file
		testCreateFile(t, src)
		defer func() {
			err := os.Remove(src)
			require.NoError(t, err)
		}()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := CopyWithContext(ctx, ReplaceAll, src, dst)
		require.NoError(t, err)

		exist, err := system.IsExist(dir)
		require.NoError(t, err)
		require.True(t, exist)

		err = os.RemoveAll(dir)
		require.NoError(t, err)
	})

	t.Run("copy directory", func(t *testing.T) {
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

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err = CopyWithContext(ctx, SkipAll, srcDir, dstDir)
		require.NoError(t, err)

		exist, err := system.IsExist(dstDir)
		require.NoError(t, err)
		require.True(t, exist)

		err = os.RemoveAll(dstDir)
		require.NoError(t, err)
	})

	t.Run("cancel", func(t *testing.T) {
		const (
			src = "testdata/file.dat"
			dst = "testdata/file/file.dat"
			dir = "testdata/file"
		)

		// create test file
		testCreateFile(t, src)
		defer func() {
			err := os.Remove(src)
			require.NoError(t, err)
		}()

		// use errCtrl to call cancel
		testCreateFile(t, dst)
		defer func() { _ = os.Remove(dst) }()

		ctx, cancel := context.WithCancel(context.Background())
		ec := func(_ context.Context, _ uint8, _ error, _ *SrcDstStat) uint8 {
			cancel()
			// wait close chan
			time.Sleep(time.Second)
			return ErrCtrlOpReplace
		}
		err := CopyWithContext(ctx, ec, src, dst)
		require.Equal(t, context.Canceled, errors.Cause(err))

		exist, err := system.IsExist(dir)
		require.NoError(t, err)
		require.True(t, exist)

		err = os.RemoveAll(dir)
		require.NoError(t, err)
	})
}

func TestCopyWithNotice(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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

		t.Run("retry", func(t *testing.T) {
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
				return ErrCtrlOpRetry
			}
			err := Copy(ec, srcDir, dstDir)
			require.Error(t, err)

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
				require.Equal(t, ErrCtrlCollectFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
				require.Equal(t, ErrCtrlCopyDirFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}
			err := Copy(ec, srcDir, dstDir)
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
				require.Equal(t, ErrCtrlCopyDirFailed, typ)
				require.Error(t, err)
				count++
				return ErrCtrlOpSkip
			}
			err := Copy(ec, srcDir, dstDir)
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
				require.Equal(t, ErrCtrlCopyDirFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err := Copy(ec, srcDir, dstDir)
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
				require.Equal(t, ErrCtrlCopyDirFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err := Copy(ec, srcDir, dstDir)
			require.Error(t, err)

			require.Equal(t, 1, count)
		})
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
				require.Equal(t, ErrCtrlCopyDirFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}
			err = Copy(ec, srcDir, dstDir)
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
				require.Equal(t, ErrCtrlCopyDirFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}
			err = Copy(ec, srcDir, dstDir)
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
				require.Equal(t, ErrCtrlCopyDirFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}
			err = Copy(ec, srcDir, dstDir)
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
				require.Equal(t, ErrCtrlCopyDirFailed, typ)
				require.Error(t, err)
				count++
				pg.Unpatch()
				return ErrCtrlOpInvalid
			}
			err = Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err = Copy(ec, srcDir, dstDir)
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
			err = Copy(ec, srcDir, dstDir)
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
			err = Copy(ec, srcDir, dstDir)
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
			err = Copy(ec, srcDir, dstDir)
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
			err = Copy(ec, srcDir, dstDir)
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
			err = Copy(ec, srcDir, dstDir)
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
			err = Copy(ec, srcDir, dstDir)
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
			err = Copy(ec, srcDir, dstDir)
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
			err = Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
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
			err := Copy(ec, srcDir, dstDir)
			require.Error(t, err)

			require.Equal(t, 1, count)

			exist, err := system.IsExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})
	})
}

func TestCopyTask_Progress(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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

	t.Run("common", func(t *testing.T) {
		defer func() {
			err = os.RemoveAll(dstDir)
			require.NoError(t, err)
		}()

		// create exist file(file2 will copy first, than copy file1)
		err := os.MkdirAll(dstDir, 0750)
		require.NoError(t, err)
		testCreateFile2(t, filepath.Join(dstDir, srcFile1))

		ec := func(_ context.Context, _ uint8, _ error, stats *SrcDstStat) uint8 {
			time.Sleep(2 * time.Second)
			return ErrCtrlOpReplace
		}
		ct := NewCopyTask(ec, srcDir, dstDir, nil)

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
				fmt.Println("progress:", ct.Progress())
				fmt.Println("detail:", ct.Detail())
				fmt.Println()
				time.Sleep(250 * time.Millisecond)
			}
		}()

		err = ct.Start()
		require.NoError(t, err)

		close(done)
		wg.Wait()

		fmt.Println("progress:", ct.Progress())
		fmt.Println("detail:", ct.Detail())

		rct := ct.Task()
		testsuite.IsDestroyed(t, ct)
		testsuite.IsDestroyed(t, rct)

		exist, err := system.IsExist(dstDir)
		require.NoError(t, err)
		require.True(t, exist)
	})
}
