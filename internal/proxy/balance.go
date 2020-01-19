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

	"github.com/pkg/errors"
)

// Balance implemented client
type Balance struct {
	tag     string
	clients []*Client // not nil

	flags map[*Client]bool
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
		flags:   flags,
	}
	return &b, nil
}

func (b *Balance) selectNextProxyClient() *Client {
	b.mutex.Lock()
	defer b.mutex.Unlock()
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

// GetAndSelectNext is used to provide chain
// next.client will not be *Balance
func (b *Balance) GetAndSelectNext() *Client {
	next := b.selectNextProxyClient()
	if next.Mode == ModeBalance {
		next = next.client.(*Balance).GetAndSelectNext()
	}
	return next
}

// Dial is used to connect to address through selected proxy client
func (b *Balance) Dial(network, address string) (net.Conn, error) {
	conn, err := b.GetAndSelectNext().Dial(network, address)
	return conn, errors.WithMessagef(err, "balance %s", b.tag)
}

// DialContext is used to connect to address through selected proxy client with context
func (b *Balance) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := b.GetAndSelectNext().DialContext(ctx, network, address)
	return conn, errors.WithMessagef(err, "balance %s", b.tag)
}

// DialTimeout is used to connect to address through selected proxy client with timeout
func (b *Balance) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	conn, err := b.GetAndSelectNext().DialTimeout(network, address, timeout)
	return conn, errors.WithMessagef(err, "balance %s", b.tag)
}

// Connect is a padding function
func (b *Balance) Connect(context.Context, net.Conn, string, string) (net.Conn, error) {
	return nil, errors.New("balance doesn't support connect")
}

// HTTP is used to set *http.Transport about proxy
func (b *Balance) HTTP(t *http.Transport) {
	t.DialContext = b.DialContext
}

// Timeout is a padding function
func (b *Balance) Timeout() time.Duration {
	return 0
}

// Server is a padding function
func (b *Balance) Server() (string, string) {
	return "", ""
}

// Info is used to get the balance info, it will print all proxy client info
func (b *Balance) Info() string {
	buf := new(bytes.Buffer)
	buf.WriteString("balance: ")
	buf.WriteString(b.tag)
	printInfo(buf, b.clients)
	return buf.String()
}

// balance: tag
// 1. tag-a:  http://admin:123456@127.0.0.1:8080
// 2. https-c:  https://admin:123456@127.0.0.1:8080
// 3. tag-c:  socks5 tcp 127.0.0.1:1080 auth: admin1:1234561
// 4. tag-dd: socks4a tcp 127.0.0.1:1081 user id: admin2
//
// if balance in balance, chain in balance or ...
//
// balance: final-balance
// 1. balance-1:
//     mode: balance
//     1. socks5-c:  socks5 tcp 127.0.0.1:4627 admin1:1234561
//     2. socks4a-c: socks4a tcp 127.0.0.1:4628
//     3. http-c:    http://admin3:1234563@127.0.0.1:4629
//     4. https-c:   https://admin4:1234564@127.0.0.1:4630
// 2. balance-2:
//     mode: balance
//     1. socks5-c:  socks5 tcp 127.0.0.1:4627 admin1:1234561
//     2. socks4a-c: socks4a tcp 127.0.0.1:4628
//     3. http-c:    http://admin3:1234563@127.0.0.1:4629
//     4. https-c:   https://admin4:1234564@127.0.0.1:4630
// 3. chain-3:
//     mode: chain
//     1. socks5-c:  socks5 tcp 127.0.0.1:4627 admin1:1234561
//     2. socks4a-c: socks4a tcp 127.0.0.1:4628
//     3. http-c:    http://admin3:1234563@127.0.0.1:4629
//     4. https-c:   https://admin4:1234564@127.0.0.1:4630
// 4. https-ccc: https://admin4:1234564@127.0.0.1:4630
// 5. http-ccc:  http://admin3:1234563@127.0.0.1:4629
func printInfo(buf *bytes.Buffer, clients []*Client) {
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
