package controller

import (
	"bytes"
	"context"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/security"
)

// Config include configuration about Controller
type Config struct {
	Test Test `toml:"-"`

	Database struct {
		Dialect         string    `toml:"dialect"` // "mysql"
		DSN             string    `toml:"dsn"`
		MaxOpenConns    int       `toml:"max_open_conns"`
		MaxIdleConns    int       `toml:"max_idle_conns"`
		LogFile         string    `toml:"log_file"`
		GORMLogFile     string    `toml:"gorm_log_file"`
		GORMDetailedLog bool      `toml:"gorm_detailed_log"`
		LogWriter       io.Writer `toml:"-"`
	} `toml:"database"`

	Logger struct {
		Level  string    `toml:"level"`
		File   string    `toml:"file"`
		Writer io.Writer `toml:"-"`
	} `toml:"logger"`

	Global struct {
		DNSCacheExpire      time.Duration `toml:"dns_cache_expire"`
		TimeSyncSleepFixed  uint          `toml:"timesync_sleep_fixed"`
		TimeSyncSleepRandom uint          `toml:"timesync_sleep_random"`
		TimeSyncInterval    time.Duration `toml:"timesync_interval"`
	} `toml:"global"`

	Client struct {
		ProxyTag string        `toml:"proxy_tag"`
		Timeout  time.Duration `toml:"timeout"`
		DNSOpts  dns.Options   `toml:"dns"`
	} `toml:"client"`

	Sender struct {
		MaxConns      int           `toml:"max_conns"`
		Worker        int           `toml:"worker"`
		Timeout       time.Duration `toml:"timeout"`
		QueueSize     int           `toml:"queue_size"`
		MaxBufferSize int           `toml:"max_buffer_size"`
	} `toml:"sender"`

	Syncer struct {
		ExpireTime time.Duration `toml:"expire_time"`
	} `toml:"syncer"`

	Worker struct {
		Number        int `toml:"number"`
		QueueSize     int `toml:"queue_size"`
		MaxBufferSize int `toml:"max_buffer_size"`
	} `toml:"worker"`

	Web struct {
		Dir      string `toml:"dir"`
		CertFile string `toml:"cert_file"`
		KeyFile  string `toml:"key_file"`
		Address  string `toml:"address"`
		Username string `toml:"username"` // super user
		Password string `toml:"password"`
	} `toml:"web"`
}

// TrustNode is used to trust Node, receive system info for confirm it.
// usually for the initial node or the test
// TODO add log
func (ctrl *CTRL) TrustNode(
	ctx context.Context,
	listener *bootstrap.Listener,
) (*messages.NodeRegisterRequest, error) {
	client, err := ctrl.NewClient(ctx, listener, nil, nil)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	// send trust node command
	reply, err := client.send(protocol.CtrlTrustNode, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to send trust node command")
	}
	if len(reply) < curve25519.ScalarSize+aes.BlockSize {
		// TODO add exploit
		return nil, errors.New("node send register request with invalid size")
	}
	// calculate role session key
	key, err := ctrl.global.KeyExchange(reply[:curve25519.ScalarSize])
	if err != nil {
		const format = "node send invalid register request\nerror: %s"
		return nil, errors.Errorf(format, err)
	}
	// decrypt role register request
	request, err := aes.CBCDecrypt(reply[curve25519.ScalarSize:], key, key[:aes.IVSize])
	if err != nil {
		const format = "node send invalid register request\nerror: %s"
		return nil, errors.Errorf(format, err)
	}
	nrr := messages.NodeRegisterRequest{}
	err = msgpack.Unmarshal(request, &nrr)
	if err != nil {
		// ctrl.logger.Print(logger.Exploit, "trust node", err)
		return nil, errors.Wrap(err, "invalid node register request")
	}
	err = nrr.Validate()
	if err != nil {
		// ctrl.logger.Print(logger.Exploit, "trust node", err)
		return nil, errors.Wrap(err, "invalid node register request")
	}
	return &nrr, nil
}

