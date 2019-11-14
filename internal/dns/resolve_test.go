package dns

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"

	"project/internal/convert"
	"project/internal/testsuite"
)

const (
	testDomain = "cloudflare-dns.com"
)

func TestCustomResolve(t *testing.T) {
	t.Parallel()

	const domainPunycode = "错的是.世界"

	ctx := context.Background()
	opts := &Options{
		dialContext: new(net.Dialer).DialContext,
		transport:   new(http.Transport),
	}

	if testsuite.EnableIPv4() {
		const (
			udpServer = "1.1.1.1:53"
			tcpServer = "1.0.0.1:53"
			tlsIP     = "8.8.4.4:853"
			tlsDomain = "dns.google:853|8.8.8.8,8.8.4.4"
		)

		t.Run("IPv4 UDP", func(t *testing.T) {
			result, err := resolve(ctx, MethodUDP, udpServer, testDomain, TypeIPv4, opts)
			require.NoError(t, err)
			t.Log("UDP IPv4:", result)
		})

		t.Run("IPv4 TCP", func(t *testing.T) {
			result, err := resolve(ctx, MethodTCP, tcpServer, testDomain, TypeIPv4, opts)
			require.NoError(t, err)
			t.Log("TCP IPv4:", result)
		})

		t.Run("IPv4 DoT IP mode", func(t *testing.T) {
			result, err := resolve(ctx, MethodDoT, tlsIP, testDomain, TypeIPv4, opts)
			require.NoError(t, err)
			t.Log("DOT-IP IPv4:", result)
		})

		t.Run("IPv4 DoT domain mode", func(t *testing.T) {
			result, err := resolve(ctx, MethodDoT, tlsDomain, testDomain, TypeIPv4, opts)
			require.NoError(t, err)
			t.Log("DOT-Domain IPv4:", result)
		})

		t.Run("IPv4 punycode", func(t *testing.T) {
			result, err := resolve(ctx, MethodUDP, udpServer, domainPunycode, TypeIPv4, opts)
			require.NoError(t, err)
			t.Log("punycode:", result)
		})
	}

	if testsuite.EnableIPv6() {
		const (
			udpServer = "[2606:4700:4700::1111]:53"
			tcpServer = "[2606:4700:4700::1001]:53"
			TLSip     = "[2606:4700:4700::64]:853"
			TLSDomain = "cloudflare-dns.com:853|2606:4700:4700::1111,2606:4700:4700::1001"
		)

		t.Run("IPv6 UDP", func(t *testing.T) {
			result, err := resolve(ctx, MethodUDP, udpServer, testDomain, TypeIPv6, opts)
			require.NoError(t, err)
			t.Log("UDP IPv6:", result)
		})

		t.Run("IPv6 TCP", func(t *testing.T) {
			result, err := resolve(ctx, MethodTCP, tcpServer, testDomain, TypeIPv6, opts)
			require.NoError(t, err)
			t.Log("TCP IPv6:", result)
		})

		t.Run("IPv6 DoT IP mode", func(t *testing.T) {
			result, err := resolve(ctx, MethodDoT, TLSip, testDomain, TypeIPv6, opts)
			require.NoError(t, err)
			t.Log("DOT-IP IPv6:", result)
		})

		t.Run("IPv6 DoT domain mode", func(t *testing.T) {
			result, err := resolve(ctx, MethodDoT, TLSDomain, testDomain, TypeIPv6, opts)
			require.NoError(t, err)
			t.Log("DOT-Domain IPv6:", result)
		})

		t.Run("IPv6 punycode", func(t *testing.T) {
			result, err := resolve(ctx, MethodUDP, udpServer, domainPunycode, TypeIPv6, opts)
			require.NoError(t, err)
			t.Log("punycode:", result)
		})
	}

	t.Run("DoH", func(t *testing.T) {
		const dnsDOH = "https://cloudflare-dns.com/dns-query"
		result, err := resolve(ctx, MethodDoH, dnsDOH, testDomain, TypeIPv4, opts)
		require.NoError(t, err)
		t.Log("DOH:", result)
	})

	const dnsServer = "1.0.0.1:53"

	t.Run("resolve IP", func(t *testing.T) {
		result, err := resolve(ctx, MethodUDP, dnsServer, "1.1.1.1", TypeIPv4, opts)
		require.NoError(t, err)
		require.Equal(t, []string{"1.1.1.1"}, result)
	})

	t.Run("empty domain", func(t *testing.T) {
		result, err := resolve(ctx, MethodUDP, dnsServer, "", TypeIPv4, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))
	})

	t.Run("failed to resolve", func(t *testing.T) {
		opts.Timeout = time.Second
		result, err := resolve(ctx, MethodUDP, "0.0.0.0:1", domainPunycode, TypeIPv4, opts)
		require.Error(t, err)
		require.Equal(t, 0, len(result))
	})
}

