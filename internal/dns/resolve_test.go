package dns

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"

	"project/internal/convert"
	"project/internal/random"
	"project/internal/testsuite"
	"project/internal/testsuite/testcert"
)

const testDomain = "cloudflare-dns.com"

func TestCustomResolve(t *testing.T) {
	ctx := context.Background()
	opts := &Options{
		Type:        TypeIPv4,
		dialContext: new(net.Dialer).DialContext,
		transport:   new(http.Transport),
	}
	opts.TLSConfig.CertPool = testcert.CertPool(t)

	if testsuite.IPv4Enabled {
		const (
			udpServer = "8.8.8.8:53"
			tcpServer = "1.0.0.1:53"
			tlsIP     = "1.1.1.1:853"
			tlsDomain = "cloudflare-dns.com:853|1.1.1.1,1.0.0.1"
		)

		t.Run("IPv4 UDP", func(t *testing.T) {
			opts.Method = MethodUDP

			result, err := resolve(ctx, udpServer, testDomain, opts)
			require.NoError(t, err)

			t.Log("UDP IPv4:", result)
		})

		t.Run("IPv4 TCP", func(t *testing.T) {
			opts.Method = MethodTCP

			result, err := resolve(ctx, tcpServer, testDomain, opts)
			require.NoError(t, err)

			t.Log("TCP IPv4:", result)
		})

		t.Run("IPv4 DoT IP mode", func(t *testing.T) {
			opts.Method = MethodDoT

			result, err := resolve(ctx, tlsIP, testDomain, opts)
			require.NoError(t, err)

			t.Log("DOT-IP IPv4:", result)
		})

		t.Run("IPv4 DoT domain mode", func(t *testing.T) {
			opts.Method = MethodDoT

			result, err := resolve(ctx, tlsDomain, testDomain, opts)
			require.NoError(t, err)

			t.Log("DOT-Domain IPv4:", result)
		})
	}

	if testsuite.IPv6Enabled {
		const (
			udpServer = "[2606:4700:4700::1111]:53"
			tcpServer = "[2606:4700:4700::1001]:53"
			TLSip     = "[2606:4700:4700::64]:853"
			TLSDomain = "cloudflare-dns.com:853|2606:4700:4700::1111,2606:4700:4700::1001"
		)

		t.Run("IPv6 UDP", func(t *testing.T) {
			opts.Method = MethodUDP

			result, err := resolve(ctx, udpServer, testDomain, opts)
			require.NoError(t, err)

			t.Log("UDP IPv6:", result)
		})

		t.Run("IPv6 TCP", func(t *testing.T) {
			opts.Method = MethodTCP

			result, err := resolve(ctx, tcpServer, testDomain, opts)
			require.NoError(t, err)

			t.Log("TCP IPv6:", result)
		})

		t.Run("IPv6 DoT IP mode", func(t *testing.T) {
			opts.Method = MethodDoT

			result, err := resolve(ctx, TLSip, testDomain, opts)
			require.NoError(t, err)

			t.Log("DOT-IP IPv6:", result)
		})

		t.Run("IPv6 DoT domain mode", func(t *testing.T) {
			opts.Method = MethodDoT

			result, err := resolve(ctx, TLSDomain, testDomain, opts)
			require.NoError(t, err)

			t.Log("DOT-Domain IPv6:", result)
		})
	}

	t.Run("DoH", func(t *testing.T) {
		const dnsDOH = "https://cloudflare-dns.com/dns-query"
		opts.Method = MethodDoH

		result, err := resolve(ctx, dnsDOH, testDomain, opts)
		require.NoError(t, err)

		t.Log("DOH:", result)
	})

	t.Run("failed to resolve", func(t *testing.T) {
		opts.Timeout = time.Second
		opts.Method = MethodUDP

		result, err := resolve(ctx, "0.0.0.0:1", testDomain, opts)
		require.Error(t, err)

		require.Empty(t, result)
	})
}

var (
	testQueryID    = uint16(random.Int(65536))
	testDNSMessage = packMessage(dnsmessage.TypeA, testDomain, testQueryID)
)

