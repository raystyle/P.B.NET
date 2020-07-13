package filemgr

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
			t.Run("destination doesn't exist", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				err := Copy(ReplaceAll, src, dstFile)
				require.NoError(t, err)

				testCompareFile(t, src, dstFile)
			})

			t.Run("destination exists", func(t *testing.T) {
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
			t.Run("destination doesn't exist", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				err := Copy(ReplaceAll, src, dstDir)
				require.NoError(t, err)

				testCompareFile(t, src, dstFile)
			})

			t.Run("destination exists", func(t *testing.T) {
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

			t.Run("destination doesn't exist", func(t *testing.T) {
				defer func() {
					err = os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				err = Copy(ReplaceAll, srcDir, dstDir)
				require.NoError(t, err)

				testCompareDirectory(t, srcDir, dstDir)
			})

			t.Run("destination exists", func(t *testing.T) {
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

			t.Run("destination doesn't exist", func(t *testing.T) {
				defer func() {
					err := os.RemoveAll(dstDir)
					require.NoError(t, err)
				}()

				err := Copy(ReplaceAll, srcDir, dstDir)
				require.NoError(t, err)

				testCompareDirectory(t, srcDir, dstDir)
			})

			t.Run("destination exists", func(t *testing.T) {
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
				err = os.RemoveAll(dstDir)
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
				err = os.RemoveAll(dstDir)
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
				err = os.RemoveAll(dstDir)
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
		patch := func(name string) (os.FileInfo, error) {
			abs, err := filepath.Abs(filepath.Join(dstDir, srcDir2))
			if err != nil {
				return nil, err
			}
			if name == abs {
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
				err = os.RemoveAll(dstDir)
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
			t.Skip()

			defer func() {
				err = os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			count := 0
			ec := func(_ context.Context, typ uint8, err error, _ *SrcDstStat) uint8 {
				require.Equal(t, ErrCtrlCopyDirFailed, typ)
				require.Error(t, err)
				count++
				// pg.Unpatch()
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

		})

		t.Run("user cancel", func(t *testing.T) {

		})
	})

	t.Run("FailedToCopy-copyFile-os.Stat", func(t *testing.T) {

	})

	t.Run("FailedToCopy-copyFile-IsDir", func(t *testing.T) {

	})

	t.Run("FailedToCopy-ioCopy", func(t *testing.T) {
		patch := func(*task.Task, func(int64), io.Writer, io.Reader) (int64, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(ioCopy, patch)
		defer pg.Unpatch()

		t.Run("retry", func(t *testing.T) {
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
				err = os.RemoveAll(dstDir)
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
			err = Copy(ec, srcDir, dstDir)
			require.NoError(t, err)

			require.Equal(t, 1, count)

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
				require.Equal(t, ErrCtrlCopyFailed, typ)
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
}

func TestCopyTask_Progress(t *testing.T) {
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
			time.Sleep(time.Second)
			return ErrCtrlOpReplace
		}
		ct := NewCopyTask(ec, srcDir, dstDir, nil)

		done := make(chan struct{})
		go func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				fmt.Println("detail:", ct.Detail())
				fmt.Println("progress:", ct.Progress())
				time.Sleep(250 * time.Millisecond)
			}
		}()

		err = ct.Start()
		require.NoError(t, err)

		close(done)

		rct := ct.Task()
		testsuite.IsDestroyed(t, ct)
		testsuite.IsDestroyed(t, rct)

		exist, err := system.IsExist(dstDir)
		require.NoError(t, err)
		require.True(t, exist)
	})
}
