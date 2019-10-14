package controller

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

type syncerClient struct {
	ctx       *syncer
	Node      *bootstrap.Node
	guid      []byte
	client    *client
	closeOnce sync.Once
}

// TODO error
func newSyncerClient(ctx *CTRL, cfg *clientCfg) (*syncerClient, error) {
	sc := syncerClient{
		ctx: ctx.syncer,
	}
	cfg.MsgHandler = sc.handleMessage
	client, err := newClient(ctx, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "new syncer client failed")
	}
	sc.Node = cfg.Node
	sc.guid = cfg.NodeGUID
	sc.client = client
	// start handle
	// <warning> not add wg
	go func() {
		defer func() {
			if r := recover(); r != nil {
				client.log(logger.Fatal, xpanic.Error("syncer client panic:", r))
			}
			sc.Close()
		}()
		protocol.HandleConn(client.conn, sc.handleMessage)
	}()
	// send start sync cmd
	resp, err := client.Send(protocol.CtrlSyncStart, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "receive sync start response failed")
	}
	if !bytes.Equal(resp, []byte{protocol.NodeSyncStart}) {
		err = errors.Errorf("invalid sync start response: %s", string(resp))
		sc.log(logger.Exploit, err)
		return nil, err
	}
	return &sc, nil
}

// TODO error
func (sc *syncerClient) Broadcast(guid, data []byte) (br *protocol.BroadcastResponse) {
	br = &protocol.BroadcastResponse{
		GUID: sc.guid,
	}
	var reply []byte
	reply, br.Err = sc.client.Send(protocol.CtrlBroadcastGUID, guid)
	if br.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.BroadcastUnhandled) {
		br.Err = errors.New(string(reply))
		return
	}
	// broadcast
	reply, br.Err = sc.client.Send(protocol.CtrlBroadcast, data)
	if br.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.BroadcastSucceed) {
		br.Err = errors.New(string(reply))
	}
	return
}

func (sc *syncerClient) SendToNode(guid, data []byte) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: sc.guid,
	}
	var reply []byte
	reply, sr.Err = sc.client.Send(protocol.CtrlSendToNodeGUID, guid)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendUnhandled) {
		sr.Err = errors.New(string(reply))
		return
	}
	reply, sr.Err = sc.client.Send(protocol.CtrlSendToNode, data)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendSucceed) {
		sr.Err = errors.New(string(reply))
	}
	return
}

func (sc *syncerClient) SendToBeacon(guid, data []byte) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: sc.guid,
	}
	var reply []byte
	reply, sr.Err = sc.client.Send(protocol.CtrlSendToBeaconGUID, guid)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendUnhandled) {
		sr.Err = errors.New(string(reply))
		return
	}
	reply, sr.Err = sc.client.Send(protocol.CtrlSendToBeacon, data)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendSucceed) {
		sr.Err = errors.New(string(reply))
	}
	return
}

// AcknowledgeToNode is used to notice Node that
// Controller has received this message
func (sc *syncerClient) AcknowledgeToNode(guid, data []byte) {
	var (
		reply []byte
		err   error
	)
	defer func() {
		if err != nil {
			sc.logln(logger.Error, "acknowledge to node failed:", err)
		}
	}()
	reply, err = sc.client.Send(protocol.CtrlAckToNodeGUID, guid)
	if err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendUnhandled) {
		return
	}
	reply, err = sc.client.Send(protocol.CtrlAckToNode, data)
	if err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendSucceed) {
		err = errors.New(string(reply))
	}
}

// AcknowledgeToBeacon is used to notice Beacon that
// Controller has received this message
func (sc *syncerClient) AcknowledgeToBeacon(guid, data []byte) {
	var (
		reply []byte
		err   error
	)
	defer func() {
		if err != nil {
			sc.logln(logger.Error, "acknowledge to beacon failed:", err)
		}
	}()
	reply, err = sc.client.Send(protocol.CtrlAckToBeaconGUID, guid)
	if err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendUnhandled) {
		return
	}
	reply, err = sc.client.Send(protocol.CtrlAckToBeacon, data)
	if err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.SendSucceed) {
		err = errors.New(string(reply))
	}
}

