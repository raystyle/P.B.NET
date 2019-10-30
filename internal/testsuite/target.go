package testsuite

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	testIPv4Addresses  map[string]bool
	testIPv4AddressesM sync.Mutex
)

func initGetIPv4Address() {
	testIPv4Addresses = make(map[string]bool)
	for _, address := range []string{
		"ipv4.vm0.test-ipv6.com:80",
		"ipv4.vm1.test-ipv6.com:80",
		"ipv4.vm2.test-ipv6.com:80",

		"ds.vm0.test-ipv6.com:80",
		"ds.vm1.test-ipv6.com:80",
		"ds.vm2.test-ipv6.com:80",

		"ipv4.test-ipv6.hkg.vr.org:80",
		"ds.test-ipv6.hkg.vr.org:80",
	} {
		testIPv4Addresses[address] = false
	}
}

// GetIPv4Address is used to return a address for test Dial(tcp4)
func GetIPv4Address() string {
	testIPv4AddressesM.Lock()
	defer testIPv4AddressesM.Unlock()
	for {
		for address, used := range testIPv4Addresses {
			if !used {
				testIPv4Addresses[address] = true
				return address
			}
		}
		// reset all clients flag
		for address := range testIPv4Addresses {
			testIPv4Addresses[address] = false
		}
	}
}

var (
	testIPv6Addresses  map[string]bool
	testIPv6AddressesM sync.Mutex
)

func initGetIPv6Address() {
	testIPv6Addresses = make(map[string]bool)
	for _, address := range []string{
		"ipv6.vm0.test-ipv6.com:80",
		"ipv6.vm1.test-ipv6.com:80",
		"ipv6.vm2.test-ipv6.com:80",

		"ds.vm0.test-ipv6.com:80",
		"ds.vm1.test-ipv6.com:80",
		"ds.vm2.test-ipv6.com:80",

		"ipv6.test-ipv6.hkg.vr.org:80",
		"ds.test-ipv6.hkg.vr.org:80",
	} {
		testIPv6Addresses[address] = false
	}
}

// GetIPv6Address is used to return a address for test Dial(tcp6)
func GetIPv6Address() string {
	testIPv6AddressesM.Lock()
	defer testIPv6AddressesM.Unlock()
	for {
		for address, used := range testIPv6Addresses {
			if !used {
				testIPv6Addresses[address] = true
				return address
			}
		}
		// reset all clients flag
		for address := range testIPv6Addresses {
			testIPv6Addresses[address] = false
		}
	}
}

var (
	testHTTP  map[string]bool
	testHTTPM sync.Mutex
)

func initGetHTTP() {
	testHTTP = make(map[string]bool)
	for _, address := range []string{
		"ds.vm0.test-ipv6.com",
		"ds.vm1.test-ipv6.com",
		"ds.vm2.test-ipv6.com",
		"ds.test-ipv6.hkg.vr.org",
	} {
		testHTTP[address] = false
	}

	if EnableIPv4() {
		for _, address := range []string{
			"ipv4.vm0.test-ipv6.com",
			"ipv4.vm1.test-ipv6.com",
			"ipv4.vm2.test-ipv6.com",
			"ipv4.test-ipv6.hkg.vr.org",
		} {
			testHTTP[address] = false
		}
	}

	if EnableIPv6() {
		for _, address := range []string{
			"ipv6.vm0.test-ipv6.com",
			"ipv6.vm1.test-ipv6.com",
			"ipv6.vm2.test-ipv6.com",
			"ipv6.test-ipv6.hkg.vr.org",
		} {
			testHTTP[address] = false
		}
	}
}

// GetHTTP is used to return a url for test http client
func GetHTTP() string {
	const format = "http://%s/ip/?callback=?&testdomain=test-ipv6.com&testname=test_a"
	testHTTPM.Lock()
	defer testHTTPM.Unlock()
	for {
		for address, used := range testHTTP {
			if !used {
				testHTTP[address] = true
				return fmt.Sprintf(format, address)
			}
		}
		// reset all clients flag
		for address := range testHTTP {
			testHTTP[address] = false
		}
	}
}

var (
	testHTTPS  map[string]bool
	testHTTPSM sync.Mutex
)

func initGetHTTPS() {
	testHTTPS = make(map[string]bool)
	for _, address := range []string{
		"ds.vm0.test-ipv6.com",
		"ds.vm1.test-ipv6.com",
		"ds.vm2.test-ipv6.com",
		"ds.test-ipv6.hkg.vr.org",
	} {
		testHTTPS[address] = false
	}

	if EnableIPv4() {
		for _, address := range []string{
			"ipv4.vm0.test-ipv6.com",
			"ipv4.vm1.test-ipv6.com",
			"ipv4.vm2.test-ipv6.com",
			"ipv4.test-ipv6.hkg.vr.org",
		} {
			testHTTPS[address] = false
		}
	}

	if EnableIPv6() {
		for _, address := range []string{
			"ipv6.vm0.test-ipv6.com",
			"ipv6.vm1.test-ipv6.com",
			"ipv6.vm2.test-ipv6.com",
			"ipv6.test-ipv6.hkg.vr.org",
		} {
			testHTTPS[address] = false
		}
	}
}

// GetHTTP is used to return a url for test http client
func GetHTTPS() string {
	const format = "https://%s/ip/?callback=?&testdomain=test-ipv6.com&testname=test_a"
	testHTTPSM.Lock()
	defer testHTTPSM.Unlock()
	for {
		for address, used := range testHTTPS {
			if !used {
				testHTTPS[address] = true
				return fmt.Sprintf(format, address)
			}
		}
		// reset all clients flag
		for address := range testHTTPS {
			testHTTPS[address] = false
		}
	}
}

// HTTPResponse is used to validate http client get
func HTTPResponse(t testing.TB, resp *http.Response) {
	require.Equal(t, http.StatusOK, resp.StatusCode)
	defer func() { _ = resp.Body.Close() }()
	b, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "callback", string(b)[:8])
}
