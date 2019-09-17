package xhttp

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

var (
	disableHTTP2Server = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
)

type listener struct {
	listener   net.Listener
	server     *http.Server
	connChan   chan net.Conn
	stopSignal chan struct{}
}

func (l *listener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connChan:
		return conn, nil
	case <-l.stopSignal:
		return nil, http.ErrServerClosed
	}
}

func (l *listener) Close() error {
	return l.server.Close()
}

func (l *listener) Addr() net.Addr {
	return l.listener.Addr()
}

func (l *listener) deploy() {
	var err error
	if l.server.TLSConfig == nil {
		err = l.server.Serve(l.listener)
	} else {
		err = l.server.ServeTLS(l.listener, "", "")
	}
	if err != http.ErrServerClosed {
		_ = l.server.Close()
	}
}

func (l *listener) handleRequest(w http.ResponseWriter, r *http.Request) {
	defer func() { recover() }()
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}
	select {
	case l.connChan <- conn:
	case <-l.stopSignal:
		return
	}
}

func newListener(network, address string, cfg *tls.Config, timeout time.Duration) (*listener, error) {
	l, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	listener := listener{
		listener:   l,
		connChan:   make(chan net.Conn, 1),
		stopSignal: make(chan struct{}),
	}
	server := http.Server{
		TLSConfig:         cfg,
		ReadHeaderTimeout: timeout,
		TLSNextProto:      disableHTTP2Server,
	}
	server.RegisterOnShutdown(func() {
		close(listener.stopSignal)
	})
	serveMux := http.ServeMux{}
	serveMux.HandleFunc("/", listener.handleRequest)
	server.Handler = &serveMux
	listener.server = &server
	go listener.deploy()
	return &listener, nil
}

func Listen(network, address string, timeout time.Duration) (net.Listener, error) {
	return newListener(network, address, nil, timeout)
}

func ListenTLS(network, address string, cfg *tls.Config, timeout time.Duration) (net.Listener, error) {
	return newListener(network, address, cfg, timeout)
}

// TODO finish Dial
func Dial(r *http.Request, tr *http.Transport, timeout time.Duration) (net.Conn, error) {
	if tr == nil {
		tr = new(http.Transport)
	}
	conn, err := net.Dial("tcp", r.Host)
	if err != nil {
		return nil, err
	}
	err = r.Write(conn)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
