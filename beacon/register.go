package beacon

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/module/info"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/security"
)

type register struct {
	ctx *Beacon

	// about random.Sleep() in Register()
	sleepFixed  uint
	sleepRandom uint

	// key = messages.Bootstrap.Tag
	bootstraps    map[string]bootstrap.Bootstrap
	bootstrapsRWM sync.RWMutex

	// Register() only use the first bootstrap
	first bootstrap.Bootstrap

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func newRegister(ctx *Beacon, config *Config) (*register, error) {
	cfg := config.Register

	if cfg.SleepFixed < 10 {
		return nil, errors.New("register SleepFixed must >= 10")
	}
	if cfg.SleepRandom < 20 {
		return nil, errors.New("register SleepRandom must >= 20")
	}
	if len(cfg.FirstBoot) == 0 {
		return nil, errors.New("no bootstraps")
	}

	memory := security.NewMemory()
	defer memory.Flush()

	register := register{
		ctx:         ctx,
		sleepFixed:  cfg.SleepFixed,
		sleepRandom: cfg.SleepRandom,
		bootstraps:  make(map[string]bootstrap.Bootstrap),
	}
	register.context, register.cancel = context.WithCancel(context.Background())

	// decrypt the first bootstrap
	err := register.loadBootstraps(cfg.FirstBoot, cfg.FirstKey, true)
	if err != nil {
		return nil, err
	}
	// set the first bootstrap(s)
	// if the cfg.FirstBoot contains more than one bootstrap,
	// register will select a bootstrap randomly
	for _, boot := range register.bootstraps {
		register.first = boot
	}

	// decrypt the rest bootstraps
	if len(cfg.RestBoots) != 0 {
		err := register.loadBootstraps(cfg.RestBoots, cfg.RestKey, false)
		if err != nil {
			return nil, err
		}
	}
	return &register, nil
}

func (register *register) loadBootstraps(boot, key []byte, single bool) error {
	defer func() {
		security.CoverBytes(boot)
		security.CoverBytes(key)
	}()

	memory := security.NewMemory()
	defer memory.Flush()

	if len(key) != aes.Key256Bit+aes.IVSize {
		return errors.New("invalid aes key size")
	}
	aesKey := key[:aes.Key256Bit]
	aesIV := key[aes.Key256Bit:]
	defer func() {
		security.CoverBytes(aesKey)
		security.CoverBytes(aesIV)
	}()

	// decrypt bootstraps
	memory.Padding()
	data, err := aes.CBCDecrypt(boot, aesKey, aesIV)
	if err != nil {
		return errors.WithStack(err)
	}
	// cover bytes at once
	security.CoverBytes(aesKey)
	security.CoverBytes(aesIV)

	// load bootstraps
	memory.Padding()
	var bootstraps []*messages.Bootstrap
	if single {
		boot := new(messages.Bootstrap)
		err = msgpack.Unmarshal(data, boot)
		if err != nil {
			return errors.WithStack(err)
		}
		bootstraps = []*messages.Bootstrap{boot}
	} else {
		err = msgpack.Unmarshal(data, &bootstraps)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	if len(bootstraps) == 0 {
		return errors.New("no bootstraps")
	}
	for i := 0; i < len(bootstraps); i++ {
		memory.Padding()
		err = register.AddBootstrap(bootstraps[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (register *register) logf(l logger.Level, format string, log ...interface{}) {
	register.ctx.logger.Printf(l, "register", format, log...)
}

func (register *register) log(l logger.Level, log ...interface{}) {
	register.ctx.logger.Println(l, "register", log...)
}

func (register *register) AddBootstrap(b *messages.Bootstrap) error {
	register.bootstrapsRWM.Lock()
	defer register.bootstrapsRWM.Unlock()
	if _, ok := register.bootstraps[b.Tag]; ok {
		return errors.Errorf("bootstrap %s already exists", b.Tag)
	}
	boot, err := bootstrap.Load(register.context, b.Mode, b.Config,
		register.ctx.global.ProxyPool, register.ctx.global.DNSClient,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to load bootstrap %s", b.Tag)
	}
	register.bootstraps[b.Tag] = boot
	return nil
}

func (register *register) DeleteBootstrap(tag string) error {
	register.bootstrapsRWM.Lock()
	defer register.bootstrapsRWM.Unlock()
	if _, ok := register.bootstraps[tag]; ok {
		delete(register.bootstraps, tag)
		return nil
	}
	return errors.Errorf("bootstrap %s doesn't exists", tag)
}

func (register *register) Bootstraps() map[string]bootstrap.Bootstrap {
	register.bootstrapsRWM.RLock()
	defer register.bootstrapsRWM.RUnlock()
	bs := make(map[string]bootstrap.Bootstrap, len(register.bootstraps))
	for tag, boot := range register.bootstraps {
		bs[tag] = boot
	}
	return bs
}

// Register is used to register to Controller
// <security> only use the first bootstrap
func (register *register) Register() error {
	register.wg.Add(1)
	defer register.wg.Done()

	register.log(logger.Debug, "start register")
	// resolve bootstrap node listeners with the first bootstrap, try 3 times
	var (
		listeners []*bootstrap.Listener
		err       error
	)
	for i := 0; i < 3; i++ {
		listeners, err = register.first.Resolve()
		if err == nil {
			break
		}
		register.log(logger.Error, err)
		random.Sleep(register.sleepFixed, register.sleepRandom)
	}
	if err != nil {
		return errors.WithMessage(err, "failed to resolve bootstrap node listeners")
	}
	// try to register with all resolved bootstrap node listeners,
	// each listener try 3 times
	for _, listener := range listeners {
		for i := 0; i < 3; i++ {
			err = register.register(listener)
			if err == nil {
				register.log(logger.Debug, "register successfully")
				return nil
			}
			register.log(logger.Error, err)
			if errors.Cause(err) != messages.ErrRegisterTimeout {
				return err
			}
			if i != 2 {
				random.Sleep(register.sleepFixed, register.sleepRandom)
			}
		}
	}
	return err
}

// register is used to register to Controller with Node
func (register *register) register(listener *bootstrap.Listener) error {
	register.wg.Add(1)
	defer register.wg.Done()

	client, err := register.ctx.NewClient(
		register.context,
		listener,
		protocol.CtrlGUID,
		nil,
	)
	if err != nil {
		return err
	}
	defer client.Close()

	conn := client.Conn
	// interrupt
	wg := sync.WaitGroup{}
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer func() {
			recover()
			wg.Done()
		}()
		select {
		case <-done:
		case <-register.context.Done():
			_ = conn.Close()
		}
	}()
	defer func() {
		close(done)
		wg.Wait()
	}()

	// send register request
	_, err = conn.Write([]byte{protocol.BeaconOperationRegister})
	if err != nil {
		return errors.Wrap(err, "failed to send register operation")
	}
	// get external IP address
	address, err := conn.Receive()
	if err != nil {
		return errors.Wrap(err, "failed to receive external ip address")
	}
	// send register request
	err = conn.Send(register.packRequest(string(address)))
	if err != nil {
		return errors.Wrap(err, "failed to send register request")
	}
	// wait register result
	timeout := time.Duration(60+random.Int(30)) * time.Second
	_ = conn.SetDeadline(register.ctx.global.Now().Add(timeout))
	result := make([]byte, 1)
	_, err = io.ReadFull(conn, result)
	if err != nil {
		return errors.Wrap(err, "failed to receive register result")
	}
	switch result[0] {
	case messages.RegisterResultAccept:

		return nil
	case messages.RegisterResultRefused:
		return errors.WithStack(messages.ErrRegisterRefused)
	case messages.RegisterResultTimeout:
		return errors.WithStack(messages.ErrRegisterTimeout)
	default:
		err = errors.WithMessagef(messages.ErrRegisterUnknownResult, "%d", result[0])
		register.log(logger.Exploit, err)
		return err
	}
}

// packRequest is used to pack beacon register request and encrypt it.
// it is used to register.Register().
//
// self key exchange public key (curve25519),
// use session key encrypt register request data.
// +----------------+----------------+
// | kex public key | encrypted data |
// +----------------+----------------+
// |    32 Bytes    |       var      |
// +----------------+----------------+
func (register *register) packRequest(address string) []byte {
	nrr := messages.NodeRegisterRequest{
		GUID:         *register.ctx.global.GUID(),
		PublicKey:    register.ctx.global.PublicKey(),
		KexPublicKey: register.ctx.global.KeyExchangePublicKey(),
		ConnAddress:  address,
		SystemInfo:   info.GetSystemInfo(),
		RequestTime:  register.ctx.global.Now(),
	}
	data, err := msgpack.Marshal(&nrr)
	if err != nil {
		panic("register internal error: " + err.Error())
	}
	cipherData, err := register.ctx.global.Encrypt(data)
	if err != nil {
		panic("register internal error: " + err.Error())
	}
	request := make([]byte, curve25519.ScalarSize)
	copy(request, register.ctx.global.KeyExchangePublicKey())
	return append(request, cipherData...)
}

func (register *register) Close() {
	register.cancel()
	register.wg.Wait()
	register.ctx = nil
}