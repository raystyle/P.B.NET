package testsuite

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	enableIPv4 bool
	enableIPv6 bool
)

func init() {
	initGetIPv4Address()
	initGetIPv6Address()
	initGetHTTP()
	initGetHTTPS()

	// check IPv4
	if os.Getenv("skip_ipv4") != "1" {
		for i := 0; i < 5; i++ {
			addr := GetIPv4Address()
			conn, err := net.DialTimeout("tcp4", addr, 15*time.Second)
			if err == nil {
				_ = conn.Close()
				enableIPv4 = true
				break
			}
		}
	}

	// check IPv6
	if os.Getenv("skip_ipv6") != "1" {
		for i := 0; i < 5; i++ {
			addr := GetIPv6Address()
			conn, err := net.DialTimeout("tcp6", addr, 15*time.Second)
			if err == nil {
				_ = conn.Close()
				enableIPv6 = true
				break
			}
		}
	}

	// check network
	if !enableIPv4 && !enableIPv6 {
		fmt.Print("network unavailable")
		os.Exit(0)
	}

	// deploy pprof
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/debug/pprof/", pprof.Index)
	serverMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	serverMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	serverMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	serverMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	server := http.Server{Handler: serverMux}
	var (
		listener net.Listener
		err      error
	)
	listener, err = net.Listen("tcp", "localhost:9931")
	if err != nil {
		listener, err = net.Listen("tcp", "localhost:0")
		if err != nil {
			fmt.Printf("failed to deploy pprof: %s\n", err)
			os.Exit(0)
		}
	}
	fmt.Printf("[debug] pprof: %s\n", listener.Addr())
	go func() { _ = server.Serve(listener) }()
}

// EnableIPv4 is used to determine whether IPv4 is available
func EnableIPv4() bool {
	return enableIPv4
}

// EnableIPv6 is used to determine whether IPv6 is available
func EnableIPv6() bool {
	return enableIPv6
}

func isDestroyed(object interface{}) bool {
	destroyed := make(chan struct{})
	runtime.SetFinalizer(object, func(_ interface{}) {
		close(destroyed)
	})
	for i := 0; i < 40; i++ {
		runtime.GC()
		select {
		case <-destroyed:
			return true
		case <-time.After(25 * time.Millisecond):
		}
	}
	return false
}

// IsDestroyed is used to check if the object has been recycled by the GC
func IsDestroyed(t testing.TB, object interface{}) {
	require.True(t, isDestroyed(object), "object not destroyed")
}

// Bytes is used to generate test data: []byte{0, 1, .... 254, 255}
func Bytes() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

// RunHTTPServer is used to start a http or https server
func RunHTTPServer(t testing.TB, network string, server *http.Server) string {
	listener, err := net.Listen(network, server.Addr)
	require.NoError(t, err)

	// run
	go func() {
		if server.TLSConfig != nil {
			_ = server.ServeTLS(listener, "", "")
		} else {
			_ = server.Serve(listener)
		}
	}()

	// get port
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	return port
}
