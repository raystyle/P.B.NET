package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// supported modes
const (
	// basic
	ModeSocks5  = "socks5"
	ModeSocks4a = "socks4a"
	ModeSocks4  = "socks4"
	ModeHTTP    = "http"
	ModeHTTPS   = "https"

	// combine proxy client, include basic proxy client
	ModeChain   = "chain"
	ModeBalance = "balance"

	// reserve proxy client in Pool
	ModeDirect = "direct"
)

// EmptyTag is a reserve tag that delete "-" in tag,
// "https proxy- " -> "https proxy", it is used to tool/proxy.
const EmptyTag = " "

type client interface {
	Dial(network, address string) (net.Conn, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
	Connect(ctx context.Context, conn net.Conn, network, address string) (net.Conn, error)
	HTTP(t *http.Transport)
	Timeout() time.Duration
	Server() (network string, address string)
	Info() string
}

type server interface {
	ListenAndServe(network, address string) error
	Serve(listener net.Listener) error
	Addresses() []net.Addr
	Info() string
	Close() error
}

// Client is the proxy client.
type Client struct {
	Tag     string `toml:"tag"`
	Mode    string `toml:"mode"`
	Network string `toml:"network"`
	Address string `toml:"address"`
	Options string `toml:"options"`

	client `check:"-"`
}

// Server is the proxy server.
type Server struct {
	Tag     string `toml:"tag"`
	Mode    string `toml:"mode"`
	Options string `toml:"options"`
	// secondary proxy
	DialContext func(ctx context.Context, network, address string) (net.Conn, error) `toml:"-"`

	server `check:"-"`

	now      func() time.Time
	createAt time.Time
	serveAt  []time.Time
	rwm      sync.RWMutex
}

func (s *Server) addServeAt() {
	s.rwm.Lock()
	defer s.rwm.Unlock()
	s.serveAt = append(s.serveAt, s.now())
}

// ListenAndServe is used to listen a listener and serve.
func (s *Server) ListenAndServe(network, address string) error {
	s.addServeAt()
	return s.server.ListenAndServe(network, address)
}

// Serve accept incoming connections on the listener.
func (s *Server) Serve(listener net.Listener) error {
	s.addServeAt()
	return s.server.Serve(listener)
}

// CreateAt is used get proxy server create time.
func (s *Server) CreateAt() time.Time {
	return s.createAt
}

// ServeAt is used get proxy server serve time.
func (s *Server) ServeAt() []time.Time {
	s.rwm.RLock()
	defer s.rwm.RUnlock()
	return s.serveAt
}

// balance: general
// 1. socks4-c:  socks4  tcp 127.0.0.1:6321 user id: admin3
// 2. http-c:    http://admin4:1234564@127.0.0.1:6319
// 3. https-c:   https://admin5:1234565@127.0.0.1:6323
// 4. socks5-c:  socks5  tcp 127.0.0.1:6320 auth: admin1:1234561
// 5. socks4a-c: socks4a tcp 127.0.0.1:6322 user id: admin2
//
// if balance in balance, chain in balance or ...
//
// balance: final-balance
// 1. balance-1:
//     mode: balance
//     1. socks4-c:  socks4  tcp 127.0.0.1:6321 user id: admin3
//     2. http-c:    http://admin4:1234564@127.0.0.1:6319
//     3. https-c:   https://admin5:1234565@127.0.0.1:6323
//     4. socks5-c:  socks5  tcp 127.0.0.1:6320 auth: admin1:1234561
//     5. socks4a-c: socks4a tcp 127.0.0.1:6322 user id: admin2
// 2. balance-2:
//     mode: balance
//     1. socks5-c:  socks5  tcp 127.0.0.1:6320 auth: admin1:1234561
//     2. socks4a-c: socks4a tcp 127.0.0.1:6322 user id: admin2
//     3. socks4-c:  socks4  tcp 127.0.0.1:6321 user id: admin3
//     4. http-c:    http://admin4:1234564@127.0.0.1:6319
//     5. https-c:   https://admin5:1234565@127.0.0.1:6323
// 3. balance-3:
//     mode: balance
//     1. socks4-c:  socks4  tcp 127.0.0.1:6321 user id: admin3
//     2. http-c:    http://admin4:1234564@127.0.0.1:6319
//     3. https-c:   https://admin5:1234565@127.0.0.1:6323
//     4. socks5-c:  socks5  tcp 127.0.0.1:6320 auth: admin1:1234561
//     5. socks4a-c: socks4a tcp 127.0.0.1:6322 user id: admin2
// 4. http-c:  http://admin4:1234564@127.0.0.1:6319
// 5. https-c: https://admin5:1234565@127.0.0.1:6323
func printClientsInfo(buf *bytes.Buffer, clients []*Client) {
	// get max tag length
	var maxTagLen int
	for _, client := range clients {
		if client.Mode == ModeBalance || client.Mode == ModeChain {
			continue
		}
		l := len(client.Tag)
		if l > maxTagLen {
			maxTagLen = l
		}
	}
	l := strconv.Itoa(maxTagLen + 1) // add ":"
	format := "\n%d. %-" + l + "s %s"
	for i, client := range clients {
		if client.Mode == ModeBalance || client.Mode == ModeChain {
			info := new(bytes.Buffer)
			_, _ = fmt.Fprintf(info, "\n     mode: %s", client.Mode)
			scanner := bufio.NewScanner(strings.NewReader(client.Info()))
			scanner.Scan() // skip mode + tag
			for scanner.Scan() {
				info.WriteString("\n     ") // 5 spaces
				info.Write(scanner.Bytes())
			}
			_, _ = fmt.Fprintf(buf, "\n%d. %s %s", i+1, client.Tag+":", info)
		} else {
			_, _ = fmt.Fprintf(buf, format, i+1, client.Tag+":", client.Info())
		}
	}
}
