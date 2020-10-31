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

	"project/internal/nettool"
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

	client
}

// Server is the proxy server.
type Server struct {
	Tag     string `toml:"tag"`
	Mode    string `toml:"mode"`
	Options string `toml:"options"`

	// secondary proxy
	DialContext nettool.DialContext `toml:"-" msgpack:"-"`

	now      func() time.Time
	createAt time.Time
	serveAt  []time.Time
	rwm      sync.RWMutex

	server
}

func (srv *Server) addServeAt() {
	srv.rwm.Lock()
	defer srv.rwm.Unlock()
	srv.serveAt = append(srv.serveAt, srv.now())
}

// ListenAndServe is used to listen a listener and serve.
func (srv *Server) ListenAndServe(network, address string) error {
	srv.addServeAt()
	return srv.server.ListenAndServe(network, address)
}

// Serve accept incoming connections on the listener.
func (srv *Server) Serve(listener net.Listener) error {
	srv.addServeAt()
	return srv.server.Serve(listener)
}

// CreateAt is used get proxy server create time.
func (srv *Server) CreateAt() time.Time {
	return srv.createAt
}

// ServeAt is used get proxy server serve time.
func (srv *Server) ServeAt() []time.Time {
	srv.rwm.RLock()
	defer srv.rwm.RUnlock()
	return srv.serveAt
}

// balance: general
// 1. socks4-c:  socks4, server: tcp 127.0.0.1:6321
// 2. http-c:    http://admin4:1234564@127.0.0.1:6319
// 3. https-c:   https://admin5:1234565@127.0.0.1:6323
// 4. socks5-c:  socks5, server: tcp 127.0.0.1:6320, auth: admin1:1234561
// 5. socks4a-c: socks4a, server: tcp 127.0.0.1:6322, user id: admin2
//
// if balance in balance, chain in balance or ...
//
// balance: final-balance
// 1. balance-1:
//      mode: balance
//      1. socks4-c:  socks4, server: tcp 127.0.1.3:6760, user id: admin3
//      2. http-c:    http://admin4:1234564@127.0.1.4:6761
//      3. https-c:   https://admin5:1234565@127.0.1.5:6762
//      4. socks5-c:  socks5, server: tcp 127.0.1.1:6758, auth: admin1:1234561
//      5. socks4a-c: socks4a, server: tcp 127.0.1.2:6759, user id: admin2
// 2. balance-2:
//      mode: balance
//      1. socks4a-c: socks4a, server: tcp 127.0.1.2:6759, user id: admin2
//      2. socks4-c:  socks4, server: tcp 127.0.1.3:6760, user id: admin3
//      3. http-c:    http://admin4:1234564@127.0.1.4:6761
//      4. https-c:   https://admin5:1234565@127.0.1.5:6762
//      5. socks5-c:  socks5, server: tcp 127.0.1.1:6758, auth: admin1:1234561
// 3. balance-3:
//      mode: balance
//      1. socks4a-c: socks4a, server: tcp 127.0.1.2:6759, user id: admin2
//      2. socks4-c:  socks4, server: tcp 127.0.1.3:6760, user id: admin3
//      3. http-c:    http://admin4:1234564@127.0.1.4:6761
//      4. https-c:   https://admin5:1234565@127.0.1.5:6762
//      5. socks5-c:  socks5, server: tcp 127.0.1.1:6758, auth: admin1:1234561
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
