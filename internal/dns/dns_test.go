package dns

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	dns_address         = "8.8.8.8:53"
	dns_tls_domain_mode = "dns.google:853|8.8.8.8,8.8.4.4"
	dns_doh             = "https://cloudflare-dns.com/dns-query"
	domain              = "ipv6.baidu.com"
	domain_punycode     = "错的是.世界"

	// dns_doh          = "https://cloudflare-dns.com/dns-query"
	// dns_doh          = "https://mozilla.cloudflare-dns.com/dns-query"
	// dns_doh          = "https://dns.rubyfish.cn/dns-query" bug?
)

func Test_Resolve(t *testing.T) {
	// udp
	ip_list, err := Resolve(dns_address, domain, nil)
	require.Nil(t, err, err)
	t.Log("UDP IPv4:", ip_list)
	// punycode
	ip_list, err = Resolve(dns_address, domain_punycode, nil)
	require.Nil(t, err, err)
	t.Log("UDP IPv4 punycode:", ip_list)
	// tcp
	opt := &Options{
		Method:  TCP,
		Type:    IPV6,
		Timeout: 15 * time.Second,
	}
	ip_list, err = Resolve(dns_address, domain, opt)
	require.Nil(t, err, err)
	t.Log("TCP IPv6:", ip_list)
	// tls
	opt.Method = TLS
	opt.Type = IPV4
	ip_list, err = Resolve(dns_tls_domain_mode, domain, opt)
	require.Nil(t, err, err)
	t.Log("TLS IPv4:", ip_list)
	// doh
	opt.Method = DOH
	opt.Timeout = time.Minute
	ip_list, err = Resolve(dns_doh, domain, opt)
	require.Nil(t, err, err)
	t.Log("DOH IPv4:", ip_list)
	// is ip
	_, err = Resolve(dns_address, "8.8.8.8", opt)
	require.Nil(t, err, err)
	// not domain
	_, err = Resolve(dns_address, "asdasdad-", opt)
	require.Equal(t, err, ERR_INVALID_DOMAIN_NAME, err)
	// invalid Type
	opt.Type = "10"
	_, err = Resolve(dns_address, domain, opt)
	require.Equal(t, err, ERR_INVALID_TYPE, err)
	// invalid method
	opt.Type = IPV4
	opt.Method = "asdasd"
	_, err = Resolve(dns_address, domain, opt)
	require.Equal(t, err, ERR_UNKNOWN_METHOD, err)
	// dial failed
	opt.Network = "udp"
	opt.Method = UDP
	opt.Timeout = time.Millisecond * 500
	_, err = Resolve("8.8.8.8:153", domain, opt)
	require.NotNil(t, err)
}

func Test_is_domain(t *testing.T) {
	require.True(t, Is_Domain("asd.com"))
	require.True(t, Is_Domain("asd-asd.com"))
	// invalid domain
	require.False(t, Is_Domain(""))
	require.False(t, Is_Domain(string([]byte{255, 254, 12, 35})))
	require.False(t, Is_Domain("asdasdad-"))
	require.False(t, Is_Domain("asdasdad.-"))
	require.False(t, Is_Domain("asdasdad.."))
	require.False(t, Is_Domain(strings.Repeat("a", 64)+".com"))
}

func Test_resolve(t *testing.T) {
	r := func(response []byte) {
		ipv4_list, err := resolve(IPV4, response)
		require.NotNil(t, err)
		ipv6_list, err := resolve(IPV6, response)
		require.NotNil(t, err)
		require.Nil(t, append(ipv4_list, ipv6_list...))
	}
	r(nil)
	// no answer
	r([]byte{0, 0, 0, 0, 0, 0, 0, 0})
	// 2 answer
	r([]byte{0, 0, 0, 0, 0, 0, 0, 2})
	// invalid type
	buffer := bytes.Buffer{}
	buffer.Write([]byte{1, 1, 1, 1, 1, 1, 0, 1}) // 1 answer
	// padding
	buffer.Write([]byte{0, 0, 0, 0, 1, 0})
	buffer.Write([]byte{0, 0, 0, 0})
	buffer.Write([]byte{0, 0, 0, 0})
	b := buffer.Bytes()
	ipv4_list, err := resolve(IPV4, b)
	require.Nil(t, err)
	ipv6_list, err := resolve(IPV6, b)
	require.Nil(t, err)
	require.Nil(t, append(ipv4_list, ipv6_list...))
}

