package proxy

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// Balance implement Client
type Balance struct {
	tag     string
	clients []*Client // must > 0
	length  int

	current int
	count   map[*Client]int
	next    *Client
	mutex   sync.Mutex
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
	count := make(map[*Client]int)
	for i := 0; i < l; i++ {
		count[c[i]] = 0
	}
	b := Balance{
		tag:     tag,
		clients: c,
		length:  l,
		current: 1,
		count:   count,
	}
	b.setNext()
	return &b, nil
}

func (b *Balance) setNext() {
	b.mutex.Lock()
	for {
		for client, count := range b.count {
			if count < b.current {
				b.count[client] += 1
				if b.count[client] == math.MaxUint8 {
					b.count[client] = 0
				}
				b.next = client
				b.mutex.Unlock()
				return
			}
		}
		// if all > current, current add 1
		if b.current == math.MaxUint8 {
			b.current = 1
		} else {
			b.current += 1
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

	return next
}

func (b *Balance) Dial(network, address string) (net.Conn, error) {
	next := b.getNext()
	b.setNext()
	conn, err := next.Dial(network, address)
	if err != nil {
		return nil, errors.WithMessagef(err, "balance %s Dial:", b.tag)
	}
	return conn, nil
}

func (b *Balance) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	defer b.setNext()
	conn, err := b.getNext().DialContext(ctx, network, address)
	if err != nil {
		return nil, errors.WithMessagef(err, "balance %s DialContext:", b.tag)
	}
	return conn, nil
}

func (b *Balance) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	defer b.setNext()
	conn, err := b.getNext().DialTimeout(network, address, timeout)
	if err != nil {
		return nil, errors.WithMessagef(err, "balance %s DialTimeout:", b.tag)
	}
	return conn, nil
}

// Connect is used to connect target, for Chain
// Connect must with Timeout() and Server() at the same time
// or Connect maybe failed because incorrect conn
func (b *Balance) Connect(conn net.Conn, network, address string) (net.Conn, error) {
	defer b.setNext()
	pConn, err := b.getNext().Connect(conn, network, address)
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
// http://admin:123456@127.0.0.1:8080
// socks5 tcp 127.0.0.1:1080 admin 123456
// socks4a tcp 127.0.0.1:1081
func (b *Balance) Info() string {
	buf := new(bytes.Buffer)
	buf.WriteString("balance: ")
	buf.WriteString(b.tag)
	for i := 0; i < b.length; i++ {
		_, _ = fmt.Fprintf(buf, "\n%s", b.clients[i].Info())
	}
	return buf.String()
}
