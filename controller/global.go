package controller

import (
	"context"
	"io/ioutil"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/patch/toml"
	"project/internal/proxy"
	"project/internal/security"
	"project/internal/timesync"
)

type global struct {
	CertPool   *cert.Pool
	ProxyPool  *proxy.Pool
	DNSClient  *dns.Client
	TimeSyncer *timesync.Syncer

	// objects include various things, see const.
	objects    map[uint32]interface{}
	objectsRWM sync.RWMutex

	// about load session key and certificate pool.
	loadKey     int32
	waitLoadKey chan struct{}
	closeOnce   sync.Once

	context context.Context
	cancel  context.CancelFunc
}

func newGlobal(logger logger.Logger, config *Config) (*global, error) {
	certPool := cert.NewPool()
	proxyPool, err := createProxyPool(certPool, config)
	if err != nil {
		return nil, err
	}
	dnsClient, err := createDNSClient(certPool, proxyPool, config)
	if err != nil {
		return nil, err
	}
	timeSyncer, err := createTimeSyncer(certPool, proxyPool, dnsClient, logger, config)
	if err != nil {
		return nil, err
	}
	global := global{
		CertPool:    certPool,
		ProxyPool:   proxyPool,
		DNSClient:   dnsClient,
		TimeSyncer:  timeSyncer,
		objects:     make(map[uint32]interface{}),
		waitLoadKey: make(chan struct{}),
	}
	global.context, global.cancel = context.WithCancel(context.Background())
	return &global, nil
}

func createProxyPool(certPool *cert.Pool, config *Config) (*proxy.Pool, error) {
	const errorMsg = "failed to load builtin proxy clients"
	data, err := ioutil.ReadFile("builtin/proxy.toml")
	if err != nil {
		return nil, errors.Wrap(err, errorMsg)
	}
	proxyClients := struct {
		Clients []*proxy.Client `toml:"clients"`
	}{}
	err = toml.Unmarshal(data, &proxyClients)
	if err != nil {
		return nil, errors.Wrap(err, errorMsg)
	}
	pool := proxy.NewPool(certPool)
	for _, client := range proxyClients.Clients {
		err = pool.Add(client)
		if err != nil {
			return nil, errors.Wrap(err, errorMsg)
		}
	}
	// try to get proxy client
	_, err = pool.Get(config.Client.ProxyTag)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func createDNSClient(
	certPool *cert.Pool,
	proxyPool *proxy.Pool,
	config *Config,
) (*dns.Client, error) {
	const errorMsg = "failed to load builtin DNS clients"
	data, err := ioutil.ReadFile("builtin/dns.toml")
	if err != nil {
		return nil, errors.Wrap(err, errorMsg)
	}
	servers := make(map[string]*dns.Server)
	err = toml.Unmarshal(data, &servers)
	if err != nil {
		return nil, errors.Wrap(err, errorMsg)
	}
	client := dns.NewClient(certPool, proxyPool)
	for tag, server := range servers {
		err = client.Add("builtin_"+tag, server)
		if err != nil {
			return nil, errors.Wrap(err, errorMsg)
		}
	}
	err = client.SetCacheExpireTime(config.Global.DNSCacheExpire)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return client, nil
}

func createTimeSyncer(
	certPool *cert.Pool,
	proxyPool *proxy.Pool,
	dnsClient *dns.Client,
	logger logger.Logger,
	config *Config,
) (*timesync.Syncer, error) {
	const errorMsg = "failed to load builtin time syncer clients"
	data, err := ioutil.ReadFile("builtin/time.toml")
	if err != nil {
		return nil, errors.Wrap(err, errorMsg)
	}
	clients := make(map[string]*timesync.Client)
	err = toml.Unmarshal(data, &clients)
	if err != nil {
		return nil, errors.Wrap(err, errorMsg)
	}
	syncer := timesync.New(certPool, proxyPool, dnsClient, logger)
	for tag, client := range clients {
		err = syncer.Add("builtin_"+tag, client)
		if err != nil {
			return nil, errors.Wrap(err, errorMsg)
		}
	}
	cfg := config.Global
	err = syncer.SetSyncInterval(cfg.TimeSyncInterval)
	if err != nil {
		return nil, err
	}
	err = syncer.SetSleep(cfg.TimeSyncSleepFixed, cfg.TimeSyncSleepRandom)
	if err != nil {
		return nil, err
	}
	return syncer, nil
}

const (
	// sign message, issue node certificate, type: []byte
	objPrivateKey uint32 = iota

	// encrypt controller broadcast message, type: *aes.CBC
	objBroadcastKey

	// check node certificate, type: []byte
	objPublicKey

	// for key exchange, type: []byte
	objKexPublicKey
)

// GetProxyClient is used to get proxy client from proxy pool.
func (global *global) GetProxyClient(tag string) (*proxy.Client, error) {
	return global.ProxyPool.Get(tag)
}

// ResolveDomain is used to resolve domain name with context and options.
func (global *global) ResolveDomain(
	ctx context.Context,
	domain string,
	opts *dns.Options,
) ([]string, error) {
	return global.DNSClient.ResolveContext(ctx, domain, opts)
}

// TestDNSOption is used to test client DNS option.
func (global *global) TestDNSOption(opts *dns.Options) error {
	_, err := global.DNSClient.TestOption(global.context, "cloudflare.com", opts)
	return err
}

// StartTimeSyncer is used to start time syncer.
func (global *global) StartTimeSyncer() error {
	return global.TimeSyncer.Start()
}

// StartTimeSyncerAddLoop is used to start time syncer add loop.
func (global *global) StartTimeSyncerAddLoop() {
	global.TimeSyncer.StartWalker()
}

// Now is used to get current time.
func (global *global) Now() time.Time {
	return global.TimeSyncer.Now()
}

// IsLoadKey is used to check is load session key and certificate pool.
func (global *global) IsLoadKey() bool {
	return atomic.LoadInt32(&global.loadKey) != 0
}

func (global *global) closeWaitLoadKey() {
	global.closeOnce.Do(func() {
		close(global.waitLoadKey)
	})
}

// LoadKey is used to load session key and certificate pool.
func (global *global) LoadKey(
	sessionKey, sessionKeyPassword []byte,
	certData, rawHash, certPassword []byte,
) error {
	defer func() {
		security.CoverBytes(sessionKey)
		security.CoverBytes(sessionKeyPassword)
		security.CoverBytes(certData)
		security.CoverBytes(certPassword)
	}()
	global.objectsRWM.Lock()
	defer global.objectsRWM.Unlock()
	if global.IsLoadKey() {
		return errors.New("already load session key")
	}
	key, err := loadSessionKey(sessionKey, sessionKeyPassword)
	if err != nil {
		return errors.WithMessage(err, "failed to load session key")
	}
	// ed25519
	pri := key[0]
	defer security.CoverBytes(pri)
	pub, _ := ed25519.ImportPublicKey(pri[32:])
	global.objects[objPublicKey] = pub
	// calculate key exchange public key
	kexPublicKey, err := curve25519.ScalarBaseMult(pri[:curve25519.ScalarSize])
	if err != nil {
		return errors.WithStack(err)
	}
	global.objects[objKexPublicKey] = kexPublicKey
	// hide private key
	memory := security.NewMemory()
	defer memory.Flush()
	global.objects[objPrivateKey] = security.NewBytes(pri)
	security.CoverBytes(pri)
	// aes crypto about broadcast
	cbc, _ := aes.NewCBC(key[1], key[2])
	global.objects[objBroadcastKey] = cbc

	// load certificate pool
	err = loadCertPool(global.CertPool, certData, rawHash, certPassword)
	if err != nil {
		return errors.WithStack(err)
	}

	global.closeWaitLoadKey()
	atomic.StoreInt32(&global.loadKey, 1)
	return nil
}

// WaitLoadSessionKey is used to wait load session key.
func (global *global) WaitLoadSessionKey() bool {
	<-global.waitLoadKey
	return global.IsLoadKey()
}

// Sign is used to verify controller(handshake) and sign message.
func (global *global) Sign(message []byte) []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pri := global.objects[objPrivateKey].(*security.Bytes)
	b := pri.Get()
	defer pri.Put(b)
	return ed25519.Sign(b, message)
}

