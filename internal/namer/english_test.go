package namer

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/security"
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

func TestEnglish_Load(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("failed to read zip", func(t *testing.T) {
		english := NewEnglish()

		err := english.Load(nil)
		require.Error(t, err)

		testsuite.IsDestroyed(t, english)
	})

	t.Run("read useless file", func(t *testing.T) {
		buf := bytes.NewBuffer(make([]byte, 0, 4096))
		writer := zip.NewWriter(buf)
		_, err := writer.Create("test.txt")
		require.NoError(t, err)
		err = writer.Close()
		require.NoError(t, err)

		english := NewEnglish()

		err = english.Load(buf.Bytes())
		require.Error(t, err)

		testsuite.IsDestroyed(t, english)
	})

	t.Run("failed to load words", func(t *testing.T) {
		patch := func(file *zip.File) (*security.Bytes, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(loadWordsFromZipFile, patch)
		defer pg.Unpatch()

		resource := testGenerateEnglishResource(t)

		english := NewEnglish()

		err := english.Load(resource)
		monkey.IsMonkeyError(t, err)

		testsuite.IsDestroyed(t, english)
	})
}

func TestEnglish_Generate(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("failed to check word number", func(t *testing.T) {
		english := NewEnglish()
		english.prefix = security.NewBytes(nil)
		english.stem = security.NewBytes(nil)
		english.suffix = security.NewBytes(nil)

		word, err := english.Generate(nil)
		require.EqualError(t, err, "empty prefix")
		require.Zero(t, word)

		testsuite.IsDestroyed(t, english)
	})

	t.Run("generate empty word", func(t *testing.T) {
		resource := testGenerateEnglishResource(t)

		english := NewEnglish()

		err := english.Load(resource)
		require.NoError(t, err)

		opts := Options{
			DisablePrefix: true,
			DisableStem:   true,
			DisableSuffix: true,
		}
		word, err := english.Generate(&opts)
		require.EqualError(t, err, "generated a empty word")
		require.Zero(t, word)

		testsuite.IsDestroyed(t, english)
	})
}

func TestEnglish_checkWordNumber(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("empty prefix", func(t *testing.T) {
		english := NewEnglish()
		english.prefix = security.NewBytes(nil)

		err := english.checkWordNumber()
		require.EqualError(t, err, "empty prefix")

		testsuite.IsDestroyed(t, english)
	})

	t.Run("empty stem", func(t *testing.T) {
		english := NewEnglish()
		english.prefix = security.NewBytes([]byte{0})
		english.stem = security.NewBytes(nil)

		err := english.checkWordNumber()
		require.EqualError(t, err, "empty stem")

		testsuite.IsDestroyed(t, english)
	})

	t.Run("empty suffix", func(t *testing.T) {
		english := NewEnglish()
		english.prefix = security.NewBytes([]byte{0})
		english.stem = security.NewBytes([]byte{0})
		english.suffix = security.NewBytes(nil)

		err := english.checkWordNumber()
		require.EqualError(t, err, "empty suffix")

		testsuite.IsDestroyed(t, english)
	})
}
