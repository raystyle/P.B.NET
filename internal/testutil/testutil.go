package testutil

import (
	"net/http"
	_ "net/http/pprof"
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
	require.True(t, isDestroyed(object, gcNum), "object not destroyed")
}

func PPROF() {
	go func() {
		_ = http.ListenAndServe("127.0.0.1:8080", nil)
	}()
}