// ConfirmTrustNode is used to confirm trust node, register node
// TODO add log
func (ctrl *CTRL) ConfirmTrustNode(
	ctx context.Context,
	listener *bootstrap.Listener,
	nrr *messages.NodeRegisterRequest,
) error {
	client, err := ctrl.NewClient(ctx, listener, nil, nil)
	if err != nil {
		return err
	}
	defer client.Close()
	// register node
	cert, err := ctrl.registerNode(nrr, true)
	if err != nil {
		return err
	}
	// send certificate
	reply, err := client.send(protocol.CtrlSetNodeCert, cert.Encode())
	if err != nil {
		return errors.WithMessage(err, "failed to set node certificate")
	}
	if bytes.Compare(reply, []byte{messages.RegisterResultAccept}) != 0 {
		return errors.Errorf("failed to trust node: %s", reply)
	}
	return nil
}

func (ctrl *CTRL) registerNode(
	nrr *messages.NodeRegisterRequest,
	bootstrap bool,
) (*protocol.Certificate, error) {
	failed := func(err error) error {
		return errors.Wrap(err, "failed to register node")
	}
	// issue certificate
	cert := protocol.Certificate{
		GUID:      nrr.GUID,
		PublicKey: nrr.PublicKey,
	}
	privateKey := ctrl.global.PrivateKey()
	defer security.CoverBytes(privateKey)
	err := protocol.IssueCertificate(&cert, privateKey)
	if err != nil {
		return nil, failed(err)
	}
	security.CoverBytes(privateKey)
	// calculate session key
	sessionKey, err := ctrl.global.KeyExchange(nrr.KexPublicKey)
	if err != nil {
		err = errors.WithMessage(err, "failed to calculate session key")
		ctrl.logger.Print(logger.Exploit, "register node", err)
		return nil, failed(err)
	}
	err = ctrl.database.InsertNode(&mNode{
		GUID:        nrr.GUID[:],
		PublicKey:   nrr.PublicKey,
		SessionKey:  sessionKey,
		IsBootstrap: bootstrap,
	})
	if err != nil {
		return nil, failed(err)
	}
	return &cert, nil
}

// GenerateRoleConfigAboutTheFirstBootstrap is used to generate the first bootstrap
func GenerateRoleConfigAboutTheFirstBootstrap(b *messages.Bootstrap) ([]byte, []byte, error) {
	return generateRoleConfigAboutBootstraps(b)
}

// GenerateRoleConfigAboutRestBootstraps is used to generate role rest bootstraps
func GenerateRoleConfigAboutRestBootstraps(b ...*messages.Bootstrap) ([]byte, []byte, error) {
	if len(b) == 0 {
		return nil, nil, nil
	}
	return generateRoleConfigAboutBootstraps(b)
}

func generateRoleConfigAboutBootstraps(b interface{}) ([]byte, []byte, error) {
	data, _ := msgpack.Marshal(b)
	rand := random.New()
	aesKey := rand.Bytes(aes.Key256Bit)
	aesIV := rand.Bytes(aes.IVSize)
	enc, err := aes.CBCEncrypt(data, aesKey, aesIV)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	return enc, append(aesKey, aesIV...), nil
}

// GenerateNodeConfigAboutListeners is used to generate node listener and encrypt it
func GenerateNodeConfigAboutListeners(l ...*messages.Listener) ([]byte, []byte, error) {
	if len(l) == 0 {
		return nil, nil, errors.New("no listeners")
	}
	data, _ := msgpack.Marshal(l)
	rand := random.New()
	aesKey := rand.Bytes(aes.Key256Bit)
	aesIV := rand.Bytes(aes.IVSize)
	enc, err := aes.CBCEncrypt(data, aesKey, aesIV)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	return enc, append(aesKey, aesIV...), nil
}
