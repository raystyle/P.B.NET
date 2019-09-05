package logger

import (
	"bytes"
	"fmt"
	"net"
)

func Conn(conn net.Conn) *bytes.Buffer {
	b := bytes.Buffer{}
	_, _ = fmt.Fprintf(&b, "%s %s <-> %s %s ",
		conn.LocalAddr().Network(), conn.LocalAddr(),
		conn.RemoteAddr().Network(), conn.RemoteAddr())
	return &b
}
