package testutil

import (
	"runtime"
)

func IsDestroyed(object interface{}, gcNum int) bool {
	destroyed := false
	runtime.SetFinalizer(object, func(_ interface{}) {
		destroyed = true
	})
	for i := 0; i < gcNum; i++ {
		runtime.GC()
	}
	return destroyed
}
