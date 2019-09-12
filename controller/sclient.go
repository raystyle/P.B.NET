package controller

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xpanic"
)

// syncer client
type sClient struct {
	ctx    *syncer
	guid   []byte
	client *client
}

func newSClient(ctx *syncer, cfg *clientCfg) (*sClient, error) {
	sc := sClient{
		ctx: ctx,
	}
	cfg.MsgHandler = sc.handleMessage
	client, err := newClient(ctx.ctx, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "new syncer client failed")
	}
	sc.guid = cfg.NodeGUID
	sc.client = client
	// start handle
	// <warning> not add wg
	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := xpanic.Error("syncer client panic:", r)
				client.log(logger.FATAL, err)
			}
			client.Close()
		}()
		protocol.HandleConn(client.conn, sc.handleMessage)
	}()
	// send start sync cmd
	resp, err := client.Send(protocol.CtrlSyncStart, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "receive sync start response failed")
	}
	if !bytes.Equal(resp, []byte{protocol.CtrlSyncStart}) {
		err = errors.WithMessage(err, "invalid sync start response")
		sc.log(logger.EXPLOIT, err)
		return nil, err
	}
	return &sc, nil
}

func (sc *sClient) Broadcast(token, message []byte) *protocol.BroadcastResponse {
	br := protocol.BroadcastResponse{}
	br.Role = protocol.Node
	br.GUID = sc.guid
	reply, err := sc.client.Send(protocol.CtrlBroadcastToken, token)
	if err != nil {
		br.Err = err
		return &br
	}
	if !bytes.Equal(reply, protocol.BroadcastUnhandled) {
		br.Err = protocol.ErrBroadcastHandled
		return &br
	}
	// broadcast
	reply, err = sc.client.Send(protocol.CtrlBroadcast, message)
	if err != nil {
		br.Err = err
		return &br
	}
	if bytes.Equal(reply, protocol.BroadcastSucceed) {
		return &br
	} else {
		br.Err = errors.New(string(reply))
		return &br
	}
}

func (sc *sClient) SyncSend(token, message []byte) *protocol.SyncResponse {
	sr := &protocol.SyncResponse{}
	sr.Role = protocol.Node
	sr.GUID = sc.guid
	resp, err := sc.client.Send(protocol.CtrlSyncSendToken, token)
	if err != nil {
		sr.Err = err
		return sr
	}
	if !bytes.Equal(resp, protocol.SyncUnhandled) {
		sr.Err = protocol.ErrSyncHandled
		return sr
	}
	resp, err = sc.client.Send(protocol.CtrlSyncSend, message)
	if err != nil {
		sr.Err = err
		return sr
	}
	if bytes.Equal(resp, protocol.SyncSucceed) {
		return sr
	} else {
		sr.Err = errors.New(string(resp))
		return sr
	}
}

// notice node clean the message
func (sc *sClient) SyncReceive(token, message []byte) *protocol.SyncResponse {
	sr := &protocol.SyncResponse{}
	sr.Role = protocol.Node
	sr.GUID = sc.guid
	resp, err := sc.client.Send(protocol.CtrlSyncRecvToken, token)
	if err != nil {
		sr.Err = err
		return sr
	}
	if !bytes.Equal(resp, protocol.SyncUnhandled) {
		sr.Err = protocol.ErrSyncHandled
		return sr
	}
	resp, err = sc.client.Send(protocol.CtrlSyncRecv, message)
	if err != nil {
		sr.Err = err
		return sr
	}
	if bytes.Equal(resp, protocol.SyncSucceed) {
		return sr
	} else {
		sr.Err = errors.New(string(resp))
		return sr
	}
}

func (sc *sClient) QueryMessage(request []byte) (*protocol.SyncReply, error) {
	reply, err := sc.client.Send(protocol.CtrlSyncQuery, request)
	if err != nil {
		return nil, err
	}
	sr := protocol.SyncReply{}
	err = msgpack.Unmarshal(reply, &sr)
	if err != nil {
		err = errors.Wrap(err, "invalid sync reply")
		sc.log(logger.EXPLOIT, err)
		sc.Close()
		return nil, err
	}
	return &sr, nil
}

func (sc *sClient) Close() {
	sc.client.Close()
}

func (sc *sClient) logf(l logger.Level, format string, log ...interface{}) {
	b := logger.Conn(sc.client.conn)
	_, _ = fmt.Fprintf(b, format, log...)
	sc.ctx.ctx.Print(l, "syncer-client", b)
}

func (sc *sClient) log(l logger.Level, log ...interface{}) {
	b := logger.Conn(sc.client.conn)
	_, _ = fmt.Fprint(b, log...)
	sc.ctx.ctx.Print(l, "syncer-client", b)
}

func (sc *sClient) logln(l logger.Level, log ...interface{}) {
	b := logger.Conn(sc.client.conn)
	_, _ = fmt.Fprintln(b, log...)
	sc.ctx.ctx.Print(l, "syncer-client", b)
}

