package dns

import (
	"net"

	"github.com/pkg/errors"
	"golang.org/x/net/dns/dnsmessage"

	"project/internal/random"
)

const (
	IPv4 = "ipv4"
	IPv6 = "ipv6"
)

var (
	types = map[string]dnsmessage.Type{
		IPv4: dnsmessage.TypeA,
		IPv6: dnsmessage.TypeAAAA,
	}
)

var (
	ErrNoResolveResult = errors.New("no resolve result")
)

// from GOROOT/src/net/dnsclient.go

// checks if a string is a presentation-format domain name
// (currently restricted to hostname-compatible "preferred name" LDH labels and
// SRV-like "underscore labels"; see golang.org/issue/12421).
func IsDomainName(s string) bool {
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
	partLen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		default:
			return false
		case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_':
			nonNumeric = true
			partLen++
		case '0' <= c && c <= '9':
			// fine
			partLen++
		case c == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partLen++
			nonNumeric = true
		case c == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partLen > 63 || partLen == 0 {
				return false
			}
			partLen = 0
		}
		last = c
	}
	if last == '-' || partLen > 63 {
		return false
	}
	return nonNumeric
}

func packMessage(typ dnsmessage.Type, domain string) []byte {
	header := dnsmessage.Header{
		ID:                 uint16(random.Int(65536)),
		Response:           false,
		OpCode:             0,
		Authoritative:      false,
		Truncated:          false,
		RecursionDesired:   true,
		RecursionAvailable: false,
		RCode:              0,
	}
	// name is not in canonical format (it must end with a .)
	domain += "."
	name, _ := dnsmessage.NewName(domain)
	question := dnsmessage.Question{
		Name:  name,
		Type:  typ,
		Class: dnsmessage.ClassINET,
	}
	msg := dnsmessage.Message{
		Header:    header,
		Questions: []dnsmessage.Question{question},
	}
	b, _ := msg.Pack()
	return b
}

func unpackMessage(message []byte) ([]string, error) {
	msg := dnsmessage.Message{}
	err := msg.Unpack(message)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var result []string
	for i := 0; i < len(msg.Answers); i++ {
		switch msg.Answers[i].Header.Type {
		case dnsmessage.TypeA:
			res := msg.Answers[i].Body.(*dnsmessage.AResource)
			ip := make([]byte, net.IPv4len)
			copy(ip, res.A[:])
			result = append(result, net.IP(ip).String())
		case dnsmessage.TypeAAAA:
			res := msg.Answers[i].Body.(*dnsmessage.AAAAResource)
			ip := make([]byte, net.IPv6len)
			copy(ip, res.AAAA[:])
			result = append(result, net.IP(ip).String())
		}
	}
	if len(result) == 0 {
		return nil, ErrNoResolveResult
	}
	return result, nil
}
