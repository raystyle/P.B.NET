package proxy

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
)

// Balance implemented client.
type Balance struct {
	tag     string
	clients []*Client // not nil

	flags   map[*Client]bool
	flagsMu sync.Mutex
}

// NewBalance is used to create a proxy client that with load balance.
func NewBalance(tag string, clients ...*Client) (*Balance, error) {
	if tag == "" {
		return nil, errors.New("empty proxy balance tag")
	}
	l := len(clients)
	if l == 0 {
		return nil, errors.New("proxy balance need at least one proxy client")
	}
	// initialize flags
	flags := make(map[*Client]bool)
	for i := 0; i < l; i++ {
		flags[clients[i]] = false
	}
	return &Balance{
		tag:     tag,
		clients: clients,
		flags:   flags,
	}, nil
}

// GetAndSelectNext is used to provide chain, next.client will not be *Balance.
func (b *Balance) GetAndSelectNext() *Client {
	next := b.selectNextProxyClient()
	if next.Mode == ModeBalance {
		next = next.client.(*Balance).GetAndSelectNext()
	}
	return next
}

func (b *Balance) selectNextProxyClient() *Client {
	b.flagsMu.Lock()
	defer b.flagsMu.Unlock()
	for {
		for client, used := range b.flags {
			if !used {
				b.flags[client] = true
				return client
			}
		}
		// reset all clients flag
		for client := range b.flags {
			b.flags[client] = false
		}
	}
}

// Dial is used to connect to address through selected proxy client.
func (b *Balance) Dial(network, address string) (net.Conn, error) {
	conn, err := b.GetAndSelectNext().Dial(network, address)
	if err != nil {
		const format = "dial: balance %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, b.tag, address)
	}
	return conn, nil
}

// DialContext is used to connect to address through selected proxy client with context.
func (b *Balance) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := b.GetAndSelectNext().DialContext(ctx, network, address)
	if err != nil {
		const format = "dial context: balance %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, b.tag, address)
	}
	return conn, nil
}

// DialTimeout is used to connect to address through selected proxy client with timeout.
func (b *Balance) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	conn, err := b.GetAndSelectNext().DialTimeout(network, address, timeout)
	if err != nil {
		const format = "dial timeout: balance %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, b.tag, address)
	}
	return conn, nil
}

// Connect is a padding function.
func (b *Balance) Connect(context.Context, net.Conn, string, string) (net.Conn, error) {
	return nil, errors.New("proxy balance doesn't support connect method")
}

// HTTP is used to set *http.Transport about proxy.
func (b *Balance) HTTP(t *http.Transport) {
	t.DialContext = b.DialContext
}

// Timeout is a padding function.
func (b *Balance) Timeout() time.Duration {
	return 0
}

// Server is a padding function.
func (b *Balance) Server() (string, string) {
	return "", ""
}

// Info is used to get the balance information, it will print all proxy client information.
func (b *Balance) Info() string {
	buf := new(bytes.Buffer)
	buf.WriteString("balance: ")
	buf.WriteString(b.tag)
	printClientsInfo(buf, b.clients)
	return buf.String()
}
