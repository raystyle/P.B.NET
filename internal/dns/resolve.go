package dns

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	"project/internal/convert"
	"project/internal/xnet"
)

const (
	defaultTimeout     = time.Minute // udp is 5 second
	defaultMaxBodySize = 4096        // about DOH

	headerSize = 2 // tcp && tls need
)

var (
	ErrNoConnection = errors.New("no connection")
)

type UnknownTypeError string

func (t UnknownTypeError) Error() string {
	return fmt.Sprintf("unknown type: %s", string(t))
}

// address = dns server(doh server) ip + port
func resolve(address, domain string, opts *Options) ([]string, error) {
	// check domain name
	if net.ParseIP(domain) != nil { // ip
		return []string{domain}, nil
	}
	// punycode
	domain, err := toASCII(domain)
	if err != nil {
		return nil, err
	}
	if !IsDomainName(domain) {
		return nil, fmt.Errorf("invalid domain name: %s", domain)
	}
	// check type
	switch opts.Type {
	case "", IPv4, IPv6:
	default:
		return nil, UnknownTypeError(opts.Type)
	}
	message := packMessage(types[opts.Type], domain)
	switch opts.Method {
	case "", UDP: // default
		message, err = dialUDP(address, message, opts)
	case TCP:
		message, err = dialTCP(address, message, opts)
	case DoT:
		message, err = dialDoT(address, message, opts)
	case DoH:
		message, err = dialDoH(address, message, opts)
	default:
		return nil, UnknownMethodError(opts.Method)
	}
	if err != nil {
		return nil, err
	}
	return unpackMessage(message)
}

// if question > 512 use tcp tls doh
func dialUDP(address string, message []byte, opts *Options) ([]byte, error) {
	network := opts.Network
	switch network {
	case "":
		network = "udp" // default
	case "udp", "udp4", "udp6":
	default:
		return nil, net.UnknownNetworkError(network)
	}
	dial := net.DialTimeout
	if opts.dial != nil {
		dial = opts.dial
	}
	// set timeout
	timeout := opts.Timeout
	if opts.Timeout < 1 {
		timeout = 5 * time.Second
	}
	for i := 0; i < 3; i++ {
		conn, err := dial(network, address, timeout)
		if err != nil {
			return nil, err // not continue
		}
		dConn := xnet.DeadlineConn(conn, timeout)
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
	if opts.Timeout < 1 {
		timeout = defaultTimeout
	}
	dial := net.DialTimeout
	if opts.dial != nil {
		dial = opts.dial
	}
	conn, err := dial(network, address, timeout)
	if err != nil {
		return nil, err
	}
	dConn := xnet.DeadlineConn(conn, timeout)
	defer func() { _ = dConn.Close() }()
	// add size header
	header := bytes.NewBuffer(convert.Uint16ToBytes(uint16(len(message))))
	header.Write(message)
	_, err = dConn.Write(header.Bytes())
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, 512)
	_, err = io.ReadFull(dConn, buffer[:headerSize])
	if err != nil {
		return nil, err
	}
	l := int(convert.BytesToUint16(buffer[:headerSize]))
	if l > 512 {
		buffer = make([]byte, l)
	}
	_, err = io.ReadFull(dConn, buffer[:l])
	if err != nil {
		return nil, err
	}
	return buffer[:l], nil
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
	if opts.Timeout < 1 {
		timeout = defaultTimeout
	}
	dial := net.DialTimeout
	if opts.dial != nil {
		dial = opts.dial
	}
	config := strings.Split(address, "|")
	var (
		conn *tls.Conn
		err  error
	)
	host, port, err := net.SplitHostPort(config[0])
	if err != nil {
		return nil, err
	}
	switch len(config) {
	case 1: // ip mode     8.8.8.8:853
		c, err := dial(network, address, timeout)
		if err != nil {
			return nil, err
		}
		conn = tls.Client(c, &tls.Config{ServerName: host})
	case 2: // domain mode dns.google:853|8.8.8.8,8.8.4.4
		ipList := strings.Split(strings.TrimSpace(config[1]), ",")
		for i := 0; i < len(ipList); i++ {
			c, err := dial(network, ipList[i]+":"+port, timeout)
			if err == nil {
				conn = tls.Client(c, &tls.Config{ServerName: host})
				break
			}
		}
		if conn == nil {
			return nil, ErrNoConnection
		}
	default:
		return nil, fmt.Errorf("invalid address: %s", address)
	}
	dConn := xnet.DeadlineConn(conn, timeout)
	defer func() { _ = dConn.Close() }()
	// add size header
	header := bytes.NewBuffer(convert.Uint16ToBytes(uint16(len(message))))
	header.Write(message)
	_, err = dConn.Write(header.Bytes())
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, 512)
	// read message size
	_, err = io.ReadFull(dConn, buffer[:headerSize])
	if err != nil {
		return nil, err
	}
	l := int(convert.BytesToUint16(buffer[:headerSize]))
	if l > 512 {
		buffer = make([]byte, l)
	}
	_, err = io.ReadFull(dConn, buffer[:l])
	if err != nil {
		return nil, err
	}
	return buffer[:l], nil
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
	req.Header = opts.Header.Clone()
	if req.Header == nil {
		req.Header = http.Header{}
	}
	if req.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/dns-message")
	}
	req.Header.Set("Accept", "application/dns-message")
	// http client
	client := http.Client{
		Timeout: opts.Timeout,
	}
	if opts.transport != nil {
		client.Transport = opts.transport
	}
	if opts.Timeout < 1 {
		client.Timeout = defaultTimeout
	}
	maxBodySize := opts.MaxBodySize
	if maxBodySize < 1 {
		maxBodySize = defaultMaxBodySize
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
		client.CloseIdleConnections()
	}()
	return ioutil.ReadAll(io.LimitReader(resp.Body, maxBodySize))
}
