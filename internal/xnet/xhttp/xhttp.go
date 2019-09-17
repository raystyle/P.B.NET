package xhttp

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

type Conn struct {
}

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
	if l.server.TLSConfig == nil {
		_ = l.server.Serve(l.listener)
	} else {
		_ = l.server.ServeTLS(l.listener, "", "")
	}
}

func (l *listener) handleRequest(w http.ResponseWriter, r *http.Request) {
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
	server := http.Server{TLSConfig: cfg}
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

func Dial(r *http.Request, tr *http.Transport, timeout time.Duration) (net.Conn, error) {
	client := http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
	resp, err := client.Do(r)
	if err != nil {
		return nil, err
	}
	conn, _, err := resp.Body.(http.Hijacker).Hijack()
	return conn, nil
}
