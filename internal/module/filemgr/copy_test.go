package filemgr

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func testCreateFile(t *testing.T, name string) {
	data := testsuite.Bytes()
	err := ioutil.WriteFile(name, data, 0600)
	require.NoError(t, err)
}

func testCompareFile(t *testing.T, a, b string) {
	aFile, err := os.Open(a)
	require.NoError(t, err)
	defer func() { _ = aFile.Close() }()
	bFile, err := os.Open(b)
	require.NoError(t, err)
	defer func() { _ = bFile.Close() }()

	// compare data
	aFileData, err := ioutil.ReadAll(aFile)
	require.NoError(t, err)
	bFileData, err := ioutil.ReadAll(bFile)
	require.NoError(t, err)
	require.Equal(t, aFileData, bFileData)

	// compare
	aStat, err := aFile.Stat()
	require.NoError(t, err)
	bStat, err := bFile.Stat()
	require.NoError(t, err)

	require.Equal(t, aStat.Size(), bStat.Size())
	require.Equal(t, aStat.Mode(), bStat.Mode())
	require.Equal(t, aStat.IsDir(), bStat.IsDir())

	// mod time is not equal about wall
	const format = "2006-01-02 15:04:05"
	am := aStat.ModTime().Format(format)
	bm := bStat.ModTime().Format(format)
	require.Equal(t, am, bm)
}

func TestCopy(t *testing.T) {
	t.Run("src is file", func(t *testing.T) {
		const (
			srcFile = "testdata/fef.dat"
			dstDir  = "testdata/fef/"
			dstFile = "testdata/fef/fef.dat"
		)

		// create test file
		testCreateFile(t, srcFile)
		defer func() {
			err := os.Remove(srcFile)
			require.NoError(t, err)
		}()

		t.Run("to exist file path", func(t *testing.T) {
			// create destination directory
			err := os.MkdirAll(dstDir, 0750)
			require.NoError(t, err)
			defer func() {
				err = os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			t.Run("destination doesn't exist", func(t *testing.T) {
				err = Copy(ReplaceAll, srcFile, dstFile)
				require.NoError(t, err)

				testCompareFile(t, srcFile, dstFile)
			})

			t.Run("destination exists", func(t *testing.T) {
				t.Run("file", func(t *testing.T) {
					// create test file (exists)
					testCreateFile(t, dstFile)
					defer func() {
						err = os.Remove(dstFile)
						require.NoError(t, err)
					}()

					count := 0
					err = Copy(func(typ uint8, src, dst string) uint8 {
						require.Equal(t, SameFile, typ)
						count++
						return SameCtrlReplace
					}, srcFile, dstFile)
					require.NoError(t, err)

					testCompareFile(t, srcFile, dstFile)
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
					err = Copy(func(typ uint8, src, dst string) uint8 {
						require.Equal(t, SameFileDir, typ)
						count++
						return SameCtrlSkip
					}, srcFile, dstFile)
					require.NoError(t, err)
				})
			})
		})

		t.Run("to exist directory", func(t *testing.T) {
			// create destination directory
			err := os.MkdirAll(dstDir, 0750)
			require.NoError(t, err)
			defer func() {
				err = os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			t.Run("destination doesn't exist", func(t *testing.T) {
				err = Copy(ReplaceAll, srcFile, dstDir)
				require.NoError(t, err)

				testCompareFile(t, srcFile, dstFile)
			})

			t.Run("destination exists", func(t *testing.T) {
				t.Run("file", func(t *testing.T) {
					// create test file(exists)
					testCreateFile(t, dstFile)
					defer func() {
						err = os.Remove(dstFile)
						require.NoError(t, err)
					}()

					count := 0
					err = Copy(func(typ uint8, src, dst string) uint8 {
						require.Equal(t, SameFile, typ)
						count++
						return SameCtrlReplace
					}, srcFile, dstDir)
					require.NoError(t, err)

					testCompareFile(t, srcFile, dstFile)
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
					err = Copy(func(typ uint8, src, dst string) uint8 {
						require.Equal(t, SameFileDir, typ)
						count++
						return SameCtrlSkip
					}, srcFile, dstDir)
					require.NoError(t, err)
				})
			})
		})

		t.Run("to doesn't exist file path", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			err := Copy(ReplaceAll, srcFile, dstFile)
			require.NoError(t, err)

			testCompareFile(t, srcFile, dstFile)
		})

		t.Run("to doesn't exist directory", func(t *testing.T) {
			defer func() {
				err := os.RemoveAll(dstDir)
				require.NoError(t, err)
			}()

			err := Copy(ReplaceAll, srcFile, dstDir)
			require.NoError(t, err)

			testCompareFile(t, srcFile, dstFile)
		})
	})

	t.Run("src is directory", func(t *testing.T) {
		t.Run("to exist directory", func(t *testing.T) {

		})

		t.Run("to doesn't exist directory", func(t *testing.T) {

		})

		t.Run("to file path", func(t *testing.T) {

		})
	})
}