// Verify is used to verify node certificate.
func (global *global) Verify(message, signature []byte) bool {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pub := global.objects[objPublicKey].(ed25519.PublicKey)
	return ed25519.Verify(pub, message, signature)
}

// KeyExchange is use to calculate session key.
func (global *global) KeyExchange(publicKey []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pri := global.objects[objPrivateKey].(*security.Bytes)
	b := pri.Get()
	defer pri.Put(b)
	return curve25519.ScalarMult(b[:curve25519.ScalarSize], publicKey)
}

// PrivateKey is used to get private key.
// <danger> must remember cover it after use.
func (global *global) PrivateKey() ed25519.PrivateKey {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pri := global.objects[objPrivateKey].(*security.Bytes)
	b := pri.Get()
	defer pri.Put(b)
	bs := make([]byte, ed25519.PrivateKeySize)
	copy(bs, b)
	return bs
}

// PublicKey is used to get public key.
func (global *global) PublicKey() ed25519.PublicKey {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objPublicKey].(ed25519.PublicKey)
}

// KeyExchangePublicKey is used to get key exchange public key.
func (global *global) KeyExchangePublicKey() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objKexPublicKey].([]byte)
}

// Encrypt is used to encrypt controller broadcast message.
func (global *global) Encrypt(data []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	cbc := global.objects[objBroadcastKey].(*aes.CBC)
	return cbc.Encrypt(data)
}

// BroadcastKey is used to get broadcast key.
func (global *global) BroadcastKey() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	key, iv := global.objects[objBroadcastKey].(*aes.CBC).KeyIV()
	return append(key, iv...)
}

// Close is used to close global.
func (global *global) Close() {
	global.closeWaitLoadKey()
	global.cancel()
	global.TimeSyncer.Stop()
}
