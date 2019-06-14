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
	"project/internal/options"
)

const (
	DEFAULT_METHOD = TLS
	// tcp && tls use
	header_size     = 2
	default_timeout = time.Minute // udp is 5 second
)

// query type
type Type string

const (
	IPV4 Type = "ipv4"
	IPV6 Type = "ipv6"
)

var (
	query_type = map[Type]uint16{
		"":   1,
		IPV4: 1,  // A
		IPV6: 28, // AAAA
	}
)

// resolve method
type Method string

const (
	UDP Method = "udp"
	TCP Method = "tcp"
	TLS Method = "tls" // DNS-Over-TLS
	DOH Method = "doh" // DNS-Over-HTTPS
)

var (
	ERR_INVALID_DOMAIN_NAME = errors.New("invalid domain name")
	ERR_INVALID_TYPE        = errors.New("invalid ip type")
	ERR_UNKNOWN_METHOD      = errors.New("unknown method")
	ERR_UNKNOWN_NETWORK     = errors.New("unknown network")
	ERR_NO_RESOLVE_RESULT   = errors.New("no resolve result")
	ERR_NO_CONNECTION       = errors.New("no connection")
	ERR_INVALID_TLS_CONFIG  = errors.New("invalid tls method config")
)

type Options struct {
	// default "ipv4"
	Type Type
	// default tls
	Method Method
	// "tcp" "tcp4" "tcp6"
	// default "tcp" if use UDP Method "udp"
	// useless for dns_doh
	Network string
	// default 60s
	Timeout time.Duration
	// for proxy useless for doh
	Dial func(network, address string) (net.Conn, error)
	// about DOH
	Header    http.Header
	Transport *http.Transport
}

// address = dns server(doh server) ip + port
func Resolve(address, domain string, opts *Options) ([]string, error) {
	if opts == nil {
		opts = new(Options)
	}
	// check domain name
	if Is_IP(domain) {
		return []string{domain}, nil
	}
	// punycode
	domain, err := to_ascii(domain)
	if err != nil {
		return nil, err
	}
	if !Is_Domain(domain) {
		return nil, ERR_INVALID_DOMAIN_NAME
	}
	// check type
	switch opts.Type {
	case "", IPV4, IPV6:
	default:
		return nil, ERR_INVALID_TYPE
	}
	question := pack_question(query_type[opts.Type], domain)
	// send request
	var answer []byte
	switch opts.Method {
	case "", TLS: // default
		answer, err = dial_tls(address, question, opts)
	case UDP:
		answer, err = dial_udp(address, question, opts)
	case TCP:
		answer, err = dial_tcp(address, question, opts)
	case DOH:
		answer, err = dial_https(address, question, opts)
	default:
		return nil, ERR_UNKNOWN_METHOD
	}
	if err != nil {
		return nil, err
	}
	return resolve(opts.Type, answer)
}

func Is_IP(ip string) bool {
	if net.ParseIP(ip) == nil {
		return false
	}
	return true
}

// from GOROOT/src/net/dnsclient.go

// checks if a string is a presentation-format domain name
// (currently restricted to hostname-compatible "preferred name" LDH labels and
// SRV-like "underscore labels"; see golang.org/issue/12421).
func Is_Domain(s string) bool {
	// See RFC 1035, RFC 3696.
	// Presentation format has dots before every label except the first, and the
	// terminal empty label is optional here because we assume fully-qualified
	// (absolute) input. We must therefore reserve space for the first and last
	// labels' length octets in wire format, where they are necessary and the
	// maximum total length is 255.
	// So our _effective_ maximum is 253, but 254 is not rejected if the last
	// character is a dot.
	l := len(s)
	if l == 0 || l > 254 || l == 254 && s[l-1] != '.' {
		return false
	}
	last := byte('.')
	nonNumeric := false // true once we've seen a letter or hyphen
	partlen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		default:
			return false
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_':
			nonNumeric = true
			partlen++
		case '0' <= c && c <= '9':
			// fine
			partlen++
		case c == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partlen++
			nonNumeric = true
		case c == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partlen > 63 || partlen == 0 {
				return false
			}
			partlen = 0
		}
		last = c
	}
	if last == '-' || partlen > 63 {
		return false
	}
	return nonNumeric
}

func dial_tls(address string, question []byte, opts *Options) ([]byte, error) {
	network := opts.Network
	switch network {
	case "": //default
		network = "tcp"
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ERR_UNKNOWN_NETWORK
	}
	dial := net.Dial
	if opts.Dial != nil {
		dial = opts.Dial
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
		c, err := dial(network, address)
		if err != nil {
			return nil, err
		}
		conn = tls.Client(c, &tls.Config{ServerName: host})
	case 2: // domain mode dns.google:853|8.8.8.8,8.8.4.4
		ip_list := strings.Split(config[1], ",")
		for i := 0; i < len(ip_list); i++ {
			c, err := dial(network, ip_list[i]+":"+port)
			if err == nil {
				conn = tls.Client(c, &tls.Config{ServerName: host})
				break
			}
		}
		if conn == nil {
			return nil, ERR_NO_CONNECTION
		}
	default:
		return nil, ERR_INVALID_TLS_CONFIG
	}
	defer func() { _ = conn.Close() }()
	// set timeout
	if opts.Timeout > 0 {
		err = conn.SetDeadline(time.Now().Add(opts.Timeout))
	} else {
		err = conn.SetDeadline(time.Now().Add(default_timeout))
	}
	if err != nil {
		return nil, err
	}
	// add size header
	q := bytes.NewBuffer(convert.Uint16_Bytes(uint16(len(question))))
	q.Write(question)
	_, err = conn.Write(q.Bytes())
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, 512)
	_, err = io.ReadAtLeast(conn, buffer[:header_size], header_size)
	if err != nil {
		return nil, err
	}
	l := int(convert.Bytes_Uint16(buffer[:header_size]))
	if l > 512 {
		buffer = make([]byte, l)
	}
	_, err = io.ReadAtLeast(conn, buffer[:l], l)
	if err != nil {
		return nil, err
	}
	return buffer[:l], nil
}

