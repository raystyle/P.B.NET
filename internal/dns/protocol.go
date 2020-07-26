package dns

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
	"golang.org/x/net/dns/dnsmessage"
)

// support query type
const (
	TypeIPv4 = "ipv4"
	TypeIPv6 = "ipv6"
)

var (
	types = map[string]dnsmessage.Type{
		TypeIPv4: dnsmessage.TypeA,
		TypeIPv6: dnsmessage.TypeAAAA,
	}
)

// ErrNoResolveResult is an error of the resolve
var ErrNoResolveResult = fmt.Errorf("no resolve result")

// IsDomainName is used to checks if a string is a presentation-format domain name
// (currently restricted to hostname-compatible "preferred name" LDH labels and
// SRV-like "underscore labels"; see golang.org/issue/12421).
//
// See RFC 1035, RFC 3696.
// Presentation format has dots before every label except the first, and the
// terminal empty label is optional here because we assume fully-qualified
// (absolute) input. We must therefore reserve space for the first and last
// labels' length octets in wire format, where they are necessary and the
// maximum total length is 255.
// So our _effective_ maximum is 253, but 254 is not rejected if the last
// character is a dot.
//
// from GOROOT/src/net/dnsclient.go
func IsDomainName(s string) bool {
	l := len(s)
	if l == 0 || l > 254 || l == 254 && s[l-1] != '.' {
		return false
	}
	last := byte('.')
	nonNumeric := false // true once we've seen a letter or hyphen
	partLen := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		ok := false
		checkChar(c, last, &nonNumeric, &partLen, &ok)
		if !ok {
			return false
		}
		last = c
	}
	if last == '-' || partLen > 63 {
		return false
	}
	return nonNumeric
}

func checkChar(c byte, last byte, nonNumeric *bool, partLen *int, ok *bool) {
	switch {
	case 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_':
		*nonNumeric = true
		*partLen++
		*ok = true
	case '0' <= c && c <= '9':
		// fine
		*partLen++
		*ok = true
	case c == '-':
		// Byte before dash cannot be dot.
		if last == '.' {
			return
		}
		*partLen++
		*nonNumeric = true
		*ok = true
	case c == '.':
		// Byte before dot cannot be dot, dash.
		if last == '.' || last == '-' {
			return
		}
		if *partLen > 63 || *partLen == 0 {
			return
		}
		*partLen = 0
		*ok = true
	}
}

// packMessage is used to pack to DNS message.
func packMessage(typ dnsmessage.Type, domain string, queryID uint16) []byte {
	header := dnsmessage.Header{
		ID:               queryID,
		RecursionDesired: true,
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

// unpackMessage is used to unpack message and verify message.
func unpackMessage(message []byte, domain string, queryID uint16) ([]string, error) {
	msg := dnsmessage.Message{}
	err := msg.Unpack(message)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// check message is response
	if !msg.Response {
		return nil, errors.New("dns message is not a response")
	}
	// check query ID
	if msg.ID != queryID {
		const format = "query id \"0x%04X\" in dns message is different with original \"0x%04X\""
		return nil, errors.Errorf(format, msg.ID, queryID)
	}
	// check question name is equal original domain name
	if len(msg.Questions) != 1 {
		return nil, errors.New("dns message with unexpected question")
	}
	name := msg.Questions[0].Name
	// name.Length must >= 1
	nameStr := name.String()[:name.Length-1]
	if nameStr != domain {
		const format = "domain name \"%s\" in dns message is different with original \"%s\""
		return nil, errors.Errorf(format, nameStr, domain)
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
		return nil, errors.WithStack(ErrNoResolveResult)
	}
	return result, nil
}
