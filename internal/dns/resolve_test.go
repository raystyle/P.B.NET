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
	ipList, err = systemResolve(IPv4, "asd.asd")
	require.Error(t, err)
	require.Nil(t, ipList)
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
			tcpServer = "1.1.1.1:53"
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

	// resolve failed
	opts.Timeout = time.Second
	ipList, err = customResolve(UDP, "0.0.0.0:1", domainPunycode, IPv4, opts)
	require.Error(t, err)
}

func TestDialUDP(t *testing.T) {
	opt := Options{
		Network: "udp",
		dial:    net.DialTimeout,
	}
	msg := packMessage(dnsmessage.TypeA, testDomain)
	msg, err := dialUDP("testIPv4DNSServer", msg, &opt)
	require.NoError(t, err)
	ipList, err := unpackMessage(msg)
	require.NoError(t, err)
	t.Log("UDP IPv4:", ipList)
	// no port
	_, err = dialUDP("1.2.3.4", msg, &opt)
	require.Error(t, err)
	// no response
	_, err = dialUDP("1.2.3.4:23421", msg, &opt)
	require.Equal(t, ErrNoConnection, err)
}

func TestDialTCP(t *testing.T) {
	opt := Options{
		Network: "tcp",
		dial:    net.DialTimeout,
	}
	msg := packMessage(dnsmessage.TypeA, testDomain)
	msg, err := dialTCP("testIPv4DNSServer", msg, &opt)
	require.NoError(t, err)
	ipList, err := unpackMessage(msg)
	require.NoError(t, err)
	t.Log("TCP IPv4:", ipList)
	// no port
	_, err = dialTCP("8.8.8.8", msg, &opt)
	require.Error(t, err)
}

func TestDialDoT(t *testing.T) {
	opt := Options{
		Network: "tcp",
		dial:    net.DialTimeout,
	}
	msg := packMessage(dnsmessage.TypeA, testDomain)
	// domain name mode
	resp, err := dialDoT("testDNSTLSDomainMode", msg, &opt)
	require.NoError(t, err)
	ipList, err := unpackMessage(resp)
	require.NoError(t, err)
	t.Log("DoT domain IPv4:", ipList)
	// ip mode
	resp, err = dialDoT("1.1.1.1:853", msg, &opt)
	require.NoError(t, err)
	ipList, err = unpackMessage(resp)
	require.NoError(t, err)
	t.Log("DoT ip IPv4:", ipList)
	// no port(ip mode)
	_, err = dialDoT("1.2.3.4", msg, &opt)
	require.Error(t, err)
	// dial failed
	_, err = dialDoT("127.0.0.1:888", msg, &opt)
	require.Error(t, err)
	// error ip(domain mode)
	_, err = dialDoT("dns.google:853|127.0.0.1", msg, &opt)
	require.Equal(t, ErrNoConnection, err)
	// no port(domain mode)
	_, err = dialDoT("dns.google|1.2.3.235", msg, &opt)
	require.Error(t, err)
	// invalid config
	_, err = dialDoT("asd:153|xxx|xxx", msg, &opt)
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
