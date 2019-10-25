package socks

import (
	"errors"
	"strconv"
)

const (
	version5 uint8 = 0x05
	// auth method
	notRequired         uint8 = 0x00
	usernamePassword    uint8 = 0x02
	noAcceptableMethods uint8 = 0xFF
	// auth
	usernamePasswordVersion uint8 = 0x01
	statusSucceeded         uint8 = 0x00
	statusFailed            uint8 = 0x01

	reserve   uint8 = 0x00
	noReserve uint8 = 0x01
	// cmd
	connect uint8 = 0x01
	// address
	ipv4 uint8 = 0x01
	fqdn uint8 = 0x03
	ipv6 uint8 = 0x04
	// reply
	succeeded         uint8 = 0x00
	connRefused       uint8 = 0x05
	commandNotSupport uint8 = 0x07
	addressNotSupport uint8 = 0x08
)

var (
	ErrNotSupportNetwork   = errors.New("support only tcp tcp4 tcp6")
	ErrNoAcceptableMethods = errors.New("no acceptable authentication methods")
)

type Reply uint8

func (r Reply) String() string {
	switch r {
	case 0x01:
		return "general SOCKS server failure"
	case 0x02:
		return "connection not allowed by ruleset"
	case 0x03:
		return "network unreachable"
	case 0x04:
		return "host unreachable"
	case 0x05:
		return "connection refused"
	case 0x06:
		return "TTL expired"
	case 0x07:
		return "command not supported"
	case 0x08:
		return "address type not supported"
	default:
		return "unknown code: " + strconv.Itoa(int(r))
	}
}
