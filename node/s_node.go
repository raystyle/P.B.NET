package node

import (
	"bytes"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/protocol"
)

func (c *conn) onFrameServeNode(frame []byte) {
	if c.onFrame(frame) {
		return
	}
	// check command
	switch frame[0] {

	default:
		c.log(logger.Exploit, protocol.ErrRecvUnknownCMD, frame)
		c.Close()
	}
}

type nodeConn struct {
	ctx *Node

	guid []byte
	tag  string
	conn *conn
}

// Send is used to send message to connected controller
func (node *nodeConn) Send(guid, message []byte) (sr *protocol.SendResponse) {
	sr = &protocol.SendResponse{
		Role: protocol.Node,
		GUID: node.guid,
	}
	var reply []byte
	reply, sr.Err = node.conn.Send(protocol.NodeSendGUID, guid)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		sr.Err = protocol.GetReplyError(reply)
		return
	}
	reply, sr.Err = node.conn.Send(protocol.NodeSend, message)
	if sr.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		sr.Err = errors.New(string(reply))
	}
	return
}

// Acknowledge is used to acknowledge to controller
func (node *nodeConn) Acknowledge(guid, message []byte) (ar *protocol.AcknowledgeResponse) {
	ar = &protocol.AcknowledgeResponse{
		Role: protocol.Node,
		GUID: node.guid,
	}
	var reply []byte
	reply, ar.Err = node.conn.Send(protocol.NodeAckGUID, guid)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplyUnhandled) {
		ar.Err = protocol.GetReplyError(reply)
		return
	}
	reply, ar.Err = node.conn.Send(protocol.NodeAck, message)
	if ar.Err != nil {
		return
	}
	if !bytes.Equal(reply, protocol.ReplySucceed) {
		ar.Err = errors.New(string(reply))
	}
	return
}
