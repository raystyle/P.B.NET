package filemgr

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/system"
	"project/internal/xio"
)

func TestCopy(t *testing.T) {
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
					err := Copy(func(typ uint8, err error, src, dst string) uint8 {
						require.Equal(t, ErrCtrlSameFile, typ)
						count++
						return ErrCtrlOpReplace
					}, src, dstFile)
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
					err := Copy(func(typ uint8, err error, src, dst string) uint8 {
						require.Equal(t, ErrCtrlSameFile, typ)
						count++
						return ErrCtrlOpReplace
					}, src, dstDir)
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
					err = Copy(func(typ uint8, err error, src, dst string) uint8 {
						require.Equal(t, ErrCtrlSameFileDir, typ)
						count++
						return ErrCtrlOpSkip
					}, src, dstDir)
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
				err = Copy(func(typ uint8, err error, src string, dst string) uint8 {
					require.Equal(t, ErrCtrlSameFileDir, typ)
					count++
					return ErrCtrlOpSkip
				}, srcDir, dstDir)
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
				err := Copy(func(typ uint8, err error, src string, dst string) uint8 {
					require.Equal(t, ErrCtrlSameDirFile, typ)
					count++
					return ErrCtrlOpSkip
				}, srcDir, dstDir)
				require.NoError(t, err)

				require.Equal(t, 1, count)
			})
		})
	})

	t.Run("with context", func(t *testing.T) {
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
			cancel()
			err := CopyWithContext(ctx, SkipAll, src, dst)
			require.Equal(t, context.Canceled, err)

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
			cancel()
			err = CopyWithContext(ctx, SkipAll, srcDir, dstDir)
			require.Equal(t, context.Canceled, err)

			exist, err := system.IsNotExist(dstDir)
			require.NoError(t, err)
			require.True(t, exist)
		})
	})

	t.Run("noticeFailedToCopy", func(t *testing.T) {
		const (
			src = "testdata/file.dat"
			dst = "testdata/file/file.dat"
			dir = "testdata/file"
		)

		// create test file
		testCreateFile(t, src)
		defer func() {
			err := os.RemoveAll(src)
			require.NoError(t, err)
		}()

		t.Run("retry", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dir)
				require.NoError(t, err)
			}()

			patch := func(context.Context, io.Writer, io.Reader) (int64, error) {
				return 0, monkey.Error
			}
			pg := monkey.Patch(xio.CopyWithContext, patch)
			defer pg.Unpatch()

			count := 0
			err := Copy(func(typ uint8, err error, src string, dst string) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				count++
				pg.Unpatch()
				return ErrCtrlOpRetry
			}, src, dst)
			require.NoError(t, err)

			require.Equal(t, 1, count)
		})

		t.Run("skip", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dir)
				require.NoError(t, err)
			}()

			patch := func(context.Context, io.Writer, io.Reader) (int64, error) {
				return 0, monkey.Error
			}
			pg := monkey.Patch(xio.CopyWithContext, patch)
			defer pg.Unpatch()

			count := 0
			err := Copy(func(typ uint8, err error, src string, dst string) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				count++
				pg.Unpatch()
				return ErrCtrlOpSkip
			}, src, dst)
			require.NoError(t, err)

			require.Equal(t, 1, count)
		})

		t.Run("user canceled", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dir)
				require.NoError(t, err)
			}()

			patch := func(context.Context, io.Writer, io.Reader) (int64, error) {
				return 0, monkey.Error
			}
			pg := monkey.Patch(xio.CopyWithContext, patch)
			defer pg.Unpatch()

			count := 0
			err := Copy(func(typ uint8, err error, src string, dst string) uint8 {
				require.Equal(t, ErrCtrlCopyFailed, typ)
				count++
				pg.Unpatch()
				return ErrCtrlOpCancel
			}, src, dst)
			require.Equal(t, ErrUserCanceled, err)

			require.Equal(t, 1, count)
		})
	})
}
