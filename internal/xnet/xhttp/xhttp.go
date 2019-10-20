package xhttp

import (
	"bytes"
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
	serveMux.HandleFunc("/a", listener.handleRequest)
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
	if tr == nil {
		tr = new(http.Transport)
	}
	conn, err := net.Dial("tcp", r.URL.Host)
	if err != nil {
		return nil, err
	}
	buf := bytes.Buffer{}
	buf.WriteString("GET /a HTTP/1.1\r\n")
	buf.WriteString("Host: " + r.URL.Host + "\r\n")
	buf.WriteString("User-Agent: Go-http-client/1.1\r\n")
	buf.WriteString("\r\n")
	_, err = conn.Write(buf.Bytes())
	if err != nil {
		return nil, err
	}
	// b, err := ioutil.ReadAll(conn)
	// fmt.Println(string(b))

	time.Sleep(100 * time.Millisecond)
	return conn, nil
}
