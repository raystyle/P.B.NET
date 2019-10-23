package internal

import (
	"net"
	"time"

	"project/internal/options"
)

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
