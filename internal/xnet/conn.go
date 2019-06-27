package xnet

import (
	"io"
	"net"
	"sync"

	"project/internal/convert"
	"project/internal/protocol"
)

type Info struct {
	Local_Network  string
	Local_Address  string
	Remote_Network string
	Remote_Address string
	Connect_Time   int64
	Version        protocol.Version
	Send           int
	Receive        int
}

type Conn struct {
	net.Conn
	l_network string
	l_address string
	r_network string
	r_address string
	connect   int64 // timestamp
	version   protocol.Version
	send      int // imprecise
	receive   int // imprecise
	rwm       sync.RWMutex
}

func New_Conn(c net.Conn, now int64, v protocol.Version) *Conn {
	return &Conn{
		Conn:      c,
		l_network: c.LocalAddr().Network(),
		l_address: c.LocalAddr().String(),
		r_network: c.RemoteAddr().Network(),
		r_address: c.RemoteAddr().String(),
		connect:   now,
		version:   v,
	}
}

func (this *Conn) Read(b []byte) (int, error) {
	n, err := this.Conn.Read(b)
	this.rwm.Lock()
	this.receive += n
	this.rwm.Unlock()
	if err != nil {
		return n, err
	}
	return n, nil
}

func (this *Conn) Write(b []byte) (int, error) {
	n, err := this.Conn.Write(b)
	this.rwm.Lock()
	this.send += n
	this.rwm.Unlock()
	if err != nil {
		return n, err
	}
	return n, nil
}

// send message
func (this *Conn) Send(msg []byte) error {
	size := convert.Uint32_Bytes(uint32(len(msg)))
	_, err := this.Write(append(size, msg...))
	return err
}

// receive message
func (this *Conn) Receive() ([]byte, error) {
	size := make([]byte, 4)
	_, err := io.ReadFull(this, size)
	if err != nil {
		return nil, err
	}
	s := convert.Bytes_Uint32(size)
	msg := make([]byte, int(s))
	_, err = io.ReadFull(this, msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

func (this *Conn) Info() *Info {
	this.rwm.RLock()
	i := &Info{
		Send:    this.send,
		Receive: this.receive,
	}
	this.rwm.RUnlock()
	i.Local_Network = this.l_network
	i.Local_Address = this.l_address
	i.Remote_Network = this.r_network
	i.Remote_Address = this.r_address
	i.Connect_Time = this.connect
	i.Version = this.version
	return i
}