var (
	testDNSMessage = packMessage(dnsmessage.TypeA, testDomain)
)

func TestDialUDP(t *testing.T) {
	ctx := context.Background()
	opt := &Options{dialContext: new(net.Dialer).DialContext}

	if testsuite.EnableIPv4() {
		msg, err := dialUDP(ctx, "8.8.8.8:53", testDNSMessage, opt)
		require.NoError(t, err)
		result, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("UDP (IPv4 DNS Server):", result)
	}

	if testsuite.EnableIPv6() {
		msg, err := dialUDP(ctx, "[2606:4700:4700::1001]:53", testDNSMessage, opt)
		require.NoError(t, err)
		result, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("UDP (IPv6 DNS Server):", result)
	}

	// unknown network
	opt.Network = "foo network"
	_, err := dialUDP(ctx, "", nil, opt)
	require.Error(t, err)

	// no port
	opt.Network = "udp"
	_, err = dialUDP(ctx, "1.2.3.4", nil, opt)
	require.Error(t, err)

	// no response
	opt.Timeout = time.Second
	if testsuite.EnableIPv4() {
		_, err = dialUDP(ctx, "1.2.3.4:23421", nil, opt)
		require.EqualError(t, err, ErrNoConnection.Error())
	}
	if testsuite.EnableIPv6() {
		_, err = dialUDP(ctx, "[::1]:23421", nil, opt)
		require.EqualError(t, err, ErrNoConnection.Error())
	}
}

func TestDialTCP(t *testing.T) {
	ctx := context.Background()
	opt := &Options{dialContext: new(net.Dialer).DialContext}

	if testsuite.EnableIPv4() {
		msg, err := dialTCP(ctx, "8.8.8.8:53", testDNSMessage, opt)
		require.NoError(t, err)
		result, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("TCP (IPv4 DNS Server):", result)
	}

	if testsuite.EnableIPv6() {
		msg, err := dialTCP(ctx, "[2606:4700:4700::1001]:53", testDNSMessage, opt)
		require.NoError(t, err)
		result, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("TCP (IPv6 DNS Server):", result)
	}

	// unknown network
	opt.Network = "foo network"
	_, err := dialTCP(ctx, "", nil, opt)
	require.Error(t, err)

	// no port
	opt.Network = "tcp"
	_, err = dialTCP(ctx, "1.2.3.4", nil, opt)
	require.Error(t, err)
}

