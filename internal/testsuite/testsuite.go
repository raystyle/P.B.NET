package testsuite

import (
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/nettool"
)

var (
	// IPv4Enabled is used to tell tests current system is enable IPv4.
	IPv4Enabled bool

	// IPv6Enabled is used to tell tests current system is enable IPv6.
	IPv6Enabled bool

	// InGoland is used to tell tests in run by Goland.
	InGoland bool
)

func init() {
	printNetworkInfo()
	deployPPROFHTTPServer()
	isInGoland()
}

func printNetworkInfo() {
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
}

func deployPPROFHTTPServer() {
	for port := 9931; port < 65536; port++ {
		if startPPROFHTTPServer(port) {
			fmt.Printf("[debug] pprof http server port: %d\n", port)
			return
		}
	}
	panic("failed to deploy pprof http server")
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
	// wait server serve
	time.Sleep(50 * time.Millisecond)
	return true
}

func isInGoland() {
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

// DeferForPanic is used to add recover and log panic in defer function,
// it used to some tests like this:
//
// defer func() {
//      r := recover()
//      require.NotNil(t, r)
//      t.Log(r)
// }()
func DeferForPanic(t testing.TB) {
	r := recover()
	require.NotNil(t, r)
	t.Logf("\npanic in %s:\n%s\n", t.Name(), r)
}

// CheckErrorInTestMain is used to check error in function TestMain(),
// because no t *testing.T, so we need check it self.
func CheckErrorInTestMain(err error) {
	if err != nil {
		panic(err)
	}
}

func checkOptions(father string, v interface{}) string {
	ok, result := checkSpecialType(father, v)
	if ok {
		return result
	}
	typ := reflect.TypeOf(v)
	var value reflect.Value
	if typ.Kind() == reflect.Ptr {
		// check is nil point
		value = reflect.ValueOf(v)
		typ = value.Type()
		if value.IsNil() {
			return father + typ.Name() + " is nil point"
		}
		value = value.Elem()
		typ = value.Type()
	} else {
		value = reflect.ValueOf(v)
	}
	for i := 0; i < value.NumField(); i++ {
		fieldType := typ.Field(i)
		fieldValue := value.Field(i)
		// skip unexported field
		if fieldType.PkgPath != "" && !fieldType.Anonymous {
			continue
		}
		// skip filed with check tag
		if fieldType.Tag.Get("check") == "-" {
			continue
		}
		switch fieldType.Type.Kind() {
		case reflect.Struct, reflect.Ptr, reflect.Interface:
			var f string
			if father == "" {
				f = typ.Name() + "." + fieldType.Name
			} else {
				f = father + "." + fieldType.Name
			}
			str := checkOptions(f, fieldValue.Interface())
			if str != "" {
				return str
			}
		case reflect.Chan, reflect.Func, reflect.Complex64,
			reflect.Complex128, reflect.UnsafePointer:
			continue
		default:
			if !fieldValue.IsZero() {
				continue
			}
			const format = "%s.%s is zero value"
			if father == "" {
				return fmt.Sprintf(format, typ.Name(), fieldType.Name)
			}
			return fmt.Sprintf(format, father, fieldType.Name)
		}
	}
	return ""
}

func checkSpecialType(father string, v interface{}) (bool, string) {
	var typ string
	switch val := v.(type) {
	case *time.Time:
		if val != nil && !val.IsZero() {
			return true, ""
		}
		typ = "time.Time"
	case time.Time:
		if !val.IsZero() {
			return true, ""
		}
		typ = "time.Time"
	default:
		return false, ""
	}
	if father == "" {
		return true, typ + " is zero value"
	}
	return true, father + " is zero value"
}

// CheckOptions is used to check unmarshal is apply value to each field,
// it will check each field value is zero.
func CheckOptions(t *testing.T, v interface{}) {
	str := checkOptions("", v)
	require.True(t, str == "", str)
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
