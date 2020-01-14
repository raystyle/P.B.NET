package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// Balance implement client
//
// <warning> Balance must be used independently, if you add a
// Balance to a proxy chain, you can't use this balance elsewhere
type Balance struct {
	tag     string
	clients []*Client // must > 0
	length  int

	flags map[*Client]bool
	next  *Client
	mutex sync.Mutex
}

// NewBalance is used to create a proxy client that with load balance
func NewBalance(tag string, clients ...*Client) (*Balance, error) {
	if tag == "" {
		return nil, errors.New("empty balance tag")
	}
	l := len(clients)
	if l == 0 {
		return nil, errors.New("balance need at least one proxy client")
	}
	c := make([]*Client, l)
	copy(c, clients)
	// init flags
	flags := make(map[*Client]bool)
	for i := 0; i < l; i++ {
		flags[c[i]] = false
	}
	b := Balance{
		tag:     tag,
		clients: c,
		length:  l,
		flags:   flags,
	}
	b.selectNextProxyClient()
	return &b, nil
}

func (b *Balance) selectNextProxyClient() {
	for {
		for client, used := range b.flags {
			if !used {
				b.flags[client] = true
				b.next = client
				return
			}
		}
		// reset all clients flag
		for client := range b.flags {
			b.flags[client] = false
		}
	}
}

func (b *Balance) getNextProxyClient() *Client {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.next
}

func (b *Balance) getAndSelect() *Client {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	next := b.next
	b.selectNextProxyClient()
	return next
}

// Dial is used to connect to address through selected proxy client
func (b *Balance) Dial(network, address string) (net.Conn, error) {
	conn, err := b.getAndSelect().Dial(network, address)
	return conn, errors.WithMessagef(err, "balance %s Dial", b.tag)
}

// DialContext is used to connect to address through selected proxy client with context
func (b *Balance) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := b.getAndSelect().DialContext(ctx, network, address)
	return conn, errors.WithMessagef(err, "balance %s DialContext", b.tag)
}

// DialTimeout is used to connect to address through selected proxy client with timeout
func (b *Balance) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	conn, err := b.getAndSelect().DialTimeout(network, address, timeout)
	return conn, errors.WithMessagef(err, "balance %s DialTimeout", b.tag)
}

// Connect is used to connect target
func (b *Balance) Connect(
	ctx context.Context,
	conn net.Conn,
	network string,
	address string,
) (net.Conn, error) {
	pConn, err := b.getAndSelect().Connect(ctx, conn, network, address)
	return pConn, errors.WithMessagef(err, "balance %s Connect", b.tag)
}

// HTTP is used to set *http.Transport about proxy
func (b *Balance) HTTP(t *http.Transport) {
	t.DialContext = b.DialContext
}

// Timeout is used to get the next proxy client timeout
func (b *Balance) Timeout() time.Duration {
	return b.getNextProxyClient().Timeout()
}

// Server is used to get the next proxy client related proxy server address
func (b *Balance) Server() (string, string) {
	return b.getNextProxyClient().Server()
}

// Info is used to get the balance info, it will print all proxy client info
//
// balance: tag
// 1. tag-a:  http://admin:123456@127.0.0.1:8080
// 2. tag-b:  https://admin:123456@127.0.0.1:8080
// 3. tag-c:  socks5 tcp 127.0.0.1:1080 admin 123456
// 4. tag-dd: socks4a tcp 127.0.0.1:1081
func (b *Balance) Info() string {
	buf := new(bytes.Buffer)
	buf.WriteString("balance: ")
	buf.WriteString(b.tag)
	// get max tag length
	var maxTagLen int
	for i := 0; i < b.length; i++ {
		l := len(b.clients[i].Tag)
		if l > maxTagLen {
			maxTagLen = l
		}
	}
	l := strconv.Itoa(maxTagLen + 1) // add ":"
	format := "\n%d. %-" + l + "s %s"
	for i := 0; i < b.length; i++ {
		c := b.clients[i]
		_, _ = fmt.Fprintf(buf, format, i+1, c.Tag+":", c.Info())
	}
	return buf.String()
}
