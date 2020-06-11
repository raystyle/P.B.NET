package anko

import (
	"github.com/mattn/anko/core"
	"github.com/mattn/anko/env"
)

// NewEnv is used to create an new global scope with packages.
func NewEnv() *env.Env {
	e := env.NewEnv()
	core.Import(e)
	return e
}
