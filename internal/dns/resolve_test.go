package dns

import (
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"

	"project/internal/testsuite"
)

const (
	// resolve target
	testDomain = "cloudflare-dns.com"
)

func TestSystemResolve(t *testing.T) {
	// ipv4
	ipList, err := systemResolve(IPv4, testDomain)
	require.NoError(t, err)
	t.Log("system resolve ipv4:", ipList)
	// ipv6
	ipList, err = systemResolve(IPv6, testDomain)
	require.NoError(t, err)
	t.Log("system resolve ipv6:", ipList)
	// invalid host
	ipList, err = systemResolve(IPv4, "asd")
	require.Error(t, err)
	require.Equal(t, 0, len(ipList))
}

func TestCustomResolve(t *testing.T) {
	opts := &Options{
		dial:      net.DialTimeout,
		transport: &http.Transport{},
	}
	switch {
	case testsuite.EnableIPv4():
		const (
			udpServer = "1.1.1.1:53"
			tcpServer = "8.8.8.8:53"
			tlsIP     = "8.8.4.4:853"
			tlsDomain = "dns.google:853|8.8.8.8,8.8.4.4"
		)
		// udp
		ipList, err := customResolve(UDP, udpServer, testDomain, IPv4, opts)
		require.NoError(t, err)
		t.Log("UDP IPv4:", ipList)
		// tcp
		ipList, err = customResolve(TCP, tcpServer, testDomain, IPv4, opts)
		require.NoError(t, err)
		t.Log("TCP IPv4:", ipList)
		// dot ip mode
		ipList, err = customResolve(DoT, tlsIP, testDomain, IPv4, opts)
		require.NoError(t, err)
		t.Log("DOT-IP IPv4:", ipList)
		// dot domain mode
		ipList, err = customResolve(DoT, tlsDomain, testDomain, IPv4, opts)
		require.NoError(t, err)
		t.Log("DOT-Domain IPv4:", ipList)
	case testsuite.EnableIPv6():
		const (
			udpServer = "[2606:4700:4700::1001]:53"
			tcpServer = "[2606:4700:4700::1001]:53"
			TLSIP     = "[2606:4700:4700::1001]:853"
			TLSDomain = "cloudflare-dns.com:853|2606:4700:4700::1111,2606:4700:4700::1001"
		)
		// udp
		ipList, err := customResolve(UDP, udpServer, testDomain, IPv6, opts)
		require.NoError(t, err)
		t.Log("UDP IPv6:", ipList)
		// tcp
		ipList, err = customResolve(TCP, tcpServer, testDomain, IPv6, opts)
		require.NoError(t, err)
		t.Log("TCP IPv6:", ipList)
		// dot ip mode
		ipList, err = customResolve(DoT, TLSIP, testDomain, IPv6, opts)
		require.NoError(t, err)
		t.Log("DOT-IP IPv6:", ipList)
		// dot domain mode
		ipList, err = customResolve(DoT, TLSDomain, testDomain, IPv6, opts)
		require.NoError(t, err)
		t.Log("DOT-Domain IPv6:", ipList)
	}
	// doh
	const dnsDOH = "https://cloudflare-dns.com/dns-query"
	ipList, err := customResolve(DoH, dnsDOH, testDomain, IPv4, opts)
	require.NoError(t, err)
	t.Log("DOH:", ipList)

	// resolve ip
	const dnsServer = "1.0.0.1:53"
	ipList, err = customResolve(UDP, dnsServer, "1.1.1.1", IPv4, opts)
	require.NoError(t, err)
	require.Equal(t, []string{"1.1.1.1"}, ipList)

	// resolve domain name with punycode
	const domainPunycode = "münchen.com"
	ipList, err = customResolve(UDP, "8.8.8.8:53", domainPunycode, IPv4, opts)
	require.NoError(t, err)
	t.Log("punycode:", ipList)

	// empty domain
	ipList, err = customResolve(UDP, dnsServer, "", IPv4, opts)
	require.Error(t, err)
	require.Equal(t, 0, len(ipList))

	// resolve failed
	opts.Timeout = time.Second
	ipList, err = customResolve(UDP, "0.0.0.0:1", domainPunycode, IPv4, opts)
	require.Error(t, err)
	require.Equal(t, 0, len(ipList))
}

var (
	testDNSMessage = packMessage(dnsmessage.TypeA, testDomain)
)

