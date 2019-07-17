package controller

import (
	"bytes"
	"io"
	"net"

	"github.com/pkg/errors"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/xnet"
)

// certificates = [2 byte uint16] size + signature with node guid
//                [2 byte uint16] size + signature with ctrl guid
func (this *client) handshake(c net.Conn) (*xnet.Conn, error) {
	conn := xnet.New_Conn(c, this.ctx.global.Now().Unix())
	// receive certificates
	certificates, err := conn.Receive()
	if err != nil {
		return nil, errors.Wrap(err, "receive certificate failed")
	}
	// if guid = nil, skip verify
	if this.guid != nil {
		const (
			act = "read certificate(with node guid) "
		)
		reader := bytes.NewReader(certificates)
		// read cert size
		cert_size := make([]byte, 2)
		_, err = io.ReadFull(reader, cert_size)
		if err != nil {
			return nil, errors.Wrap(err, act+"size failed")
		}
		// read cert
		cert_with_node_guid := make([]byte, convert.Bytes_Uint16(cert_size))
		_, err = io.ReadFull(reader, cert_size)
		if err != nil {
			return nil, errors.Wrap(err, act+"failed")
		}
		// verify certificate
		buffer := bytes.Buffer{}
		buffer.WriteString(this.node.Mode)
		buffer.WriteString(this.node.Network)
		buffer.WriteString(this.node.Address)
		buffer.Write(this.guid)
		if bytes.Equal(this.guid, protocol.CTRL_GUID) {
			const (
				act = "read certificate(with controller guid) "
			)
			// read cert size
			_, err = io.ReadFull(reader, cert_size)
			if err != nil {
				return nil, errors.Wrap(err, act+"size failed")
			}
			// read cert
			cert_with_ctrl_guid := make([]byte, convert.Bytes_Uint16(cert_size))
			_, err = io.ReadFull(reader, cert_size)
			if err != nil {
				return nil, errors.Wrap(err, act+"failed")
			}
			if !this.ctx.global.Verify(buffer.Bytes(), cert_with_ctrl_guid) {
				const (
					l = "invalid certificate(with controller guid)"
				)
				this.log(logger.EXPLOIT, l)
				return nil, errors.Wrap(err, l)
			}
		} else {
			if !this.ctx.global.Verify(buffer.Bytes(), cert_with_node_guid) {
				const (
					l = "invalid certificate(with node guid)"
				)
				this.log(logger.EXPLOIT, l)
				return nil, errors.Wrap(err, l)
			}
		}
	}
	// send role
	_, err = conn.Write([]byte{protocol.CTRL})
	if err != nil {
		return nil, errors.Wrap(err, "send role failed")
	}
	err = this.authenticate(conn)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (this *client) authenticate(conn *xnet.Conn) error {
	// receive challenge
	challenge, err := conn.Receive()
	if err != nil {
		return errors.Wrap(err, "receive challenge data failed")
	}
	// <danger>
	// receive random challenge data(length 2048-4096)
	// len(challenge) must > len(GUID + Mode + Network + Address)
	// because maybe fake node will send some special data
	// and if controller sign it will destory net
	if len(challenge) < 2048 || len(challenge) > 4096 {
		const (
			l = "invalid challenge size"
		)
		this.log(logger.EXPLOIT, l)
		return errors.Wrap(err, l)
	}
	// send signature
	err = conn.Send(this.ctx.global.Sign(challenge))
	if err != nil {
		return errors.Wrap(err, "send challenge signature failed")
	}
	resp, err := conn.Receive()
	if err != nil {
		return errors.Wrap(err, "receive authentication response failed")
	}
	if !bytes.Equal(resp, protocol.AUTH_SUCCESS) {
		return errors.WithStack(protocol.ERR_AUTH_FAILED)
	}
	return nil
}
