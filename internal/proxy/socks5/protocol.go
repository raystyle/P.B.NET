package socks5

import (
	"errors"
	"strconv"
)

const (
	version5 uint8 = 0x05
	// auth method
	not_required          uint8 = 0x00
	username_password     uint8 = 0x02
	no_acceptable_methods uint8 = 0xFF
	// auth
	username_password_version uint8 = 0x01
	status_succeeded          uint8 = 0x00
	status_failed             uint8 = 0x01

	reserve uint8 = 0x00
	// cmd
	connect uint8 = 0x01
	// address
	ipv4 uint8 = 0x01
	fqdn uint8 = 0x03
	ipv6 uint8 = 0x04
	// reply
	succeeded           uint8 = 0x00
	command_not_support uint8 = 0x07
)

var (
	ERR_NOT_SUPPORT_NETWORK   = errors.New("support only tcp tcp4 tcp6")
	ERR_NO_ACCEPTABLE_METHODS = errors.New("no acceptable authentication methods")
)

type Reply uint8

func (this Reply) String() string {
	switch this {
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
		return "unknown code: " + strconv.Itoa(int(this))
	}
}
