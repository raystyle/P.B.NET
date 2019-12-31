package node

import (
	"context"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/modules/info"
	"project/internal/protocol"
)

type register struct {
	ctx *Node

	// skip register for genesis node
	// or Controller trust node manually
	skip bool
}

func newRegister(ctx *Node, config *Config) *register {
	cfg := config.Register
	register := register{
		ctx:  ctx,
		skip: cfg.Skip,
	}
	return &register
}

func (reg *register) logf(l logger.Level, format string, log ...interface{}) {
	reg.ctx.logger.Printf(l, "register", format, log...)
}

func (reg *register) log(l logger.Level, log ...interface{}) {
	reg.ctx.logger.Print(l, "register", log...)
}

// PackRequest is used to pack node register request
// is used to register.Register() and ctrlConn.handleTrustNode()
func (reg *register) PackRequest() []byte {
	req := messages.NodeRegisterRequest{
		GUID:         reg.ctx.global.GUID(),
		PublicKey:    reg.ctx.global.PublicKey(),
		KexPublicKey: reg.ctx.global.KeyExchangePub(),
		SystemInfo:   info.GetSystemInfo(),
		RequestTime:  reg.ctx.global.Now(),
	}
	b, err := msgpack.Marshal(&req)
	if err != nil {
		panic(err)
	}
	return b
}

// Register is used to register to Controller with Node
func (reg *register) Register(ctx context.Context, node *bootstrap.Node) error {
	client, err := newClient(ctx, reg.ctx, node, protocol.CtrlGUID, nil)
	if err != nil {
		return err
	}
	defer client.Close()
	conn := client.Conn
	// send register operation
	_, err = conn.Write([]byte{1}) // 1 = register
	if err != nil {
		return errors.Wrap(err, "failed to send register operation")
	}
	// send register request
	err = conn.SendMessage(reg.PackRequest())
	if err != nil {
		return errors.Wrap(err, "failed to send register request")
	}
	// wait register result
	_ = conn.SetDeadline(reg.ctx.global.Now().Add(time.Minute))
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
		reg.log(logger.Exploit, "register", err)
		return err
	}
}

func (reg *register) Skip() bool {
	return reg.skip
}

// AutoRegister is used to register to Controller automatically
func (reg *register) AutoRegister() error {
	return nil
}

func (reg *register) Close() {
	reg.ctx = nil
}
