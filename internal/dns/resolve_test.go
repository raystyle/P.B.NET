package dns

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

const (
	dnsServer        = "8.8.8.8:53"
	dnsTLSDomainMode = "dns.google:853|8.8.8.8,8.8.4.4"
	dnsDOH           = "https://cloudflare-dns.com/dns-query"
	domain           = "cloudflare-dns.com"
	domainPunycode   = "münchen.com"

	// dnsDOH           = "https://cloudflare-dns.com/dns-query"
	// dnsDOH           = "https://mozilla.cloudflare-dns.com/dns-query"
)

func TestSystemResolve(t *testing.T) {
	// ipv4
	ipList, err := systemResolve(domain, IPv4)
	require.NoError(t, err)
	t.Log("system resolve ipv4:", ipList)
	// ipv6
	ipList, err = systemResolve(domain, IPv6)
	require.NoError(t, err)
	t.Log("system resolve ipv6:", ipList)
	// invalid host
	ipList, err = systemResolve("asd.asd", IPv4)
	require.Error(t, err)
	require.Nil(t, ipList)
	// invalid type
	ipList, err = systemResolve(domain, "asd")
	require.Error(t, err)
	require.Nil(t, ipList)
}

func TestCustomResolve(t *testing.T) {
	opt := Options{}
	// udp
	ipList, err := customResolve(dnsServer, domain, &opt)
	require.NoError(t, err)
	t.Log("UDP IPv4:", ipList)
	// punycode
	ipList, err = customResolve(dnsServer, domainPunycode, &opt)
	require.NoError(t, err)
	t.Log("UDP IPv4 punycode:", ipList)
	// tcp
	opt.Method = TCP
	opt.Type = IPv6
	ipList, err = customResolve(dnsServer, domain, &opt)
	require.NoError(t, err)
	t.Log("TCP IPv6:", ipList)
	// tls
	opt.Method = DoT
	opt.Type = IPv4
	ipList, err = customResolve(dnsTLSDomainMode, domain, &opt)
	require.NoError(t, err)
	t.Log("DoT IPv4:", ipList)
	// doh
	opt.Method = DoH
	ipList, err = customResolve(dnsDOH, domain, &opt)
	require.NoError(t, err)
	t.Log("DoH IPv4:", ipList)
	// is ip
	ipList, err = customResolve(dnsServer, "8.8.8.8", &opt)
	require.NoError(t, err)
	require.Equal(t, "8.8.8.8", ipList[0])
	ipList, err = customResolve(dnsServer, "::1", &opt)
	require.NoError(t, err)
	require.Equal(t, "::1", ipList[0])
	// not domain
	_, err = customResolve(dnsServer, "xxx-", &opt)
	require.Error(t, err)
	require.Equal(t, "invalid domain name: xxx-", err.Error())
	// invalid Type
	opt.Type = "foo"
	_, err = customResolve(dnsServer, domain, &opt)
	require.Error(t, err)
	require.Equal(t, "unknown type: foo", err.Error())
	// invalid method
	opt.Type = IPv4
	opt.Method = "foo"
	_, err = customResolve(dnsServer, domain, &opt)
	require.Error(t, err)
	require.Equal(t, "unknown method: foo", err.Error())
	// dial failed
	opt.Network = "udp"
	opt.Method = UDP
	opt.Timeout = time.Millisecond * 500
	_, err = customResolve("8.8.8.8:153", domain, &opt)
	require.Equal(t, ErrNoConnection, err)
}

func TestDialUDP(t *testing.T) {
	opt := Options{
		Network: "udp",
		dial:    net.DialTimeout,
	}
	msg := packMessage(dnsmessage.TypeA, domain)
	msg, err := dialUDP(dnsServer, msg, &opt)
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
	msg := packMessage(dnsmessage.TypeA, domain)
	msg, err := dialTCP(dnsServer, msg, &opt)
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
	msg := packMessage(dnsmessage.TypeA, domain)
	// domain name mode
	resp, err := dialDoT(dnsTLSDomainMode, msg, &opt)
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
	msg := packMessage(dnsmessage.TypeA, domain)
	// get
	resp, err := dialDoH(dnsDOH, msg, &opt)
	require.NoError(t, err)
	ipList, err := unpackMessage(resp)
	require.NoError(t, err)
	t.Log("DoH get IPv4:", ipList)
	// post
	resp, err = dialDoH(dnsDOH+"#"+strings.Repeat("a", 2048), msg, &opt)
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
