package testutil

import (
	"runtime"

	"github.com/stretchr/testify/require"
)

func isDestroyed(object interface{}, gcNum int) bool {
	destroyed := false
	runtime.SetFinalizer(object, func(_ interface{}) {
		destroyed = true
	})
	for i := 0; i < gcNum; i++ {
		runtime.GC()
	}
	return destroyed
}

func IsDestroyed(t require.TestingT, object interface{}, gcNum int) {
	require.True(t, isDestroyed(object, gcNum))
}
