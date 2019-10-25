package xnetutil

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"project/internal/options"
)

var ErrEmptyPort = errors.New("empty port")

type InvalidPortError int

func (p InvalidPortError) Error() string {
	return fmt.Sprintf("invalid port: %d", p)
}

func CheckPort(port int) error {
	if port < 1 || port > 65535 {
		return InvalidPortError(port)
	}
	return nil
}

func CheckPortString(port string) error {
	if port == "" {
		return ErrEmptyPort
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return err
	}
	return CheckPort(n)
}

type TrafficUnit int

func (ts TrafficUnit) String() string {
	const (
		kb = 1 << 10
		mb = 1 << 20
		gb = 1 << 30
		tb = 1 << 40
	)
	switch {
	case ts < kb:
		return fmt.Sprintf("%d Byte", ts)
	case ts < mb:
		return fmt.Sprintf("%.3f KB", float64(ts)/kb)
	case ts < gb:
		return fmt.Sprintf("%.3f MB", float64(ts)/mb)
	case ts < tb:
		return fmt.Sprintf("%.3f GB", float64(ts)/gb)
	default:
		return fmt.Sprintf("%.3f TB", float64(ts)/tb)
	}
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
// and SetWriteDeadline() before each Read and Write
func DeadlineConn(conn net.Conn, deadline time.Duration) net.Conn {
	dc := deadlineConn{
		Conn:     conn,
		deadline: deadline,
	}
	if dc.deadline < 1 {
		dc.deadline = options.DefaultDeadline
	}
	return &dc
}
