package testnamer

import (
	"errors"

	"project/internal/namer"
)

// errors about namer.
var (
	ErrLoad     = errors.New("failed to load")
	ErrGenerate = errors.New("failed to generate")
)

// Namer implemented namer.Namer.
type Namer struct {
	prefixes map[string]struct{}
	stems    map[string]struct{}
	suffixes map[string]struct{}

	// return error flag
	load     bool
	generate bool
}

// Load is a padding function.
func (namer *Namer) Load([]byte) error {
	if namer.load {
		return ErrLoad
	}
	return nil
}

// Generate is used to generate a random word.
func (namer *Namer) Generate(*namer.Options) (string, error) {
	if namer.generate {
		return "", ErrGenerate
	}
	var (
		prefix string
		stem   string
		suffix string
	)
	for prefix = range namer.prefixes {
	}
	for stem = range namer.stems {
	}
	for suffix = range namer.suffixes {
	}
	return prefix + stem + suffix, nil
}

// NewNamer is used to create a namer for test.
func NewNamer() *Namer {
	prefixes := make(map[string]struct{})
	stems := make(map[string]struct{})
	suffixes := make(map[string]struct{})
	prefixes["dis"] = struct{}{}
	prefixes["in"] = struct{}{}
	prefixes["im"] = struct{}{}
	prefixes["il"] = struct{}{}
	stems["agr"] = struct{}{}
	stems["ann"] = struct{}{}
	stems["astro"] = struct{}{}
	stems["audi"] = struct{}{}
	suffixes["st"] = struct{}{}
	suffixes["eer"] = struct{}{}
	suffixes["er"] = struct{}{}
	suffixes["or"] = struct{}{}
	return &Namer{
		prefixes: prefixes,
		stems:    stems,
		suffixes: suffixes,
	}
}

// NewNamerWithLoadFailed is used to create a namer that will failed to load resource.
func NewNamerWithLoadFailed() *Namer {
	return &Namer{load: true}
}

// NewNamerWithGenerateFailed is used to create a namer that will failed to generate word.
func NewNamerWithGenerateFailed() *Namer {
	return &Namer{generate: true}
}
