package xlight

import (
	"net"
	"sync"
	"time"

	"project/internal/options"
)

// timeout is for handshake
func Dial(network, address string, timeout time.Duration) (*Conn, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return Client(conn, timeout), nil
}

func Client(conn net.Conn, timeout time.Duration) *Conn {
	return &Conn{Conn: conn, handshake_timeout: timeout, is_client: true}
}

func Server(conn net.Conn, timeout time.Duration) *Conn {
	return &Conn{Conn: conn, handshake_timeout: timeout}
}

type Conn struct {
	net.Conn
	is_client         bool
	handshake_timeout time.Duration
	handshake_err     error
	handshake_m       sync.Mutex
	handshake_once    sync.Once
	cryptor           *cryptor
}

func (this *Conn) Handshake() error {
	defer this.handshake_m.Unlock()
	this.handshake_m.Lock()
	if this.handshake_err != nil {
		return this.handshake_err
	}
	this.handshake_once.Do(func() {
		// default handshake timeout
		if this.handshake_timeout < 1 {
			this.handshake_timeout = options.DEFAULT_HANDSHAKE_TIMEOUT
		}
		deadline := time.Now().Add(this.handshake_timeout)
		this.handshake_err = this.SetDeadline(deadline)
		if this.handshake_err != nil {
			return
		}
		if this.is_client {
			this.handshake_err = this.client_handshake()
		} else {
			this.handshake_err = this.server_handshake()
		}
		if this.handshake_err != nil {
			return
		}
		this.handshake_err = this.SetDeadline(time.Time{})
		if this.handshake_err != nil {
			return
		}
	})
	return this.handshake_err
}

func (this *Conn) Read(b []byte) (n int, err error) {
	err = this.Handshake()
	if err != nil {
		return
	}
	n, err = this.Conn.Read(b)
	if err != nil {
		return
	}
	this.cryptor.decrypt(b)
	return n, nil
}

func (this *Conn) Write(b []byte) (n int, err error) {
	err = this.Handshake()
	if err != nil {
	}
	return this.Conn.Write(this.cryptor.encrypt(b))
}
