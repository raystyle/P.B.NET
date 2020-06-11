// Package gosrc generate by resource/code/anko/package.go, don't edit it.
package gosrc

import (
	"crypto/rand"
	"reflect"

	"github.com/mattn/anko/env"
)

func init() {
	initCryptoRand()
}

func initCryptoRand() {
	env.Packages["crypto/rand"] = map[string]reflect.Value{
		// define constants

		// define variables
		"Reader": reflect.ValueOf(rand.Reader),

		// define functions
		"Read":  reflect.ValueOf(rand.Read),
		"Prime": reflect.ValueOf(rand.Prime),
		"Int":   reflect.ValueOf(rand.Int),
	}
	var ()
	env.PackageTypes["crypto/rand"] = map[string]reflect.Type{}
}
