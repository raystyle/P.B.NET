package namer

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/testsuite"
)

func testGenerateEnglishResource(t *testing.T) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	writer := zip.NewWriter(buf)
	for _, name := range []string{
		"prefix.txt",
		"stem.txt",
		"suffix.txt",
	} {
		file, err := os.Open("testdata/english/" + name)
		require.NoError(t, err)
		stat, err := file.Stat()
		require.NoError(t, err)

		fileHeader, err := zip.FileInfoHeader(stat)
		require.NoError(t, err)
		w, err := writer.CreateHeader(fileHeader)
		require.NoError(t, err)

		_, err = io.Copy(w, file)
		require.NoError(t, err)
	}
	err := writer.Close()
	require.NoError(t, err)
	return buf.Bytes()
}

func TestEnglish(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	resource := testGenerateEnglishResource(t)

	english := NewEnglish()

	err := english.Load(resource)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		word, err := english.Generate(nil)
		require.NoError(t, err)

		t.Log(word)
	}

	testsuite.IsDestroyed(t, english)
}
