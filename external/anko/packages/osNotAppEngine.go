// +build !appengine

package packages

import (
	"os"
	"reflect"

	"project/external/anko/env"
)

func osNotAppEngine() {
	env.Packages["os"]["Getppid"] = reflect.ValueOf(os.Getppid)
}