func Test_dial_udp(t *testing.T) {
	opt := &Options{
		Network: "udp",
		Timeout: time.Second * 1,
		Dial:    net.Dial,
	}
	question := pack_question(1, domain)
	b, err := dial_udp(dns_address, question, opt)
	ip_list, err := resolve(IPV4, b)
	require.Nil(t, err, err)
	t.Log("UDP IPv4:", ip_list)
	// no port
	_, err = dial_udp("1.2.3.4", question, opt)
	require.NotNil(t, err)
	// no response
	_, err = dial_udp("1.2.3.4:23421", question, opt)
	require.Equal(t, err, ERR_NO_CONNECTION, err)
}

func Test_dial_tcp(t *testing.T) {
	opt := &Options{
		Network: "tcp",
		Timeout: time.Second * 2,
		Dial:    net.Dial,
	}
	question := pack_question(1, domain)
	b, err := dial_tcp(dns_address, question, opt)
	require.Nil(t, err, err)
	ip_list, err := resolve(IPV4, b)
	require.Nil(t, err, err)
	t.Log("TCP IPv4:", ip_list)
	// no port
	_, err = dial_tcp("8.8.8.8", question, opt)
	require.NotNil(t, err)
}

func Test_dial_tls(t *testing.T) {
	opt := &Options{
		Network: "tcp",
		Timeout: time.Second * 2,
		Dial:    net.Dial,
	}
	question := pack_question(1, domain)
	b, err := dial_tls("dns.google:853|8.8.8.8", question, opt)
	require.Nil(t, err, err)
	ip_list, err := resolve(IPV4, b)
	require.Nil(t, err, err)
	t.Log("TLS domain IPv4:", ip_list)
	// ip mode
	b, err = dial_tls("1.1.1.1:853", question, opt)
	require.Nil(t, err, err)
	ip_list, err = resolve(IPV4, b)
	require.Nil(t, err, err)
	t.Log("TLS ip IPv4:", ip_list)
	// no port(ip mode)
	_, err = dial_tls("1.2.3.4", question, opt)
	require.NotNil(t, err)
	// dial failed
	_, err = dial_tls("127.0.0.1:888", question, opt)
	require.NotNil(t, err)
	// error ip(domain mode)
	_, err = dial_tls("dns.google:853|127.0.0.1", question, opt)
	require.Equal(t, err, ERR_NO_CONNECTION, err)
	// no port(domain mode)
	_, err = dial_tls("dns.google|1.2.3.235", question, opt)
	require.NotNil(t, err)
	// invalid config
	_, err = dial_tls("asd:153|asfasf|asfasf", question, opt)
	require.Equal(t, err, ERR_INVALID_TLS_CONFIG, err)
}

func Test_dial_https(t *testing.T) {
	opt := &Options{
		Timeout: time.Minute,
	}
	question := pack_question(1, domain)
	// get
	b, err := dial_https(dns_doh, question, opt)
	require.Nil(t, err, err)
	ip_list, err := resolve(IPV4, b)
	require.Nil(t, err, err)
	t.Log("DOH get IPv4:", ip_list)
	// post
	b, err = dial_https(dns_doh+"#"+strings.Repeat("a", 2048), question, opt)
	require.Nil(t, err, err)
	ip_list, err = resolve(IPV4, b)
	require.Nil(t, err, err)
	t.Log("DOH post IPv4:", ip_list)
	// invalid dohserver
	_, err = dial_https("asdsad\n", question, opt)
	require.NotNil(t, err, err)
	_, err = dial_https("asdsad\n"+"#"+strings.Repeat("a", 2048), question, opt)
	require.NotNil(t, err, err)
	// Do failed
	_, err = dial_https("http://asd.1dsa.asd", question, opt)
	require.NotNil(t, err, err)
}