func TestDialUDP(t *testing.T) {
	const (
		dnsServerIPV4 = "8.8.8.8:53"
		dnsServerIPv6 = "[2606:4700:4700::1001]:53"
	)
	opt := Options{dial: net.DialTimeout}
	if testsuite.EnableIPv4() {
		msg, err := dialUDP(dnsServerIPV4, testDNSMessage, &opt)
		require.NoError(t, err)
		ipList, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("UDP IPv4(IPv4 DNS Server):", ipList)
	}
	if testsuite.EnableIPv6() {
		msg, err := dialUDP(dnsServerIPv6, testDNSMessage, &opt)
		require.NoError(t, err)
		ipList, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("UDP IPv4(IPv6 DNS Server):", ipList)
	}
	// unknown network
	opt.Network = "foo network"
	_, err := dialUDP("", nil, &opt)
	require.Error(t, err)
	// no port
	opt.Network = "udp"
	_, err = dialUDP("1.2.3.4", nil, &opt)
	require.Error(t, err)
	// no response
	opt.Timeout = time.Second
	_, err = dialUDP("1.2.3.4:23421", nil, &opt)
	require.Equal(t, ErrNoConnection, err)
}

func TestDialTCP(t *testing.T) {
	const (
		dnsServerIPV4 = "8.8.8.8:53"
		dnsServerIPv6 = "[2606:4700:4700::1001]:53"
	)
	opt := Options{dial: net.DialTimeout}
	if testsuite.EnableIPv4() {
		msg, err := dialTCP(dnsServerIPV4, testDNSMessage, &opt)
		require.NoError(t, err)
		ipList, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("TCP IPv4(IPv4 DNS Server):", ipList)
	}
	if testsuite.EnableIPv6() {
		msg, err := dialTCP(dnsServerIPv6, testDNSMessage, &opt)
		require.NoError(t, err)
		ipList, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("TCP IPv4(IPv6 DNS Server):", ipList)
	}
	// unknown network
	opt.Network = "foo network"
	_, err := dialTCP("", nil, &opt)
	require.Error(t, err)
	// no port
	opt.Network = "tcp"
	_, err = dialTCP("1.2.3.4", nil, &opt)
	require.Error(t, err)
}

func TestDialDoT(t *testing.T) {
	opt := Options{
		dial: net.DialTimeout,
	}
	// domain name mode
	resp, err := dialDoT("testDNSTLSDomainMode", testDNSMessage, &opt)
	require.NoError(t, err)
	ipList, err := unpackMessage(resp)
	require.NoError(t, err)
	t.Log("DoT domain IPv4:", ipList)
	// ip mode
	resp, err = dialDoT("1.1.1.1:853", testDNSMessage, &opt)
	require.NoError(t, err)
	ipList, err = unpackMessage(resp)
	require.NoError(t, err)
	t.Log("DoT ip IPv4:", ipList)
	// no port(ip mode)
	_, err = dialDoT("1.2.3.4", testDNSMessage, &opt)
	require.Error(t, err)
	// dial failed
	_, err = dialDoT("127.0.0.1:888", testDNSMessage, &opt)
	require.Error(t, err)
	// error ip(domain mode)
	_, err = dialDoT("dns.google:853|127.0.0.1", testDNSMessage, &opt)
	require.Equal(t, ErrNoConnection, err)
	// no port(domain mode)
	_, err = dialDoT("dns.google|1.2.3.235", testDNSMessage, &opt)
	require.Error(t, err)
	// invalid config
	_, err = dialDoT("asd:153|xxx|xxx", testDNSMessage, &opt)
	require.Error(t, err)
	require.Equal(t, "invalid address: asd:153|xxx|xxx", err.Error())
}

func TestDialDoH(t *testing.T) {
	opt := Options{}
	msg := packMessage(dnsmessage.TypeA, testDomain)
	// get
	resp, err := dialDoH("testDNSDOH", msg, &opt)
	require.NoError(t, err)
	ipList, err := unpackMessage(resp)
	require.NoError(t, err)
	t.Log("DoH get IPv4:", ipList)
	// post
	resp, err = dialDoH("testDNSDOH"+"#"+strings.Repeat("a", 2048), msg, &opt)
	require.NoError(t, err)
	ipList, err = unpackMessage(resp)
	require.NoError(t, err)
	t.Log("DoH post IPv4:", ipList)
	// invalid doh server
	_, err = dialDoH("foo\n", msg, &opt)
	require.Error(t, err)
	_, err = dialDoH("foo\n"+"#"+strings.Repeat("a", 2048), msg, &opt)
	require.Error(t, err)
	// Do failed
	_, err = dialDoH("http://asd.1dsa.asd", msg, &opt)
	require.Error(t, err)
}
