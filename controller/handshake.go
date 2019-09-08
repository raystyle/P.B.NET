package controller

import (
	"bytes"
	"net"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

func (client *client) handshake(c net.Conn) (*xnet.Conn, error) {
	conn := xnet.NewConn(c, client.ctx.global.Now().Unix())
	// receive certificate
	cert, err := conn.Receive()
	if err != nil {
		return nil, errors.Wrap(err, "receive certificate failed")
	}
	if !client.ctx.verifyCertificate(cert, client.node, client.guid) {
		err = errors.New("invalid certificate")
		client.log(logger.EXPLOIT, err)
		return nil, err
	}
	// send role
	_, err = conn.Write([]byte{protocol.Ctrl})
	if err != nil {
		return nil, errors.Wrap(err, "send role failed")
	}
	// receive challenge
	challenge, err := conn.Receive()
	if err != nil {
		return nil, errors.Wrap(err, "receive challenge data failed")
	}
	// <danger>
	// receive random challenge data(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and if controller sign it will destory net
	if len(challenge) < 2048 || len(challenge) > 4096 {
		err = errors.New("invalid challenge size")
		client.log(logger.EXPLOIT, err)
		return nil, err
	}
	// send signature
	err = conn.Send(client.ctx.global.Sign(challenge))
	if err != nil {
		return nil, errors.Wrap(err, "send challenge signature failed")
	}
	resp, err := conn.Receive()
	if err != nil {
		return nil, errors.Wrap(err, "receive authentication response failed")
	}
	if !bytes.Equal(resp, protocol.AuthSucceed) {
		err = errors.WithStack(protocol.ErrAuthFailed)
		client.log(logger.EXPLOIT, err)
		return nil, err
	}
	return conn, nil
}
