package testsuite

import (
	"fmt"
	"sync"
)

const suffix = "/ip/?callback=?&testdomain=test-ipv6.com&testname=test_a"

var (
	testIPv4Addresses  map[string]bool
	testIPv4AddressesM sync.Mutex
)

func initGetIPv4Address() {
	testIPv4Addresses = make(map[string]bool)
	for _, address := range []string{
		"66.220.4.230:80",    // ipv4.vm0.test-ipv6.com:80
		"216.218.228.119:80", // ipv4.vm1.test-ipv6.com:80
		"216.218.228.125:80", // ipv4.vm2.test-ipv6.com:80
		"103.6.84.104:80",    // ipv4.test-ipv6.hkg.vr.org:80
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
		"2001:470:1:18::4:230",  // "ipv6.vm0.test-ipv6.com:80",
		"2001:470:1:18::119",    // "ipv6.vm1.test-ipv6.com:80",
		"2001:470:1:18::125",    // "ipv6.vm2.test-ipv6.com:80",
		"2403:2500:8000:1::aa2", // "ipv6.test-ipv6.hkg.vr.org:80",
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
	testIPv4Domains  map[string]bool
	testIPv4DomainsM sync.Mutex
)

func initGetIPv4Domain() {
	testIPv4Domains = make(map[string]bool)
	for _, domain := range []string{
		"ipv4.vm0.test-ipv6.com:80",
		"ipv4.vm1.test-ipv6.com:80",
		"ipv4.vm2.test-ipv6.com:80",
		"ipv4.test-ipv6.hkg.vr.org:80",
	} {
		testIPv4Domains[domain] = false
	}
}

// GetIPv4Domain is used to return a domain name for test Dial(tcp4)
func GetIPv4Domain() string {
	testIPv4DomainsM.Lock()
	defer testIPv4DomainsM.Unlock()
	for {
		for domain, used := range testIPv4Domains {
			if !used {
				testIPv4Domains[domain] = true
				return domain
			}
		}
		// reset all clients flag
		for domain := range testIPv4Domains {
			testIPv4Domains[domain] = false
		}
	}
}

var (
	testIPv6Domains  map[string]bool
	testIPv6DomainsM sync.Mutex
)

func initGetIPv6Domain() {
	testIPv6Domains = make(map[string]bool)
	for _, domain := range []string{
		"ipv6.vm0.test-ipv6.com:80",
		"ipv6.vm1.test-ipv6.com:80",
		"ipv6.vm2.test-ipv6.com:80",
		"ipv6.test-ipv6.hkg.vr.org:80",
	} {
		testIPv6Domains[domain] = false
	}
}

// GetIPv6Domain is used to return a domain name for test Dial(tcp6)
func GetIPv6Domain() string {
	testIPv6DomainsM.Lock()
	defer testIPv6DomainsM.Unlock()
	for {
		for domain, used := range testIPv6Domains {
			if !used {
				testIPv6Domains[domain] = true
				return domain
			}
		}
		// reset all clients flag
		for domain := range testIPv6Domains {
			testIPv6Domains[domain] = false
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
}

// GetHTTP is used to return a url for test http client
func GetHTTP() string {
	const format = "http://%s" + suffix
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
}

// GetHTTP is used to return a url for test http client
func GetHTTPS() string {
	const format = "https://%s" + suffix
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