func (sc *syncerClient) Close() {
	sc.closeOnce.Do(func() {
		sc.client.Close()
		key := hex.EncodeToString(sc.guid)
		sc.ctx.ctx.clientsRWM.Lock()
		delete(sc.ctx.ctx.clients, key)
		sc.ctx.ctx.clientsRWM.Unlock()
	})
}

func (sc *syncerClient) logf(l logger.Level, format string, log ...interface{}) {
	b := logger.Conn(sc.client.conn)
	_, _ = fmt.Fprintf(b, format, log...)
	sc.ctx.ctx.Print(l, "syncer-client", b)
}

func (sc *syncerClient) log(l logger.Level, log ...interface{}) {
	b := logger.Conn(sc.client.conn)
	_, _ = fmt.Fprint(b, log...)
	sc.ctx.ctx.Print(l, "syncer-client", b)
}

func (sc *syncerClient) logln(l logger.Level, log ...interface{}) {
	b := logger.Conn(sc.client.conn)
	_, _ = fmt.Fprintln(b, log...)
	sc.ctx.ctx.Print(l, "syncer-client", b)
}

// can use client.Close()
func (sc *syncerClient) handleMessage(msg []byte) {
	const (
		cmd = protocol.MsgCMDSize
		id  = protocol.MsgCMDSize + protocol.MsgIDSize
	)
	if sc.client.closing() {
		return
	}
	// cmd(1) + msg id(2) or reply
	if len(msg) < id {
		sc.log(logger.Exploit, protocol.ErrInvalidMsgSize)
		sc.Close()
		return
	}
	switch msg[0] {
	case protocol.BeaconQueryGUID:
		sc.handleBeaconQueryGUID(msg[cmd:id], msg[id:])
	case protocol.BeaconQuery:
		sc.handleBeaconQuery(msg[cmd:id], msg[id:])
	case protocol.BeaconSendGUID:
		sc.handleBeaconSendGUID(msg[cmd:id], msg[id:])
	case protocol.BeaconSend:
		sc.handleBeaconSend(msg[cmd:id], msg[id:])
	case protocol.NodeSendGUID:
		sc.handleNodeSendGUID(msg[cmd:id], msg[id:])
	case protocol.NodeSend:
		sc.handleNodeSend(msg[cmd:id], msg[id:])
	// ---------------------------internal--------------------------------
	case protocol.NodeReply:
		sc.client.handleReply(msg[cmd:])
	case protocol.NodeHeartbeat:
		sc.client.heartbeat <- struct{}{}
	case protocol.ErrCMDRecvNullMsg:
		sc.log(logger.Exploit, protocol.ErrRecvNullMsg)
		sc.Close()
	case protocol.ErrCMDTooBigMsg:
		sc.log(logger.Exploit, protocol.ErrRecvTooBigMsg)
		sc.Close()
	case protocol.TestCommand:
		sc.client.Reply(msg[cmd:id], msg[id:])
	default:
		sc.logln(logger.Exploit, protocol.ErrRecvUnknownCMD, msg)
		sc.Close()
		return
	}
}

func (sc *syncerClient) handleNodeSendGUID(id, guid_ []byte) {
	if len(guid_) != guid.Size {
		// fake reply and close
		sc.log(logger.Exploit, "invalid node send guid size")
		sc.client.Reply(id, protocol.SendHandled)
		sc.Close()
		return
	}
	if expired, _ := sc.ctx.CheckGUIDTimestamp(guid_); expired {
		sc.client.Reply(id, protocol.SendExpired)
	} else if sc.ctx.CheckNodeSendGUID(guid_, false, 0) {
		sc.client.Reply(id, protocol.SendUnhandled)
	} else {
		sc.client.Reply(id, protocol.SendHandled)
	}
}