// can use client.Close()
func (sc *sClient) handleMessage(msg []byte) {
	const (
		cmd = protocol.MsgCMDSize
		id  = protocol.MsgCMDSize + protocol.MsgIDSize
	)
	if sc.client.isClosed() {
		return
	}
	// cmd(1) + msg id(2) or reply
	if len(msg) < id {
		sc.log(logger.EXPLOIT, protocol.ErrInvalidMsgSize)
		sc.Close()
		return
	}
	switch msg[0] {
	case protocol.NodeSyncSendToken:
		sc.handleSyncSendToken(msg[cmd:id], msg[id:])
	case protocol.NodeSyncSend:
		sc.handleSyncSend(msg[cmd:id], msg[id:])
	case protocol.NodeSyncRecvToken:
		sc.handleSyncReceiveToken(msg[cmd:id], msg[id:])
	case protocol.NodeSyncRecv:
		sc.handleSyncReceive(msg[cmd:id], msg[id:])
	case protocol.NodeBroadcastToken:
		sc.handleBroadcastToken(msg[cmd:id], msg[id:])
	case protocol.NodeBroadcast:
		sc.handleBroadcast(msg[cmd:id], msg[id:])
	// ---------------------------internal--------------------------------
	case protocol.NodeReply:
		sc.client.handleReply(msg[cmd:])
	case protocol.NodeHeartbeat:
		sc.client.heartbeatC <- struct{}{}
	case protocol.ErrNullMsg:
		sc.log(logger.EXPLOIT, protocol.ErrRecvNullMsg)
		sc.Close()
	case protocol.ErrTooBigMsg:
		sc.log(logger.EXPLOIT, protocol.ErrRecvTooBigMsg)
		sc.Close()
	case protocol.TestMessage:
		sc.client.Reply(msg[cmd:id], msg[id:])
	default:
		sc.log(logger.EXPLOIT, protocol.ErrRecvUnknownCMD, msg)
		sc.Close()
		return
	}
}

func (sc *sClient) handleBroadcastToken(id, message []byte) {
	// role + message guid
	if len(message) != 1+guid.SIZE {
		// fake reply and close
		sc.client.Reply(id, protocol.BroadcastHandled)
		sc.log(logger.EXPLOIT, "invalid broadcast token size")
		sc.Close()
		return
	}
	if sc.ctx.checkBroadcastToken(message[0], message[1:]) {
		sc.client.Reply(id, protocol.BroadcastUnhandled)
	} else {
		sc.client.Reply(id, protocol.BroadcastHandled)
	}
}

func (sc *sClient) handleSyncSendToken(id, message []byte) {
	if len(message) != 1+guid.SIZE {
		// fake reply and close
		sc.client.Reply(id, protocol.SyncHandled)
		sc.log(logger.EXPLOIT, "invalid sync send token size")
		sc.Close()
		return
	}
	if sc.ctx.checkSyncSendToken(message[0], message[1:]) {
		sc.client.Reply(id, protocol.SyncUnhandled)
	} else {
		sc.client.Reply(id, protocol.SyncHandled)
	}
}

func (sc *sClient) handleSyncReceiveToken(id, message []byte) {
	if len(message) != 1+guid.SIZE {
		// fake reply and close
		sc.client.Reply(id, protocol.SyncHandled)
		sc.log(logger.EXPLOIT, "invalid sync receive token size")
		sc.Close()
		return
	}
	if sc.ctx.checkSyncReceiveToken(message[0], message[1:]) {
		sc.client.Reply(id, protocol.SyncUnhandled)
	} else {
		sc.client.Reply(id, protocol.SyncHandled)
	}
}

func (sc *sClient) handleBroadcast(id, message []byte) {
	br := protocol.Broadcast{}
	err := msgpack.Unmarshal(message, &br)
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid broadcast msgpack data")
		sc.Close()
		return
	}
	err = br.Validate()
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid broadcast", err)
		sc.Close()
		return
	}
	// TODO check role  and check sender role
	if br.ReceiverRole != protocol.Ctrl {

		return
	}

	sc.ctx.addBroadcast(&br)
	sc.client.Reply(id, protocol.BroadcastSucceed)
}

// Node -> Controller(direct)
func (sc *sClient) handleSyncSend(id, message []byte) {
	ss := protocol.SyncSend{}
	err := msgpack.Unmarshal(message, &ss)
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid sync send msgpack data")
		sc.Close()
		return
	}
	err = ss.Validate()
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid sync send", err)
		sc.Close()
		return
	}
	sc.ctx.addSyncSend(&ss)
	sc.client.Reply(id, protocol.SyncSucceed)
}

// notice controller role received this height message
func (sc *sClient) handleSyncReceive(id, message []byte) {
	sr := protocol.SyncReceive{}
	err := msgpack.Unmarshal(message, &sr)
	if err != nil {
		sc.log(logger.EXPLOIT, "invalid sync receive msgpack data")
		sc.Close()
		return
	}
	err = sr.Validate()
	if err != nil {
		// TODO spew it
		sc.log(logger.EXPLOIT, "invalid sync receive:", err)
		sc.Close()
		return
	}
	sc.ctx.addSyncReceive(&sr)
	sc.client.Reply(id, protocol.SyncSucceed)
}
