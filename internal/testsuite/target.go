package testsuite

import (
	"sync"
)

var (
	testIPv4Addresses  map[string]bool
	testIPv4AddressesM sync.Mutex
)

func initGetIPv4Address() {
	testIPv4Addresses = make(map[string]bool)
	for _, address := range []string{
		"66.220.4.230:80",    // ipv4.vm0.test-ipv6.com
		"216.218.228.119:80", // ipv4.vm1.test-ipv6.com
		"216.218.228.125:80", // ipv4.vm2.test-ipv6.com
		"103.6.84.104:80",    // ipv4.test-ipv6.hkg.vr.org
		"52.74.223.119:443",  // github.com
		"104.16.142.228:443", // www.mozilla.org
		"12.107.4.52:80",     // www.msftconnecttest.com
		"119.3.60.210:443",   // bilibili.com
		"106.75.240.122:443", // bilibili.com
		"120.92.172.228:443", // bilibili.com
		"122.228.77.85:443",  // www.bilibili.com
	} {
		testIPv4Addresses[address] = false
	}
}

// GetIPv4Address is used to return a address for test Dial(tcp4)
func getIPv4Address() string {
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
		"[2001:470:1:18::4:230]:80",  // ipv6.vm0.test-ipv6.com
		"[2001:470:1:18::119]:80",    // ipv6.vm1.test-ipv6.com
		"[2001:470:1:18::125]:80",    // ipv6.vm2.test-ipv6.com
		"[2403:2500:8000:1::aa2]:80", // ipv6.test-ipv6.hkg.vr.org
		"[2606:4700::6810:8ee4]:443", // www.mozilla.org
		"[2606:4700::6810:8fe4]:443", // www.mozilla.org
	} {
		testIPv6Addresses[address] = false
	}
}

// GetIPv6Address is used to return a address for test Dial(tcp6)
func getIPv6Address() string {
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