func (sc *syncerClient) handleBeaconSendGUID(id, guid_ []byte) {
	if len(guid_) != guid.Size {
		// fake reply and close
		sc.log(logger.Exploit, "invalid beacon send guid size")
		sc.client.Reply(id, protocol.SendHandled)
		sc.Close()
		return
	}
	if expired, _ := sc.ctx.CheckGUIDTimestamp(guid_); expired {
		sc.client.Reply(id, protocol.SendExpired)
	} else if sc.ctx.CheckBeaconSendGUID(guid_, false, 0) {
		sc.client.Reply(id, protocol.SendUnhandled)
	} else {
		sc.client.Reply(id, protocol.SendHandled)
	}
}

func (sc *syncerClient) handleBeaconQueryGUID(id, guid_ []byte) {
	if len(guid_) != guid.Size {
		// fake reply and close
		sc.log(logger.Exploit, "invalid beacon query guid size")
		sc.client.Reply(id, protocol.SendHandled)
		sc.Close()
		return
	}
	if expired, _ := sc.ctx.CheckGUIDTimestamp(guid_); expired {
		sc.client.Reply(id, protocol.SendExpired)
	} else if sc.ctx.CheckBeaconQueryGUID(guid_, false, 0) {
		sc.client.Reply(id, protocol.SendUnhandled)
	} else {
		sc.client.Reply(id, protocol.SendHandled)
	}
}

func (sc *syncerClient) handleNodeSend(id, data []byte) {
	s := protocol.Send{}
	err := msgpack.Unmarshal(data, &s)
	if err != nil {
		sc.logln(logger.Exploit, "invalid node send msgpack data:", err)
		sc.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		sc.logf(logger.Exploit, "invalid node send: %s\n%s", err, spew.Sdump(s))
		sc.Close()
		return
	}
	if expired, timestamp := sc.ctx.CheckGUIDTimestamp(s.GUID); expired {
		sc.client.Reply(id, protocol.SendExpired)
	} else if sc.ctx.CheckNodeSendGUID(s.GUID, true, timestamp) {
		sc.client.Reply(id, protocol.SendSucceed)
		sc.ctx.AddNodeSend(&s)
	} else {
		sc.client.Reply(id, protocol.SendHandled)
	}
}

func (sc *syncerClient) handleBeaconSend(id, data []byte) {
	s := protocol.Send{}
	err := msgpack.Unmarshal(data, &s)
	if err != nil {
		sc.logln(logger.Exploit, "invalid beacon send msgpack data:", err)
		sc.Close()
		return
	}
	err = s.Validate()
	if err != nil {
		sc.logf(logger.Exploit, "invalid beacon send: %s\n%s", err, spew.Sdump(s))
		sc.Close()
		return
	}
	if expired, timestamp := sc.ctx.CheckGUIDTimestamp(s.GUID); expired {
		sc.client.Reply(id, protocol.SendExpired)
	} else if sc.ctx.CheckBeaconSendGUID(s.GUID, true, timestamp) {
		sc.client.Reply(id, protocol.SendSucceed)
		sc.ctx.AddBeaconSend(&s)
	} else {
		sc.client.Reply(id, protocol.SendHandled)
	}
}

func (sc *syncerClient) handleBeaconQuery(id, data []byte) {
	q := protocol.Query{}
	err := msgpack.Unmarshal(data, &q)
	if err != nil {
		sc.logln(logger.Exploit, "invalid beacon query msgpack data:", err)
		sc.Close()
		return
	}
	err = q.Validate()
	if err != nil {
		sc.logf(logger.Exploit, "invalid beacon query: %s\n%s", err, spew.Sdump(q))
		sc.Close()
		return
	}
	if expired, timestamp := sc.ctx.CheckGUIDTimestamp(q.GUID); expired {
		sc.client.Reply(id, protocol.SendExpired)
	} else if sc.ctx.CheckBeaconQueryGUID(q.GUID, true, timestamp) {
		sc.client.Reply(id, protocol.SendSucceed)
		sc.ctx.AddBeaconQuery(&q)
	} else {
		sc.client.Reply(id, protocol.SendHandled)
	}
}
