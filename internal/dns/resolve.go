package dns

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/idna"

	"project/internal/convert"
	"project/internal/xnet/xnetutil"
)

const (
	// udp is 3 second
	defaultTimeout = 5 * time.Second

	// about DOH
	defaultMaxBodySize = 65535

	// tcp && tls need it
	headerSize = 2
)

var (
	ErrNoConnection = fmt.Errorf("no connection")
)

func systemResolve(typ string, domain string) ([]string, error) {
	ips, err := net.LookupHost(domain)
	if err != nil {
		return nil, err
	}
	var (
		ipv4List []string
		ipv6List []string
	)
	for _, ip := range ips {
		ip := net.ParseIP(ip)
		ip4 := ip.To4()
		if ip4 != nil {
			ipv4List = append(ipv4List, ip4.String())
		} else {
			ipv6List = append(ipv6List, ip.To16().String())
		}
	}
	if typ == TypeIPv4 {
		return ipv4List, nil
	} else { // about error type
		return ipv6List, nil
	}
}

// address is dns server address
func customResolve(method, address, domain, typ string, opts *Options) ([]string, error) {
	// check domain name is IP
	if ip := net.ParseIP(domain); ip != nil {
		return []string{ip.String()}, nil
	}
	// punycode
	domain, _ = idna.ToASCII(domain)
	// check domain name
	if !IsDomainName(domain) {
		return nil, errors.Errorf("invalid domain name: %s", domain)
	}
	message := packMessage(types[typ], domain)
	var err error
	switch method {
	case MethodUDP:
		message, err = dialUDP(address, message, opts)
	case MethodTCP:
		message, err = dialTCP(address, message, opts)
	case MethodDoT:
		message, err = dialDoT(address, message, opts)
	case MethodDoH:
		message, err = dialDoH(address, message, opts)
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return unpackMessage(message)
}

// if question > 512 use tcp tls doh
func dialUDP(address string, message []byte, opts *Options) ([]byte, error) {
	network := opts.Network
	switch network {
	case "":
		network = "udp"
	case "udp", "udp4", "udp6":
	default:
		return nil, net.UnknownNetworkError(network)
	}
	// set timeout
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = 3 * time.Second
	}
	// dial
	for i := 0; i < 3; i++ {
		conn, err := opts.dial(network, address, timeout)
		if err != nil {
			return nil, err // not continue
		}
		dConn := xnetutil.DeadlineConn(conn, timeout)
		_, _ = dConn.Write(message)
		buffer := make([]byte, 512)
		n, err := dConn.Read(buffer)
		if err == nil {
			_ = dConn.Close()
			return buffer[:n], nil
		}
		_ = dConn.Close()
	}
	return nil, ErrNoConnection
}

func sendMessage(conn net.Conn, message []byte, timeout time.Duration) ([]byte, error) {
	dConn := xnetutil.DeadlineConn(conn, timeout)
	defer func() { _ = dConn.Close() }()
	// add size header
	header := new(bytes.Buffer)
	header.Write(convert.Uint16ToBytes(uint16(len(message))))
	header.Write(message)
	_, err := dConn.Write(header.Bytes())
	if err != nil {
		return nil, err
	}
	// read message size
	length := make([]byte, headerSize)
	_, err = io.ReadFull(dConn, length)
	if err != nil {
		return nil, err
	}
	resp := make([]byte, int(convert.BytesToUint16(length)))
	_, err = io.ReadFull(dConn, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func dialTCP(address string, message []byte, opts *Options) ([]byte, error) {
	network := opts.Network
	switch network {
	case "":
		network = "tcp" // default
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, net.UnknownNetworkError(network)
	}
	// set timeout
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = defaultTimeout
	}
	// dial
	conn, err := opts.dial(network, address, timeout)
	if err != nil {
		return nil, err
	}
	return sendMessage(conn, message, timeout)
}

func dialDoT(address string, message []byte, opts *Options) ([]byte, error) {
	network := opts.Network
	switch network {
	case "": // default
		network = "tcp"
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, net.UnknownNetworkError(network)
	}
	// set timeout
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = 2 * defaultTimeout
	}
	// load config
	config := strings.Split(address, "|")
	host, port, err := net.SplitHostPort(config[0])
	if err != nil {
		return nil, err
	}
	var conn *tls.Conn
	switch len(config) {
	case 1: // ip mode
		// 8.8.8.8:853
		// [2606:4700:4700::1001]:853
		c, err := opts.dial(network, address, timeout)
		if err != nil {
			return nil, err
		}
		conn = tls.Client(c, &tls.Config{ServerName: host})
	case 2: // domain mode
		// dns.google:853|8.8.8.8,8.8.4.4
		// cloudflare-dns.com:853|2606:4700:4700::1001,2606:4700:4700::1111
		ipList := strings.Split(strings.TrimSpace(config[1]), ",")
		for i := 0; i < len(ipList); i++ {
			c, err := opts.dial(network, net.JoinHostPort(ipList[i], port), timeout)
			if err == nil {
				conn = tls.Client(c, &tls.Config{ServerName: host})
				break
			}
		}
		if conn == nil {
			return nil, ErrNoConnection
		}
	default:
		return nil, errors.Errorf("invalid address: %s", address)
	}
	return sendMessage(conn, message, timeout)
}

// support RFC 8484
func dialDoH(server string, question []byte, opts *Options) ([]byte, error) {
	str := base64.RawURLEncoding.EncodeToString(question)
	url := fmt.Sprintf("%s?ct=application/dns-message&dns=%s", server, str)
	var (
		req *http.Request
		err error
	)
	if len(url) < 2048 { // GET
		req, err = http.NewRequest(http.MethodGet, url, nil)
	} else { // POST
		req, err = http.NewRequest(http.MethodPost, server, bytes.NewReader(question))
	}
	if err != nil {
		return nil, err
	}

	// set header
	req.Header = opts.Header.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if req.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/dns-message")
	}
	req.Header.Set("Accept", "application/dns-message")

	// http client
	client := http.Client{
		Transport: opts.transport,
		Timeout:   opts.Timeout,
	}
	defer client.CloseIdleConnections()
	if client.Timeout < 1 {
		client.Timeout = 2 * defaultTimeout
	}
	maxBodySize := opts.MaxBodySize
	if maxBodySize < 1 {
		maxBodySize = defaultMaxBodySize
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return ioutil.ReadAll(io.LimitReader(resp.Body, maxBodySize))
}
