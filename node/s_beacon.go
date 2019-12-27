package node

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/xnet"
	"project/internal/xpanic"
)

type beaconConn struct {
	ctx *Node

	tag  string
	guid []byte // beacon guid
	conn *conn

	heartbeat bytes.Buffer
	rand      *random.Rand
	inSync    int32

	sendPool  sync.Pool
	ackPool   sync.Pool
	queryPool sync.Pool

	closeOnce sync.Once
}

func (s *server) serveBeacon(tag string, beaconGUID []byte, conn *conn) {
	bc := beaconConn{
		ctx:  s.ctx,
		tag:  tag,
		guid: beaconGUID,
		conn: conn,
		rand: random.New(),
	}
	defer func() {
		if r := recover(); r != nil {
			bc.log(logger.Exploit, xpanic.Error(r, "server.serveNode"))
		}
		bc.Close()
		if bc.isSync() {
			s.ctx.forwarder.LogoffBeacon(tag)
		}
		s.deleteBeaconConn(tag)
		bc.logf(logger.Debug, "beacon %X disconnected", beaconGUID)
	}()
	s.addBeaconConn(tag, &bc)
	_ = conn.SetDeadline(s.ctx.global.Now().Add(s.timeout))
	bc.logf(logger.Debug, "beacon %X connected", beaconGUID)
	protocol.HandleConn(conn, bc.onFrame)
}

// TODO add guid
func (beacon *beaconConn) log(l logger.Level, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintln(b, log...)
	_, _ = fmt.Fprint(b, "\n", beacon.conn)
	beacon.ctx.logger.Print(l, "serve-beacon", b)
}

func (beacon *beaconConn) logf(l logger.Level, format string, log ...interface{}) {
	b := new(bytes.Buffer)
	_, _ = fmt.Fprintf(b, format, log...)
	_, _ = fmt.Fprint(b, "\n\n", beacon.conn)
	beacon.ctx.logger.Print(l, "serve-beacon", b)
}

func (beacon *beaconConn) isSync() bool {
	return atomic.LoadInt32(&beacon.inSync) != 0
}

func (beacon *beaconConn) onFrame(frame []byte) {
	if beacon.conn.onFrame(frame) {
		return
	}
	if frame[0] == protocol.ConnSendHeartbeat {
		beacon.handleHeartbeat()
		return
	}
	id := frame[protocol.FrameCMDSize : protocol.FrameCMDSize+protocol.FrameIDSize]
	data := frame[protocol.FrameCMDSize+protocol.FrameIDSize:]
	if beacon.isSync() {
		if beacon.onFrameAfterSync(frame[0], id, data) {
			return
		}
	} else {
		if beacon.onFrameBeforeSync(frame[0], id, data) {
			return
		}
	}
	const format = "unknown command: %d\nframe:\n%s"
	beacon.logf(logger.Exploit, format, frame[0], spew.Sdump(frame))
	beacon.Close()
}

func (beacon *beaconConn) onFrameBeforeSync(cmd byte, id, data []byte) bool {
	switch cmd {
	case protocol.NodeSync:
		beacon.handleSyncStart(id)
	default:
		return false
	}
	return true
}

func (beacon *beaconConn) onFrameAfterSync(cmd byte, id, data []byte) bool {
	switch cmd {
	default:
		return false
	}
	return true
}

func (beacon *beaconConn) handleHeartbeat() {
	// <security> fake traffic like client
	fakeSize := 64 + beacon.rand.Int(256)
	// size(4 Bytes) + heartbeat(1 byte) + fake data
	beacon.heartbeat.Reset()
	beacon.heartbeat.Write(convert.Uint32ToBytes(uint32(1 + fakeSize)))
	beacon.heartbeat.WriteByte(protocol.ConnReplyHeartbeat)
	beacon.heartbeat.Write(beacon.rand.Bytes(fakeSize))
	// send heartbeat data
	_ = beacon.conn.SetWriteDeadline(time.Now().Add(protocol.SendTimeout))
	_, _ = beacon.conn.Write(beacon.heartbeat.Bytes())
}

func (beacon *beaconConn) handleSyncStart(id []byte) {
	if beacon.isSync() {
		return
	}
	beacon.sendPool.New = func() interface{} {
		return &protocol.Send{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			Message:   make([]byte, aes.BlockSize),
			Hash:      make([]byte, sha256.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	beacon.ackPool.New = func() interface{} {
		return &protocol.Acknowledge{
			GUID:      make([]byte, guid.Size),
			RoleGUID:  make([]byte, guid.Size),
			SendGUID:  make([]byte, guid.Size),
			Signature: make([]byte, ed25519.SignatureSize),
		}
	}
	beacon.queryPool.New = func() interface{} {
		return &protocol.Query{
			GUID:       make([]byte, guid.Size),
			BeaconGUID: make([]byte, guid.Size),
			Signature:  make([]byte, ed25519.SignatureSize),
		}
	}
	err := beacon.ctx.forwarder.RegisterBeacon(beacon.tag, beacon)
	if err != nil {
		beacon.conn.Reply(id, []byte(err.Error()))
		beacon.Close()
	} else {
		atomic.StoreInt32(&beacon.inSync, 1)
		beacon.conn.Reply(id, []byte{protocol.NodeSync})
		beacon.log(logger.Debug, "synchronizing")
	}
}

// Status is used to get connection status
func (beacon *beaconConn) Status() *xnet.Status {
	return beacon.conn.Status()
}

// Close is used to stop serve node
func (beacon *beaconConn) Close() {
	beacon.closeOnce.Do(func() {
		_ = beacon.conn.Close()
	})
}
