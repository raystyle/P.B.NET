package node

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/modules/info"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/security"
)

type register struct {
	ctx *Node

	// about random.Sleep() in Register()
	sleepFixed  int
	sleepRandom int
	// skip automatic register for genesis node,
	// or Controller trust node manually
	skip bool

	// Register() only use the first bootstrap
	firstTag string

	// key = messages.Bootstrap.Tag
	bootstraps    map[string]bootstrap.Bootstrap
	bootstrapsRWM sync.RWMutex

	context context.Context
	cancel  context.CancelFunc
}

func newRegister(ctx *Node, config *Config) (*register, error) {
	cfg := config.Register

	if cfg.SleepFixed < 10 {
		return nil, errors.New("register SleepFixed must >= 10")
	}
	if cfg.SleepRandom < 20 {
		return nil, errors.New("register SleepRandom must >= 20")
	}
	if !cfg.Skip && len(cfg.Bootstraps) == 0 {
		return nil, errors.New("not skip automatic register but no bootstraps")
	}

	memory := security.NewMemory()
	defer memory.Flush()

	reg := register{bootstraps: make(map[string]bootstrap.Bootstrap)}
	// decrypt bootstraps
	if len(cfg.Bootstraps) != 0 {
		if len(cfg.AESCrypto) != aes.Key256Bit+aes.IVSize {
			return nil, errors.New("invalid aes key size")
		}
		aesKey := cfg.AESCrypto[:aes.Key256Bit]
		aesIV := cfg.AESCrypto[aes.Key256Bit:]
		defer func() {
			security.CoverBytes(aesKey)
			security.CoverBytes(aesIV)
		}()
		memory.Padding()
		data, err := aes.CBCDecrypt(cfg.Bootstraps, aesKey, aesIV)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		security.CoverBytes(aesKey)
		security.CoverBytes(aesIV)
		// load bootstraps
		memory.Padding()
		var bootstraps []*messages.Bootstrap
		err = msgpack.Unmarshal(data, &bootstraps)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if len(bootstraps) == 0 {
			return nil, errors.New("no bootstraps")
		}
		for i := 0; i < len(bootstraps); i++ {
			memory.Padding()
			err = reg.AddBootstrap(bootstraps[i])
			if err != nil {
				return nil, err
			}
		}
		reg.firstTag = bootstraps[0].Tag
	}

	reg.ctx = ctx
	reg.sleepFixed = cfg.SleepFixed
	reg.sleepRandom = cfg.SleepRandom
	reg.skip = cfg.Skip
	reg.context, reg.cancel = context.WithCancel(context.Background())
	return &reg, nil
}

func (reg *register) logf(l logger.Level, format string, log ...interface{}) {
	reg.ctx.logger.Printf(l, "register", format, log...)
}

func (reg *register) log(l logger.Level, log ...interface{}) {
	reg.ctx.logger.Print(l, "register", log...)
}

func (reg *register) AddBootstrap(b *messages.Bootstrap) error {
	reg.bootstrapsRWM.Lock()
	defer reg.bootstrapsRWM.Unlock()
	if _, ok := reg.bootstraps[b.Tag]; ok {
		return errors.Errorf("bootstrap %s already exists", b.Tag)
	}
	boot, err := bootstrap.Load(
		reg.context, b.Mode, b.Config,
		reg.ctx.global.ProxyPool, reg.ctx.global.DNSClient,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to load bootstrap %s", b.Tag)
	}
	reg.bootstraps[b.Tag] = boot
	return nil
}

func (reg *register) DeleteBootstrap(tag string) error {
	reg.bootstrapsRWM.Lock()
	defer reg.bootstrapsRWM.Unlock()
	if _, ok := reg.bootstraps[tag]; ok {
		delete(reg.bootstraps, tag)
		return nil
	}
	return errors.Errorf("bootstrap %s doesn't exists", tag)
}

func (reg *register) Bootstraps() map[string]bootstrap.Bootstrap {
	reg.bootstrapsRWM.RLock()
	defer reg.bootstrapsRWM.RUnlock()
	bs := make(map[string]bootstrap.Bootstrap, len(reg.bootstraps))
	for tag, boot := range reg.bootstraps {
		bs[tag] = boot
	}
	return bs
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

// register is used to register to Controller with Node
func (reg *register) register(node *bootstrap.Node) error {
	client, err := newClient(reg.context, reg.ctx, node, protocol.CtrlGUID, nil)
	if err != nil {
		return err
	}
	defer client.Close()
	conn := client.Conn
	// send register operation
	_, err = conn.Write([]byte{nodeOperationRegister})
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
		// receive certificate and a part of node listeners that controller select
		return nil
	case messages.RegisterResultRefused:
		return errors.WithStack(messages.ErrRegisterRefused)
	case messages.RegisterResultTimeout:
		return errors.WithStack(messages.ErrRegisterTimeout)
	default:
		err = errors.WithMessagef(messages.ErrRegisterUnknownResult, "%d", result[0])
		reg.log(logger.Exploit, err)
		return err
	}
}

// Register is used to register to Controller
func (reg *register) Register() error {
	if reg.skip {
		return nil
	}
	reg.log(logger.Debug, "start register")
	// <security> only use the first bootstrap
	boot := reg.bootstraps[reg.firstTag]
	// try 3 times
	var (
		nodes []*bootstrap.Node
		err   error
	)
	for i := 0; i < 3; i++ {
		nodes, err = boot.Resolve() // resolve bootstrap nodes
		if err == nil {
			break
		}
		reg.log(logger.Error, err)
		random.Sleep(reg.sleepFixed, reg.sleepRandom)
	}
	if err != nil {
		return errors.WithMessage(err, "failed to resolve bootstrap nodes")
	}
	// try all resolved bootstrap nodes, 3 times
	for i := 0; i < 3; i++ {
		for _, node := range nodes {
			err = reg.register(node)
			if err == nil {
				reg.log(logger.Debug, "register successfully")
				return nil
			}
			reg.log(logger.Error, err)
			if errors.Cause(err) != messages.ErrRegisterTimeout {
				return err
			}
			random.Sleep(reg.sleepFixed, reg.sleepRandom)
		}
	}
	return err
}

func (reg *register) Close() {
	reg.cancel()
	reg.ctx = nil
}