func TestDialDoT(t *testing.T) {
	ctx := context.Background()
	opt := &Options{dialContext: new(net.Dialer).DialContext}

	if testsuite.EnableIPv4() {
		const (
			dnsServerIPV4 = "8.8.8.8:853"
			dnsDomainIPv4 = "dns.google:853|8.8.8.8,8.8.4.4"
		)
		// IP mode
		msg, err := dialDoT(ctx, dnsServerIPV4, testDNSMessage, opt)
		require.NoError(t, err)
		result, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("DoT-IP (IPv4 DNS Server):", result)
		// domain mode
		msg, err = dialDoT(ctx, dnsDomainIPv4, testDNSMessage, opt)
		require.NoError(t, err)
		result, err = unpackMessage(msg)
		require.NoError(t, err)
		t.Log("DoT-Domain (IPv4 DNS Server):", result)
	}

	if testsuite.EnableIPv6() {
		const (
			dnsServerIPv6 = "[2606:4700:4700::64]:853"
			dnsDomainIPv6 = "cloudflare-dns.com:853|2606:4700:4700::1111,2606:4700:4700::1001"
		)
		// IP mode
		msg, err := dialDoT(ctx, dnsServerIPv6, testDNSMessage, opt)
		require.NoError(t, err)
		result, err := unpackMessage(msg)
		require.NoError(t, err)
		t.Log("DoT-IP (IPv6 DNS Server):", result)
		// domain mode
		msg, err = dialDoT(ctx, dnsDomainIPv6, testDNSMessage, opt)
		require.NoError(t, err)
		result, err = unpackMessage(msg)
		require.NoError(t, err)
		t.Log("DoT-Domain (IPv6 DNS Server):", result)
	}

	// unknown network
	opt.Network = "foo network"
	_, err := dialDoT(ctx, "", nil, opt)
	require.Error(t, err)

	// no port(ip mode)
	opt.Network = "tcp"
	_, err = dialDoT(ctx, "1.2.3.4", nil, opt)
	require.Error(t, err)

	// failed to dial
	_, err = dialDoT(ctx, "127.0.0.1:888", nil, opt)
	require.Error(t, err)

	// error ip(domain mode)
	_, err = dialDoT(ctx, "dns.google:853|127.0.0.1", nil, opt)
	require.EqualError(t, err, ErrNoConnection.Error())

	// no port(domain mode)
	_, err = dialDoT(ctx, "dns.google|1.2.3.235", nil, opt)
	require.Error(t, err)

	// invalid config
	cfg := "asd:153|xxx|xxx"
	_, err = dialDoT(ctx, cfg, nil, opt)
	require.Errorf(t, err, "invalid address: %s", cfg)
}

func TestDialDoH(t *testing.T) {
	const dnsServer = "https://cloudflare-dns.com/dns-query"
	ctx := context.Background()
	opt := &Options{transport: new(http.Transport)}

	// get
	resp, err := dialDoH(ctx, dnsServer, testDNSMessage, opt)
	require.NoError(t, err)
	result, err := unpackMessage(resp)
	require.NoError(t, err)
	t.Log("DoH GET:", result)

	// post
	url := dnsServer + "#" + strings.Repeat("a", 2048)
	resp, err = dialDoH(ctx, url, testDNSMessage, opt)
	require.NoError(t, err)
	result, err = unpackMessage(resp)
	require.NoError(t, err)
	t.Log("DoH POST:", result)

	// invalid doh server
	_, err = dialDoH(ctx, "foo\n", testDNSMessage, opt)
	require.Error(t, err)
	url = "foo\n" + "#" + strings.Repeat("a", 2048)
	_, err = dialDoH(ctx, url, testDNSMessage, opt)
	require.Error(t, err)

	// unreachable doh server
	_, err = dialDoH(ctx, "https://1.2.3.4/", testDNSMessage, opt)
	require.Error(t, err)
}

func TestFailedToSendMessage(t *testing.T) {
	// failed to write message
	server, client := net.Pipe()
	_ = server.Close()
	_, err := sendMessage(client, testDNSMessage, time.Second)
	require.Error(t, err)

	// failed to read response size
	server, client = net.Pipe()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := io.ReadFull(server, make([]byte, headerSize+len(testDNSMessage)))
		require.NoError(t, err)
		_ = server.Close()
	}()
	_, err = sendMessage(client, testDNSMessage, time.Second)
	require.Error(t, err)
	wg.Wait()

	// failed to read response
	server, client = net.Pipe()
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := io.ReadFull(server, make([]byte, headerSize+len(testDNSMessage)))
		require.NoError(t, err)
		_, _ = server.Write(convert.Uint16ToBytes(4))
		_ = server.Close()
	}()
	_, err = sendMessage(client, testDNSMessage, time.Second)
	require.Error(t, err)
	wg.Wait()
}
