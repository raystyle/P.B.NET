package controller

import (
	"bytes"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
)

// Node_GUID != nil for sync or other
// Node_GUID = nil for trust node
// Node_GUID = controller guid for discovery
type client_cfg struct {
	Node      *bootstrap.Node
	Node_GUID []byte
	Close_Log bool
	xnet.Config
}

type client struct {
	ctx         *CTRL
	node        *bootstrap.Node
	guid        []byte
	conn        *xnet.Conn
	close_log   bool
	slots       [protocol.SLOT_SIZE]*protocol.Slot
	in_close    int32
	close_once  sync.Once
	stop_signal chan struct{}
	wg          sync.WaitGroup
}

func new_client(ctx *CTRL, cfg *client_cfg) (*client, error) {
	cfg.Network = cfg.Node.Network
	cfg.Address = cfg.Node.Address
	// TODO add ca cert
	conn, err := xnet.Dial(cfg.Node.Mode, &cfg.Config)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	c := &client{
		ctx:       ctx,
		node:      cfg.Node,
		guid:      cfg.Node_GUID,
		close_log: cfg.Close_Log,
	}
	err_chan := make(chan error, 1)
	go func() {
		// TODO recover
		xconn, err := c.handshake(conn)
		if err != nil {
			err_chan <- err
			return
		}
		c.conn = xconn
		close(err_chan)
	}()
	select {
	case err = <-err_chan:
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
	case <-time.After(time.Minute):
		_ = conn.Close()
		return nil, errors.New("handshake timeout")
	}
	// init slot
	for i := 0; i < protocol.SLOT_SIZE; i++ {
		s := &protocol.Slot{
			Available: make(chan struct{}, 1),
			Reply:     make(chan []byte, 1),
		}
		s.Available <- struct{}{}
		c.slots[i] = s
	}
	go protocol.Handle_Conn(c.conn, c.handle_message, c.Close)
	c.wg.Add(1)
	go c.heartbeat()
	return c, nil
}

func (this *client) Close() {
	this.close_once.Do(func() {
		atomic.StoreInt32(&this.in_close, 1)
		close(this.stop_signal)
		_ = this.conn.Close()
		this.wg.Wait()
		if this.close_log {
			this.logln(logger.INFO, this.conn.Info().Remote_Address, "disconnected")
		}
	})
}

func (this *client) is_closed() bool {
	return atomic.LoadInt32(&this.in_close) != 0
}

func (this *client) logf(l logger.Level, format string, log ...interface{}) {
	this.ctx.Printf(l, "client", format, log...)
}

func (this *client) log(l logger.Level, log ...interface{}) {
	this.ctx.Print(l, "client", log...)
}

func (this *client) logln(l logger.Level, log ...interface{}) {
	this.ctx.Println(l, "client", log...)
}

func (this *client) handle_message(msg []byte) {

}

func (this *client) heartbeat() {
	defer this.wg.Done()
	rand := random.New()
	buffer := bytes.NewBuffer(nil)
	for {
		t := time.Duration(30+rand.Int(60)) * time.Second
		select {
		case <-time.After(t):
			// <security> fake flow like client
			fake_size := 64 + rand.Int(256)
			// size(4 Bytes) + heartbeat(1 byte) + fake data
			buffer.Write(convert.Uint32_Bytes(uint32(1 + fake_size)))
			buffer.WriteByte(protocol.CTRL_HEARTBEAT)
			buffer.Write(rand.Bytes(fake_size))
			_, err := this.conn.Write(buffer.Bytes())
			if err != nil {
				return
			}
			buffer.Reset()
		case <-this.stop_signal:
			return
		}
	}
}
