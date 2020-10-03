// +build go1.10

package packages

import (
	"reflect"
	"time"

	"project/external/anko/env"
)

func timeGo110() {
	env.Packages["time"]["LoadLocationFromTZData"] = reflect.ValueOf(time.LoadLocationFromTZData)
}
