package proxy

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

const defaultDialTimeout = 30 * time.Second

// Chain implemented client.
type Chain struct {
	tag     string
	clients []*Client // not nil
	count   int       // len(clients)
}

// NewChain is used to create a proxy chain.
func NewChain(tag string, clients ...*Client) (*Chain, error) {
	if tag == "" {
		return nil, errors.New("empty proxy chain tag")
	}
	l := len(clients)
	if l == 0 {
		return nil, errors.New("proxy chain need at least one proxy client")
	}
	return &Chain{
		tag:     tag,
		clients: clients,
		count:   l,
	}, nil
}

// []*Client will not include ModeBalance or ModeChain.
// Can't  pre calculate clients, because it maybe changed if include Balance.
func (c *Chain) getProxyClients() []*Client {
	// if chain in clients, len(clients) will bigger than c.count
	clients := make([]*Client, 0, c.count)
	for _, client := range c.clients {
		switch client.Mode {
		case ModeBalance:
			c := client.client.(*Balance).GetAndSelectNext()
			if c.Mode == ModeChain {
				clients = append(clients, c.client.(*Chain).getProxyClients()...)
			} else {
				clients = append(clients, c)
			}
		case ModeChain:
			clients = append(clients, client.client.(*Chain).getProxyClients()...)
		default:
			clients = append(clients, client)
		}
	}
	return clients
}

// Dial is used to connect to address through proxy chain.
func (c *Chain) Dial(network, address string) (net.Conn, error) {
	clients := c.getProxyClients()
	fClient := clients[0]
	fTimeout := fClient.Timeout()
	fNetwork, fAddress := fClient.Server()
	conn, err := (&net.Dialer{Timeout: fTimeout}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "dial: chain %s failed to connect the first %s proxy server %s"
		return nil, errors.Wrapf(err, format, c.tag, fClient.Mode, fAddress)
	}
	pConn, err := c.connect(context.Background(), conn, network, address, clients)
	if err != nil {
		_ = conn.Close()
		const format = "dial: chain %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.tag, address)
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// DialContext is used to connect to address through proxy chain with context.
func (c *Chain) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	clients := c.getProxyClients()
	fClient := clients[0]
	fTimeout := fClient.Timeout()
	fNetwork, fAddress := fClient.Server()
	conn, err := (&net.Dialer{Timeout: fTimeout}).DialContext(ctx, fNetwork, fAddress)
	if err != nil {
		const format = "dial context: chain %s failed to connect the first %s proxy server %s"
		return nil, errors.Wrapf(err, format, c.tag, fClient.Mode, fAddress)
	}
	pConn, err := c.connect(ctx, conn, network, address, clients)
	if err != nil {
		_ = conn.Close()
		const format = "dial context: chain %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.tag, address)
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// DialTimeout is used to connect to address through proxy chain with timeout.
func (c *Chain) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = defaultDialTimeout
	}
	clients := c.getProxyClients()
	fClient := clients[0]
	fNetwork, fAddress := fClient.Server()
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "dial timeout: chain %s failed to connect the first %s proxy server %s"
		return nil, errors.Wrapf(err, format, c.tag, fClient.Mode, fAddress)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pConn, err := c.connect(ctx, conn, network, address, clients)
	if err != nil {
		_ = conn.Close()
		const format = "dial timeout: chain %s failed to connect %s"
		return nil, errors.WithMessagef(err, format, c.tag, address)
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// connect is used to get next proxy server network and address and use current proxy client
// to connect next proxy server, finally the last proxy server will connect the target.
func (c *Chain) connect(
	ctx context.Context,
	conn net.Conn,
	network string,
	address string,
	clients []*Client,
) (net.Conn, error) {
	// proxy client -> proxy server 1 -> proxy server 2 -> target server
	l := len(clients)
	var err error
	for i := 1; i < l; i++ {
		current := clients[i-1]
		next := clients[i]
		network, address := next.Server()
		conn, err = current.Connect(ctx, conn, network, address)
		if err != nil {
			const format = "%s proxy client %s failed to connect the next %s proxy server %s"
			args := []interface{}{current.Mode, current.Address, next.Mode, next.Address}
			return nil, errors.WithMessagef(err, format, args...)
		}
	}
	// the last proxy client will connect the target
	last := clients[l-1]
	conn, err = last.Connect(ctx, conn, network, address)
	if err != nil {
		const format = "the last %s proxy client %s failed to connect target"
		return nil, errors.WithMessagef(err, format, last.Mode, last.Address)
	}
	return conn, nil
}

// Connect is is a padding function.
func (c *Chain) Connect(context.Context, net.Conn, string, string) (net.Conn, error) {
	return nil, errors.New("proxy chain doesn't support connect method")
}

// HTTP is used to set *http.Transport about proxy.
func (c *Chain) HTTP(t *http.Transport) {
	t.DialContext = c.DialContext
}

// Timeout is a padding function.
func (c *Chain) Timeout() time.Duration {
	return 0
}

// Server is a padding function.
func (c *Chain) Server() (string, string) {
	return "", ""
}

// Info is used to get the proxy chain information, it will print all proxy client information.
func (c *Chain) Info() string {
	buf := new(bytes.Buffer)
	buf.WriteString("chain: ")
	buf.WriteString(c.tag)
	printClientsInfo(buf, c.clients)
	return buf.String()
}
