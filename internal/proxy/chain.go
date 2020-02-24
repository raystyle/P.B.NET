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
		return nil, errors.New("empty chain tag")
	}
	l := len(clients)
	if l == 0 {
		return nil, errors.New("chain need at least one proxy client")
	}
	return &Chain{
		tag:     tag,
		clients: clients,
		count:   l,
	}, nil
}

// []*Client will not include ModeBalance or ModeChain.
func (c *Chain) getClients() []*Client {
	// if chain in clients, len(clients) will bigger than c.count
	clients := make([]*Client, 0, c.count)
	for _, client := range c.clients {
		switch client.Mode {
		case ModeBalance:
			c := client.client.(*Balance).GetAndSelectNext()
			if c.Mode == ModeChain {
				clients = append(clients, c.client.(*Chain).getClients()...)
			} else {
				clients = append(clients, c)
			}
		case ModeChain:
			clients = append(clients, client.client.(*Chain).getClients()...)
		default:
			clients = append(clients, client)
		}
	}
	return clients
}

func connect(
	ctx context.Context,
	conn net.Conn,
	network string,
	address string,
	clients []*Client,
) (net.Conn, error) {
	l := len(clients)
	var err error
	for i := 1; i < l; i++ {
		// get next proxy server network and address
		network, address := clients[i].Server()
		// use current client to connect next proxy server
		conn, err = clients[i-1].Connect(ctx, conn, network, address)
		if err != nil {
			return nil, err
		}
	}
	// connect the target
	return clients[l-1].Connect(ctx, conn, network, address)
}

// Dial is used to connect to address through proxy chain.
func (c *Chain) Dial(network, address string) (net.Conn, error) {
	clients := c.getClients()
	fTimeout := clients[0].Timeout()
	fNetwork, fAddress := clients[0].Server()
	conn, err := (&net.Dialer{Timeout: fTimeout}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "chain %s dial: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.tag, fAddress)
	}
	pConn, err := connect(context.Background(), conn, network, address, clients)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessagef(err, "chain %s", c.tag)
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// DialContext is used to connect to address through proxy chain with context.
func (c *Chain) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	clients := c.getClients()
	fTimeout := clients[0].Timeout()
	fNetwork, fAddress := clients[0].Server()
	conn, err := (&net.Dialer{Timeout: fTimeout}).DialContext(ctx, fNetwork, fAddress)
	if err != nil {
		const format = "chain %s dial context: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.tag, fAddress)
	}
	pConn, err := connect(ctx, conn, network, address, clients)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessagef(err, "chain %s", c.tag)
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// DialTimeout is used to connect to address through proxy chain with timeout.
func (c *Chain) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if timeout < 1 {
		timeout = defaultDialTimeout
	}
	clients := c.getClients()
	fNetwork, fAddress := clients[0].Server()
	conn, err := (&net.Dialer{Timeout: timeout}).Dial(fNetwork, fAddress)
	if err != nil {
		const format = "chain %s dial timeout: failed to connect the first proxy %s"
		return nil, errors.Wrapf(err, format, c.tag, fAddress)
	}
	pConn, err := connect(context.Background(), conn, network, address, clients)
	if err != nil {
		_ = conn.Close()
		return nil, errors.WithMessagef(err, "chain %s", c.tag)
	}
	_ = pConn.SetDeadline(time.Time{})
	return pConn, nil
}

// Connect is is a padding function.
func (c *Chain) Connect(context.Context, net.Conn, string, string) (net.Conn, error) {
	return nil, errors.New("chain doesn't support connect")
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

// Info is used to get the proxy chain information,
// it will print all proxy client information.
func (c *Chain) Info() string {
	buf := new(bytes.Buffer)
	buf.WriteString("chain: ")
	buf.WriteString(c.tag)
	printInfo(buf, c.clients)
	return buf.String()
}
