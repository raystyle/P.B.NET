package testsuite

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/nettool"
)

// about network
var (
	IPv4Enabled bool
	IPv6Enabled bool
)

func init() {
	// print network information
	IPv4Enabled, IPv6Enabled = nettool.IPEnabled()
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
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/debug/pprof/", pprof.Index)
	serveMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	serveMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	serveMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	serveMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	server := http.Server{Handler: serveMux}
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

// InGoland is used to tell tests in run by Goland.
var InGoland bool

func init() {
	for _, value := range os.Environ() {
		if strings.Contains(value, "IDEA") {
			InGoland = true
			break
		}
	}
}

// TestDataSize is the size of Bytes().
const TestDataSize = 256

// Bytes is used to generate test data: []byte{0, 1, .... 254, 255}.
func Bytes() []byte {
	testdata := make([]byte, TestDataSize)
	for i := 0; i < TestDataSize; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}

// Destroyed is used to check if the object has been recycled by the GC.
// It not need testing.TB.
func Destroyed(object interface{}) bool {
	destroyed := make(chan struct{})
	runtime.SetFinalizer(object, func(interface{}) {
		close(destroyed)
	})
	// total 3 seconds
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	for i := 0; i < 300; i++ {
		timer.Reset(10 * time.Millisecond)
		runtime.GC()
		select {
		case <-destroyed:
			return true
		case <-timer.C:
		}
	}
	return false
}

// IsDestroyed is used to check if the object has been recycled by the GC.
func IsDestroyed(t testing.TB, object interface{}) {
	require.True(t, Destroyed(object), "object not destroyed")
}

// RunParallel is used to call functions with go func().
// object with Add(), Get() ... need it for test with race.
func RunParallel(f ...func()) {
	l := len(f)
	if l == 0 {
		return
	}
	wg := sync.WaitGroup{}
	for i := 0; i < l; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f[i]()
		}(i)
	}
	wg.Wait()
}

// RunHTTPServer is used to start a http or https server and return port.
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
