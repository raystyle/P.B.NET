package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
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
	b.setNext()
	return &b, nil
}

func (b *Balance) setNext() {
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

func (b *Balance) getNext() *Client {
	b.mutex.Lock()
	client := b.next
	b.mutex.Unlock()
	return client
}

func (b *Balance) getAndSetNext() *Client {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	next := b.next
	b.setNext()
	return next
}

func (b *Balance) Dial(network, address string) (net.Conn, error) {
	conn, err := b.getAndSetNext().Dial(network, address)
	if err != nil {
		return nil, errors.WithMessagef(err, "balance %s Dial:", b.tag)
	}
	return conn, nil
}

func (b *Balance) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := b.getAndSetNext().DialContext(ctx, network, address)
	if err != nil {
		return nil, errors.WithMessagef(err, "balance %s DialContext:", b.tag)
	}
	return conn, nil
}

func (b *Balance) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	conn, err := b.getAndSetNext().DialTimeout(network, address, timeout)
	if err != nil {
		return nil, errors.WithMessagef(err, "balance %s DialTimeout:", b.tag)
	}
	return conn, nil
}

// Connect is used to connect target, for Chain
// Connect must with Timeout() and Server() at the same time
// or Connect maybe failed because incorrect conn
func (b *Balance) Connect(conn net.Conn, network, address string) (net.Conn, error) {
	pConn, err := b.getAndSetNext().Connect(conn, network, address)
	if err != nil {
		return nil, errors.WithMessagef(err, "balance %s Connect:", b.tag)
	}
	return pConn, nil
}

func (b *Balance) HTTP(t *http.Transport) {
	t.DialContext = b.DialContext
}

// if you want wo use balance in proxy chain
// must add lock for Timeout() and Server()
func (b *Balance) Timeout() time.Duration {
	return b.getNext().Timeout()
}

func (b *Balance) Server() (string, string) {
	return b.getNext().Server()
}

// Info is used to get the balance info
// balance: tag
// 1. tag-a: http://admin:123456@127.0.0.1:8080
// 2. tag-b: socks5 tcp 127.0.0.1:1080 admin 123456
// 3. tag-c: socks4a tcp 127.0.0.1:1081
func (b *Balance) Info() string {
	buf := new(bytes.Buffer)
	buf.WriteString("balance: ")
	buf.WriteString(b.tag)
	for i := 0; i < b.length; i++ {
		c := b.clients[i]
		_, _ = fmt.Fprintf(buf, "\n%d. %s: %s", i+1, c.Tag, c.Info())
	}
	return buf.String()
}