func TestDialUDP(t *testing.T) {
	ctx := context.Background()
	opts := &Options{dialContext: new(net.Dialer).DialContext}

	if testsuite.IPv4Enabled {
		t.Run("IPv4", func(t *testing.T) {
			msg, err := dialUDP(ctx, "8.8.8.8:53", testDNSMessage, opts)
			require.NoError(t, err)

			result, err := unpackMessage(msg, testDomain, testQueryID)
			require.NoError(t, err)

			t.Log("UDP (IPv4 DNS Server):", result)
		})
	}

	if testsuite.IPv6Enabled {
		t.Run("IPv6", func(t *testing.T) {
			msg, err := dialUDP(ctx, "[2606:4700:4700::1001]:53", testDNSMessage, opts)
			require.NoError(t, err)

			result, err := unpackMessage(msg, testDomain, testQueryID)
			require.NoError(t, err)

			t.Log("UDP (IPv6 DNS Server):", result)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		opts.Network = "foo network"
		_, err := dialUDP(ctx, "", nil, opts)
		require.Error(t, err)
	})

	t.Run("no port", func(t *testing.T) {
		opts.Network = "udp"
		_, err := dialUDP(ctx, "1.2.3.4", nil, opts)
		require.Error(t, err)
	})

	t.Run("no response", func(t *testing.T) {
		opts.Timeout = time.Second
		if testsuite.IPv4Enabled {
			t.Run("IPv4", func(t *testing.T) {
				_, err := dialUDP(ctx, "1.2.3.4:23421", nil, opts)
				require.EqualError(t, err, ErrNoConnection.Error())
			})
		}
		if testsuite.IPv6Enabled {
			t.Run("IPv6", func(t *testing.T) {
				_, err := dialUDP(ctx, "[::1]:23421", nil, opts)
				require.EqualError(t, err, ErrNoConnection.Error())
			})
		}
	})
}

func TestDialTCP(t *testing.T) {
	ctx := context.Background()
	opts := &Options{dialContext: new(net.Dialer).DialContext}

	if testsuite.IPv4Enabled {
		t.Run("IPv4", func(t *testing.T) {
			msg, err := dialTCP(ctx, "8.8.8.8:53", testDNSMessage, opts)
			require.NoError(t, err)

			result, err := unpackMessage(msg, testDomain, testQueryID)
			require.NoError(t, err)

			t.Log("TCP (IPv4 DNS Server):", result)
		})
	}

	if testsuite.IPv6Enabled {
		t.Run("IPv6", func(t *testing.T) {
			msg, err := dialTCP(ctx, "[2606:4700:4700::1001]:53", testDNSMessage, opts)
			require.NoError(t, err)

			result, err := unpackMessage(msg, testDomain, testQueryID)
			require.NoError(t, err)

			t.Log("TCP (IPv6 DNS Server):", result)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		opts.Network = "foo network"
		_, err := dialTCP(ctx, "", nil, opts)
		require.Error(t, err)
	})

	t.Run("no port", func(t *testing.T) {
		opts.Network = "tcp"
		_, err := dialTCP(ctx, "1.2.3.4", nil, opts)
		require.Error(t, err)
	})
}

func TestDialDoT(t *testing.T) {
	ctx := context.Background()
	opts := &Options{dialContext: new(net.Dialer).DialContext}
	opts.TLSConfig.CertPool = testcert.CertPool(t)

	if testsuite.IPv4Enabled {
		t.Run("IPv4", func(t *testing.T) {
			const (
				dnsServerIPV4 = "1.1.1.1:853"
				dnsDomainIPv4 = "cloudflare-dns.com:853|1.1.1.1,1.0.0.1"
			)

			t.Run("IP mode", func(t *testing.T) {
				msg, err := dialDoT(ctx, dnsServerIPV4, testDNSMessage, opts)
				require.NoError(t, err)

				result, err := unpackMessage(msg, testDomain, testQueryID)
				require.NoError(t, err)

				t.Log("DoT-IP (IPv4 DNS Server):", result)
			})

			t.Run("domain mode", func(t *testing.T) {
				msg, err := dialDoT(ctx, dnsDomainIPv4, testDNSMessage, opts)
				require.NoError(t, err)

				result, err := unpackMessage(msg, testDomain, testQueryID)
				require.NoError(t, err)

				t.Log("DoT-Domain (IPv4 DNS Server):", result)
			})
		})
	}

	if testsuite.IPv6Enabled {
		t.Run("IPv6", func(t *testing.T) {
			const (
				dnsServerIPv6 = "[2606:4700:4700::64]:853"
				dnsDomainIPv6 = "cloudflare-dns.com:853|2606:4700:4700::1111,2606:4700:4700::1001"
			)

			t.Run("IP mode", func(t *testing.T) {
				msg, err := dialDoT(ctx, dnsServerIPv6, testDNSMessage, opts)
				require.NoError(t, err)

				result, err := unpackMessage(msg, testDomain, testQueryID)
				require.NoError(t, err)

				t.Log("DoT-IP (IPv6 DNS Server):", result)
			})

			t.Run("domain mode", func(t *testing.T) {
				msg, err := dialDoT(ctx, dnsDomainIPv6, testDNSMessage, opts)
				require.NoError(t, err)

				result, err := unpackMessage(msg, testDomain, testQueryID)
				require.NoError(t, err)

				t.Log("DoT-Domain (IPv6 DNS Server):", result)
			})
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		opts.Network = "foo network"
		_, err := dialDoT(ctx, "", nil, opts)
		require.Error(t, err)
	})

	t.Run("no port(ip mode)", func(t *testing.T) {
		opts.Network = "tcp"
		_, err := dialDoT(ctx, "1.2.3.4", nil, opts)
		require.Error(t, err)

	})

	t.Run("failed to dial", func(t *testing.T) {
		_, err := dialDoT(ctx, "127.0.0.1:888", nil, opts)
		require.Error(t, err)
	})

	t.Run("error ip(domain mode)", func(t *testing.T) {
		_, err := dialDoT(ctx, "dns.google:853|127.0.0.1", nil, opts)
		require.Error(t, err)
	})

	t.Run("no port(domain mode)", func(t *testing.T) {
		_, err := dialDoT(ctx, "dns.google|1.2.3.235", nil, opts)
		require.Error(t, err)
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := "asd:153|xxx|xxx"
		_, err := dialDoT(ctx, cfg, nil, opts)
		require.EqualError(t, err, "invalid config: "+cfg)
	})

	t.Run("invalid TLS config", func(t *testing.T) {
		opts.TLSConfig.RootCAs = []string{"foo ca"}
		_, err := dialDoT(ctx, "127.0.0.1:888", nil, opts)
		require.Error(t, err)
	})
}

func TestDialDoH(t *testing.T) {
	const dnsServer = "https://cloudflare-dns.com/dns-query"
	ctx := context.Background()
	opts := new(Options)
	opts.Transport.TLSClientConfig.CertPool = testcert.CertPool(t)
	var err error
	opts.transport, err = opts.Transport.Apply()
	require.NoError(t, err)

	t.Run("GET", func(t *testing.T) {
		resp, err := dialDoH(ctx, dnsServer, testDNSMessage, opts)
		require.NoError(t, err)

		result, err := unpackMessage(resp, testDomain, testQueryID)
		require.NoError(t, err)

		t.Log("DoH GET:", result)
	})

	t.Run("POST", func(t *testing.T) {
		url := dnsServer + "#" + strings.Repeat("a", 2048)
		resp, err := dialDoH(ctx, url, testDNSMessage, opts)
		require.NoError(t, err)

		result, err := unpackMessage(resp, testDomain, testQueryID)
		require.NoError(t, err)

		t.Log("DoH POST:", result)
	})

	t.Run("invalid DOH server", func(t *testing.T) {
		_, err := dialDoH(ctx, "foo\n", testDNSMessage, opts)
		require.Error(t, err)

		url := "foo\n" + "#" + strings.Repeat("a", 2048)
		_, err = dialDoH(ctx, url, testDNSMessage, opts)
		require.Error(t, err)
	})

	t.Run("unreachable DOH server", func(t *testing.T) {
		opts.Timeout = time.Second
		_, err := dialDoH(ctx, "https://1.2.3.4/", testDNSMessage, opts)
		require.Error(t, err)
	})
}

func TestSendMessage(t *testing.T) {
	t.Run("failed to write message", func(t *testing.T) {
		conn := testsuite.NewMockConnWithWriteError()

		_, err := sendMessage(conn, testDNSMessage, time.Second)
		require.Error(t, err)
	})

	t.Run("failed to read response size", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(server net.Conn) {
				buf := make([]byte, headerSize+len(testDNSMessage))
				_, err := io.ReadFull(server, buf)
				require.NoError(t, err)

				err = server.Close()
				require.NoError(t, err)
			},
			func(client net.Conn) {
				_, err := sendMessage(client, testDNSMessage, time.Second)
				require.Error(t, err)
			},
		)
	})

	t.Run("failed to read response", func(t *testing.T) {
		testsuite.PipeWithReaderWriter(t,
			func(server net.Conn) {
				buf := make([]byte, headerSize+len(testDNSMessage))
				_, err := io.ReadFull(server, buf)
				require.NoError(t, err)

				_, err = server.Write(convert.BEUint16ToBytes(4))
				require.NoError(t, err)

				err = server.Close()
				require.NoError(t, err)
			},
			func(client net.Conn) {
				_, err := sendMessage(client, testDNSMessage, time.Second)
				require.Error(t, err)
			},
		)
	})
}
