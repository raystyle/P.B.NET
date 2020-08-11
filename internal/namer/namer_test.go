package namer

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/patch/toml"
	"project/internal/testsuite"
)

func TestLoadWordsFromZipFile(t *testing.T) {
	// create test zip file
	buf := bytes.NewBuffer(make([]byte, 0, 64))
	writer := zip.NewWriter(buf)
	file, err := writer.Create("test.dat")
	require.NoError(t, err)
	_, err = file.Write([]byte("test data"))
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)
	reader := bytes.NewReader(buf.Bytes())
	size := int64(buf.Len())

	t.Run("failed to open file", func(t *testing.T) {
		file := new(zip.File)
		patch := func(*zip.File) (io.ReadCloser, error) {
			return nil, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(file, "Open", patch)
		defer pg.Unpatch()

		zipFile, err := zip.NewReader(reader, size)
		require.NoError(t, err)

		sb, err := loadWordsFromZipFile(zipFile.File[0])
		monkey.IsMonkeyError(t, err)
		require.Nil(t, sb)
	})

	t.Run("failed to read file data", func(t *testing.T) {
		patch := func(reader io.Reader) ([]byte, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(ioutil.ReadAll, patch)
		defer pg.Unpatch()

		zipFile, err := zip.NewReader(reader, size)
		require.NoError(t, err)

		sb, err := loadWordsFromZipFile(zipFile.File[0])
		monkey.IsMonkeyError(t, err)
		require.Nil(t, sb)
	})
}

func TestOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/options.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := Options{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, opts)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: true, actual: opts.DisablePrefix},
		{expected: true, actual: opts.DisableStem},
		{expected: true, actual: opts.DisableSuffix},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
