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
)

var (
	enableIPv4 bool
	enableIPv6 bool
)

func init() {
	checkNetwork()
	deployPPROF()
}

func checkNetwork() {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags != net.FlagUp|net.FlagBroadcast|net.FlagMulticast {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipAddr := strings.Split(addr.String(), "/")[0]
			ip := net.ParseIP(ipAddr)
			ip4 := ip.To4()
			if ip4 != nil {
				if ip4.IsGlobalUnicast() {
					enableIPv4 = true
				}
			} else {
				if ip.To16().IsGlobalUnicast() {
					enableIPv6 = true
				}
			}
			if enableIPv4 && enableIPv6 {
				break
			}
		}
	}
	if !enableIPv4 && !enableIPv6 {
		fmt.Println("[warning] network unavailable")
	}
}

func deployPPROF() {
	for port := 9931; port < 65536; port++ {
		actualPort := startPPROF(port)
		if actualPort != "" {
			fmt.Printf("[Debug] pprof http server port: %s\n", actualPort)
			return
		}
	}
	panic("failed to deploy pprof http server")
}

// return port
func startPPROF(port int) string {
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
		return ""
	}
	ipv6, err = net.Listen("tcp6", address)
	if err != nil {
		return ""
	}
	go func() { _ = server.Serve(ipv4) }()
	go func() { _ = server.Serve(ipv6) }()
	_, p, _ := net.SplitHostPort(address)
	return p
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

// Bytes is used to generate test data: []byte{0, 1, .... 254, 255}
func Bytes() []byte {
	testdata := make([]byte, 256)
	for i := 0; i < 256; i++ {
		testdata[i] = byte(i)
	}
	return testdata
}
