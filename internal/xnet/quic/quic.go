package quic

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"
)

const (
	defaultNextProto = "h3-23" // HTTP/3
)

// Conn implement net.Conn
type Conn struct {
	session quic.Session
	quic.Stream
}

func newConn(session quic.Session, stream quic.Stream) *Conn {
	return &Conn{
		session: session,
		Stream:  stream,
	}
}

func (c *Conn) LocalAddr() net.Addr {
	return c.session.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.session.RemoteAddr()
}

func (c *Conn) Close() error {
	_ = c.session.Close()
	_ = c.Stream.Close()
	return nil
}

type listener struct {
	quic.Listener
}

func (l *listener) Accept() (net.Conn, error) {
	session, err := l.Listener.Accept(context.Background())
	if err != nil {
		return nil, err
	}

	stream, err := session.AcceptStream(context.Background())
	if err != nil {
		_ = session.Close()
		return nil, err
	}

	return newConn(session, stream), nil
}

func Listen(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
) (net.Listener, error) {
	addr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP(network, addr)
	if err != nil {
		return nil, err
	}
	quicCfg := quic.Config{
		HandshakeTimeout: timeout,
		IdleTimeout:      timeout,
		KeepAlive:        true,
	}
	if len(config.NextProtos) == 0 {
		config.NextProtos = []string{defaultNextProto}
	}
	l, err := quic.Listen(conn, config, &quicCfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &listener{Listener: l}, nil
}

func Dial(
	network string,
	address string,
	config *tls.Config,
	timeout time.Duration,
) (*Conn, error) {
	rAddr, err := net.ResolveUDPAddr(network, address)
	if err != nil {
		return nil, err
	}
	var lAddr *net.UDPAddr
	switch network {
	case "udp", "udp4":
		lAddr = &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	case "udp6":
		lAddr = &net.UDPAddr{IP: net.IPv6zero, Port: 0}
	}
	conn, err := net.ListenUDP(network, lAddr)
	if err != nil {
		return nil, err
	}
	quicCfg := quic.Config{
		HandshakeTimeout: timeout,
		IdleTimeout:      timeout,
		KeepAlive:        true,
	}
	if len(config.NextProtos) == 0 {
		config.NextProtos = []string{defaultNextProto}
	}
	session, err := quic.Dial(conn, rAddr, address, config, &quicCfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	stream, err := session.OpenStream()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	return newConn(session, stream), nil
}
