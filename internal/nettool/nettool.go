package nettool

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// ErrEmptyPort is an error of CheckPortString.
var ErrEmptyPort = errors.New("empty port")

// CheckPort is used to check port range.
func CheckPort(port int) error {
	if port < 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	return nil
}

// CheckPortString is used to check port range, port is a string.
func CheckPortString(port string) error {
	if port == "" {
		return ErrEmptyPort
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	return CheckPort(p)
}

// SplitHostPort is used to split address to host and port.
func SplitHostPort(address string) (string, uint16, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, err
	}
	err = CheckPort(portNum)
	if err != nil {
		return "", 0, err
	}
	return host, uint16(portNum), nil
}

// EncodeExternalAddress is used to encode connection external address.
// If address is IP+Port, parse IP and return byte slice, ot return []byte(addr).
func EncodeExternalAddress(address string) []byte {
	var external []byte
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		external = []byte(address) // for special remote address
	} else {
		ip := net.ParseIP(host)
		if ip != nil {
			external = ip
		} else {
			external = []byte(host) // for special remote address
		}
	}
	return external
}

// DecodeExternalAddress is used to decode connection external address.
// If address is a IP, return it, or return string(address).
func DecodeExternalAddress(address []byte) string {
	ip := net.IP(address).String()
	if strings.Contains(ip, ".") || strings.Contains(ip, ":") {
		return ip
	}
	return string(address)
}

// IPEnabled is used to get system IP enabled.
func IPEnabled() (ipv4Enabled, ipv6Enabled bool) {
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags != net.FlagUp|net.FlagBroadcast|net.FlagMulticast {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipAddr := strings.Split(addr.String(), "/")[0]
			ip := net.ParseIP(ipAddr)
			ip4 := ip.To4()
			if ip4 != nil {
				if ip4.IsGlobalUnicast() {
					ipv4Enabled = true
				}
			} else {
				if ip.To16().IsGlobalUnicast() {
					ipv6Enabled = true
				}
			}
			if ipv4Enabled && ipv6Enabled {
				break
			}
		}
	}
	return
}

type deadlineConn struct {
	net.Conn
	deadline time.Duration
}

func (d *deadlineConn) Read(p []byte) (n int, err error) {
	_ = d.Conn.SetReadDeadline(time.Now().Add(d.deadline))
	return d.Conn.Read(p)
}

func (d *deadlineConn) Write(p []byte) (n int, err error) {
	_ = d.Conn.SetWriteDeadline(time.Now().Add(d.deadline))
	return d.Conn.Write(p)
}

// DeadlineConn is used to return a net.Conn that SetReadDeadline()
// and SetWriteDeadline() before each Read() and Write().
func DeadlineConn(conn net.Conn, deadline time.Duration) net.Conn {
	dc := deadlineConn{
		Conn:     conn,
		deadline: deadline,
	}
	if dc.deadline < 1 {
		dc.deadline = time.Minute
	}
	return &dc
}
