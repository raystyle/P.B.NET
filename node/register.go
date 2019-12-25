package node

import (
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/logger"
	"project/internal/messages"
	"project/internal/modules/info"
	"project/internal/random"
	"project/internal/xnet"
)

func (s *server) registerNode(conn *xnet.Conn, guid []byte) {
	// receive node register request
	req, err := conn.Receive()
	if err != nil {
		s.logConn(conn, logger.Error, "failed to receive node register request:", err)
		return
	}
	// try to unmarshal
	nrr := new(messages.NodeRegisterRequest)
	err = msgpack.Unmarshal(req, nrr)
	if err != nil {
		s.logConn(conn, logger.Exploit, "invalid node register request data:", err)
		return
	}
	err = nrr.Validate()
	if err != nil {
		s.logConn(conn, logger.Exploit, "invalid node register request:", err)
		return
	}
	// create node register
	response := s.ctx.storage.CreateNodeRegister(guid)
	if response == nil {
		_ = conn.Send([]byte{messages.RegisterResultRefused})
		s.logfConn(conn, logger.Exploit, "failed to create node register\nguid: %X", guid)
		return
	}
	// send node register request to controller
	// <security> must don't handle error
	_ = s.ctx.sender.Send(messages.CMDBNodeRegisterRequest, nrr)
	// wait register result
	timeout := time.Duration(15+random.New().Int(30)) * time.Second
	timer := time.AfterFunc(timeout, func() {
		s.ctx.storage.SetNodeRegister(guid, &messages.NodeRegisterResponse{
			Result: messages.RegisterResultTimeout,
		})
	})
	defer timer.Stop()
	resp := <-response
	switch resp.Result {
	case messages.RegisterResultAccept:
		_ = conn.Send([]byte{messages.RegisterResultAccept})
		if !s.verifyNode(conn, guid) {
			_ = conn.Send([]byte{messages.RegisterResultRefused})
			return
		}
		// send certificate and listener configs
	case messages.RegisterResultRefused: // TODO add IP black list only register(other role still pass)
		_ = conn.Send([]byte{messages.RegisterResultRefused})
	case messages.RegisterResultTimeout:
		_ = conn.Send([]byte{messages.RegisterResultTimeout})
	default:
		s.logfConn(conn, logger.Exploit, "unknown register result: %d", resp.Result)
	}
}

func (client *client) Register() error {
	conn := client.conn
	defer client.Close()
	// send operation
	_, err := conn.Write([]byte{1})
	if err != nil {
		return errors.Wrap(err, "failed to send operation")
	}
	// send register request
	err = conn.SendRaw(client.ctx.packRegisterRequest())
	if err != nil {
		return errors.Wrap(err, "failed to send register request")
	}
	// wait register result
	_ = conn.SetDeadline(client.ctx.global.Now().Add(time.Minute))
	result := make([]byte, 1)
	_, err = io.ReadFull(conn, result)
	if err != nil {
		return errors.Wrap(err, "failed to receive register result")
	}
	switch result[0] {
	case messages.RegisterResultAccept:
		// receive certificate and listener configs
		return nil
	case messages.RegisterResultRefused:
		return errors.WithStack(messages.ErrRegisterRefused)
	case messages.RegisterResultTimeout:
		return errors.WithStack(messages.ErrRegisterTimeout)
	default:
		err = errors.WithMessagef(messages.ErrRegisterUnknownResult, "%d", result[0])
		client.log(logger.Exploit, err)
		return err
	}
}

func (node *Node) packRegisterRequest() []byte {
	req := messages.NodeRegisterRequest{
		GUID:         node.global.GUID(),
		PublicKey:    node.global.PublicKey(),
		KexPublicKey: node.global.KeyExchangePub(),
		SystemInfo:   info.GetSystemInfo(),
		RequestTime:  node.global.Now(),
	}
	b, err := msgpack.Marshal(&req)
	if err != nil {
		panic(err)
	}
	return b
}
