package namer

import (
	"archive/zip"
	"bufio"
	"bytes"
	"io/ioutil"

	"github.com/pkg/errors"

	"project/internal/security"
)

// Namer is used to generate a random name from dictionary.
type Namer interface {
	// Load is used to load resource about namer.
	Load(data []byte) error

	// Generate is used to generate a random word.
	Generate(opts *Options) (string, error)
}

// Options include options about all namer.
type Options struct {
	DisablePrefix bool `toml:"disable_prefix"`
	DisableStem   bool `toml:"disable_stem"`
	DisableSuffix bool `toml:"disable_suffix"`
}

func loadWordFromZipFile(file *zip.File) (*security.Bytes, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer func() { _ = rc.Close() }()
	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer security.CoverBytes(data)
	return security.NewBytes(data), nil
}

func loadWordFromSecurityBytes(sb *security.Bytes) map[string]struct{} {
	data := sb.Get()
	defer sb.Put(data)
	words := make(map[string]struct{}, 256)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		word := scanner.Text()
		if word != "" {
			words[word] = struct{}{}
		}
	}
	return words
}
