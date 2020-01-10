package testsuite

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/xnet/xnetutil"
)

// network
var (
	IPv4Enabled bool
	IPv6Enabled bool
)

func init() {
	// print network information
	IPv4Enabled, IPv6Enabled = xnetutil.IPEnabled()
	if !IPv4Enabled && !IPv6Enabled {
		fmt.Println("[debug] network unavailable")
	} else {
		const format = "[debug] network: IPv4-%t IPv6-%t"
		str := fmt.Sprintf(format, IPv4Enabled, IPv6Enabled)
		str = strings.ReplaceAll(str, "true", "Enabled")
		str = strings.ReplaceAll(str, "false", "Disabled")
		fmt.Println(str)
	}
	// deploy pprof http server
	var (
		port int
		ok   bool
	)
	for port = 9931; port < 65536; port++ {
		ok = startPPROFHTTPServer(port)
		if ok {
			break
		}
	}
	if ok {
		fmt.Printf("[debug] pprof http server port: %d\n", port)
	} else {
		panic("failed to deploy pprof http server")
	}
}

func startPPROFHTTPServer(port int) bool {
	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/debug/pprof/", pprof.Index)
	serverMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	serverMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	serverMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	serverMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	server := http.Server{Handler: serverMux}
	var (
		ipv4 net.Listener
		ipv6 net.Listener
		err  error
	)
	address := fmt.Sprintf("localhost:%d", port)
	ipv4, err = net.Listen("tcp4", address)
	if err != nil {
		return false
	}
	ipv6, err = net.Listen("tcp6", address)
	if err != nil {
		return false
	}
	go func() { _ = server.Serve(ipv4) }()
	go func() { _ = server.Serve(ipv6) }()
	return true
}

func isDestroyed(object interface{}) bool {
	destroyed := make(chan struct{})
	runtime.SetFinalizer(object, func(interface{}) {
		close(destroyed)
	})
	// total 3 second
	for i := 0; i < 12; i++ {
		runtime.GC()
		select {
		case <-destroyed:
			return true
		case <-time.After(250 * time.Millisecond):
		}
	}
	return false
}

// IsDestroyed is used to check if the object has been recycled by the GC
func IsDestroyed(t testing.TB, object interface{}) {
	require.True(t, isDestroyed(object), "object not destroyed")
}

// RunHTTPServer is used to start a http or https server and return port
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

// Bytes is used to generate test data: []byte{0, 1, .... 254, 255}
func Bytes() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}