// if question > 512 use tcp tls doh
func dial_udp(address string, question []byte, opts *Options) ([]byte, error) {
	network := opts.Network
	switch network {
	case "":
		network = "udp" //default
	case "udp", "udp4", "udp6":
	default:
		return nil, ERR_UNKNOWN_NETWORK
	}
	dial := net.Dial
	if opts.Dial != nil {
		dial = opts.Dial
	}
	for i := 0; i < 3; i++ {
		conn, err := dial(network, address)
		if err != nil {
			return nil, err //not continue
		}
		// set timeout
		if opts.Timeout > 0 {
			_ = conn.SetDeadline(time.Now().Add(opts.Timeout))
		} else {
			// udp is 5 second
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		}
		_, _ = conn.Write(question)
		buffer := make([]byte, 512)
		n, err := conn.Read(buffer)
		if err == nil {
			_ = conn.Close()
			return buffer[:n], nil
		}
		_ = conn.Close()
	}
	return nil, ERR_NO_CONNECTION
}

func dial_tcp(address string, question []byte, opts *Options) ([]byte, error) {
	network := opts.Network
	switch network {
	case "":
		network = "tcp" //default
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, ERR_UNKNOWN_NETWORK
	}
	dial := net.Dial
	if opts.Dial != nil {
		dial = opts.Dial
	}
	conn, err := dial(network, address)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	// set timeout
	if opts.Timeout > 0 {
		err = conn.SetDeadline(time.Now().Add(opts.Timeout))
	} else {
		err = conn.SetDeadline(time.Now().Add(default_timeout))
	}
	if err != nil {
		return nil, err
	}
	// add size header
	q := bytes.NewBuffer(convert.Uint16_Bytes(uint16(len(question))))
	q.Write(question)
	_, err = conn.Write(q.Bytes())
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, 512)
	_, err = io.ReadAtLeast(conn, buffer[:header_size], header_size)
	if err != nil {
		return nil, err
	}
	l := int(convert.Bytes_Uint16(buffer[:header_size]))
	if l > 512 {
		buffer = make([]byte, l)
	}
	_, err = io.ReadAtLeast(conn, buffer[:l], l)
	if err != nil {
		return nil, err
	}
	return buffer[:l], nil
}

// support RFC 8484
func dial_https(server string, question []byte, opts *Options) ([]byte, error) {
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
	if opts.Header != nil {
		req.Header = options.Copy_HTTP_Header(opts.Header)
	}
	if req.Method == http.MethodPost {
		req.Header.Set("Content-Type", "application/dns-message")
	}
	req.Header.Set("Accept", "application/dns-message")
	// http client
	c := http.Client{
		Timeout: default_timeout,
	}
	if opts.Transport != nil {
		c.Transport = opts.Transport
	}
	if opts.Timeout > 0 {
		c.Timeout = opts.Timeout
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
		c.CloseIdleConnections()
	}()
	return ioutil.ReadAll(resp.Body)
}

func resolve(t Type, answer []byte) (ip_list []string, err error) {
	// recover panic array bound
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				err = v
			}
			ip_list = nil
		}
	}()
	// answer number
	answer_rrs := convert.Bytes_Uint16(answer[6:8])
	if answer_rrs == 0 {
		return nil, ERR_NO_RESOLVE_RESULT
	}
	// queries&&answers
	queries_answers := bytes.SplitN(answer[12:], []byte{0}, 2)
	answers := queries_answers[1][4:]
	offset := 0
	switch t {
	case "", IPV4:
		for i := 0; i < int(answer_rrs); i++ {
			_type := answers[offset+2 : offset+4]
			if !bytes.Equal(_type, []byte{0, 1}) { // only type A
				data_length := convert.Bytes_Uint16(answers[offset+10 : offset+12])
				// 12 = 2(name)+2(type)+2(class)+4(time to live)+2(data length)
				offset += 12 + int(data_length)
			} else {
				ipv4 := answers[offset+12 : offset+16]
				ip_list = append(ip_list, net.IP(ipv4).String())
				offset += 16
			}
		}
	case IPV6:
		for i := 0; i < int(answer_rrs); i++ {
			_type := answers[offset+2 : offset+4]
			if !bytes.Equal(_type, []byte{0, 28}) { // only type AAAA
				data_length := convert.Bytes_Uint16(answers[offset+10 : offset+12])
				// 2(name)+2(type)+2(class)+4(time to live)+2(data len)+data len
				offset += 12 + int(data_length)
			} else {
				ipv6 := answers[offset+12 : offset+28]
				ip_list = append(ip_list, net.IP(ipv6).String())
				offset += 28
			}
		}
	}
	return
}
