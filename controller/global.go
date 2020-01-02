package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/crypto/cert/certutil"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/security"
	"project/internal/timesync"
)

type global struct {
	// when configure Node or Beacon, these proxies will not appear
	ProxyPool *proxy.Pool

	// when configure Node or Beacon, DNS Server in *dns.Client
	// will appear for select if database without DNS Server
	DNSClient *dns.Client

	// when configure Node or Beacon, time syncer client in
	// *timesync.Syncer will appear for select if database
	// without time syncer client
	TimeSyncer *timesync.Syncer

	// objects include various things
	objects    map[uint32]interface{}
	objectsRWM sync.RWMutex

	// about load session key
	loadSessionKey     int32
	waitLoadSessionKey chan struct{}
	closeOnce          sync.Once

	ctx    context.Context
	cancel context.CancelFunc
}

func newGlobal(logger logger.Logger, config *Config) (*global, error) {
	cfg := config.Global

	// load builtin proxy clients
	const errProxy = "failed to load builtin proxy clients"
	b, err := ioutil.ReadFile("builtin/proxy.toml")
	if err != nil {
		return nil, errors.Wrap(err, errProxy)
	}
	proxyClients := struct {
		Clients []*proxy.Client `toml:"clients"`
	}{}
	err = toml.Unmarshal(b, &proxyClients)
	if err != nil {
		return nil, errors.Wrap(err, errProxy)
	}
	proxyPool := proxy.NewPool()
	for _, client := range proxyClients.Clients {
		err = proxyPool.Add(client)
		if err != nil {
			return nil, errors.Wrap(err, errProxy)
		}
	}
	// try to get proxy client
	_, err = proxyPool.Get(config.Client.ProxyTag)
	if err != nil {
		return nil, err
	}

	// load builtin dns clients
	const errDNS = "failed to load builtin DNS clients"
	b, err = ioutil.ReadFile("builtin/dns.toml")
	if err != nil {
		return nil, errors.Wrap(err, errDNS)
	}
	dnsServers := make(map[string]*dns.Server)
	err = toml.Unmarshal(b, &dnsServers)
	if err != nil {
		return nil, errors.Wrap(err, errDNS)
	}
	dnsClient := dns.NewClient(proxyPool)
	for tag, server := range dnsServers {
		err = dnsClient.Add("builtin_"+tag, server)
		if err != nil {
			return nil, errors.Wrap(err, errDNS)
		}
	}
	err = dnsClient.SetCacheExpireTime(cfg.DNSCacheExpire)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	// load builtin time syncer client
	const errTSC = "failed to load builtin time syncer clients"
	b, err = ioutil.ReadFile("builtin/time.toml")
	if err != nil {
		return nil, errors.Wrap(err, errTSC)
	}
	timeSyncerClients := make(map[string]*timesync.Client)
	err = toml.Unmarshal(b, &timeSyncerClients)
	if err != nil {
		return nil, errors.Wrap(err, errTSC)
	}
	timeSyncer := timesync.New(proxyPool, dnsClient, logger)
	for tag, client := range timeSyncerClients {
		err = timeSyncer.Add("builtin_"+tag, client)
		if err != nil {
			return nil, errors.Wrap(err, errTSC)
		}
	}
	err = timeSyncer.SetSyncInterval(cfg.TimeSyncInterval)
	if err != nil {
		return nil, errors.Wrap(err, errTSC)
	}
	timeSyncer.SetSleep(cfg.TimeSyncSleepFixed, cfg.TimeSyncSleepRandom)
	g := global{
		ProxyPool:          proxyPool,
		DNSClient:          dnsClient,
		TimeSyncer:         timeSyncer,
		objects:            make(map[uint32]interface{}),
		waitLoadSessionKey: make(chan struct{}),
	}
	g.ctx, g.cancel = context.WithCancel(context.Background())
	return &g, nil
}

// <warning> must < 1048576
const (
	// sign message, issue node certificate, type []byte
	objPrivateKey uint32 = iota

	// check node certificate, type []byte
	objPublicKey

	// for key exchange, type []byte
	objKeyExPub

	// encrypt controller broadcast message, type: *aes.CBC
	objBroadcastKey

	// self CA certificates and private keys, type []*cert.KeyPair
	objSelfCA

	// system CA certificates, type []*x509.Certificate
	objSystemCA
)

// GetSelfCA is used to get self CA certificate to generate CA-sign certificate
func (global *global) GetSelfCA() []*cert.KeyPair {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objSelfCA].([]*cert.KeyPair)
}

// GetSystemCA is used to get system CA certificate
func (global *global) GetSystemCA() []*x509.Certificate {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objSystemCA].([]*x509.Certificate)
}

// GetProxyClient is used to get proxy client from proxy pool
func (global *global) GetProxyClient(tag string) (*proxy.Client, error) {
	return global.ProxyPool.Get(tag)
}

// ResolveWithContext is used to resolve domain name with context and options
func (global *global) ResolveWithContext(
	ctx context.Context,
	domain string,
	opts *dns.Options,
) ([]string, error) {
	return global.DNSClient.ResolveWithContext(ctx, domain, opts)
}

// DNSServers is used to get all DNS servers in DNS client
func (global *global) DNSServers() map[string]*dns.Server {
	return global.DNSClient.Servers()
}

// TestDNSOption is used to test client DNS option
func (global *global) TestDNSOption(opts *dns.Options) error {
	_, err := global.DNSClient.TestOption(global.ctx, "cloudflare.com", opts)
	return err
}

// TimeSyncerClients is used to get all time syncer clients in time syncer
func (global *global) TimeSyncerClients() map[string]*timesync.Client {
	return global.TimeSyncer.Clients()
}

// StartTimeSyncer is used to start time syncer
func (global *global) StartTimeSyncer() error {
	return global.TimeSyncer.Start()
}

// StartTimeSyncerAddLoop is used to start time syncer add loop
func (global *global) StartTimeSyncerAddLoop() {
	global.TimeSyncer.StartWalker()
}

// Now is used to get current time
func (global *global) Now() time.Time {
	return global.TimeSyncer.Now().Local()
}

// GenerateSessionKey is used to generate session key and save to file
func GenerateSessionKey(path string, password []byte) error {
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return errors.Errorf("file: %s already exist", path)
	}
	key, err := generateSessionKey(password)
	if err != nil {
		return nil
	}
	return ioutil.WriteFile(path, key, 644)
}

func createAESKeyIVFromPassword(password []byte) ([]byte, []byte) {
	hash := sha256.New()
	hash.Write(password)
	aesKey := hash.Sum(nil)
	hash.Reset()
	hash.Write(aesKey)
	hash.Write([]byte{20, 18, 11, 27})
	aesIV := hash.Sum(nil)[:aes.IVSize]
	return aesKey, aesIV
}

func generateSessionKey(password []byte) ([]byte, error) {
	// generate ed25519 private key(for sign message)
	privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// generate aes key & iv(for broadcast message)
	broadcastKey := append(random.Bytes(aes.Key256Bit), random.Bytes(aes.IVSize)...)
	// save keys
	buf := new(bytes.Buffer)
	buf.Write(privateKey)
	buf.Write(broadcastKey)
	// calculate hash
	hash := sha256.New()
	hash.Write(buf.Bytes())
	keysHash := hash.Sum(nil)
	// encrypt keys
	aesKey, aesIV := createAESKeyIVFromPassword(password)
	keysEnc, err := aes.CBCEncrypt(buf.Bytes(), aesKey, aesIV)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return append(keysHash, keysEnc...), nil
}

// return ed25519 private key & aes key & aes iv
func loadSessionKey(data, password []byte) ([][]byte, error) {
	const sessionKeySize = sha256.Size +
		ed25519.PrivateKeySize + aes.Key256Bit + aes.IVSize +
		aes.BlockSize
	if len(data) != sessionKeySize {
		return nil, errors.New("invalid session key size")
	}
	memory := security.NewMemory()
	defer memory.Flush()
	// decrypt session key
	aesKey, aesIV := createAESKeyIVFromPassword(password)
	keysDec, err := aes.CBCDecrypt(data[sha256.Size:], aesKey, aesIV)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// compare hash
	hash := sha256.New()
	hash.Write(keysDec)
	if subtle.ConstantTimeCompare(data[:sha256.Size], hash.Sum(nil)) != 1 {
		return nil, errors.New("invalid password")
	}
	// ed25519 private key
	memory.Padding()
	privateKey := keysDec[:ed25519.PrivateKeySize]
	// aes key & aes iv
	memory.Padding()
	offset := ed25519.PrivateKeySize
	aesKey = keysDec[offset : offset+aes.Key256Bit]
	memory.Padding()
	offset += aes.Key256Bit
	aesIV = keysDec[offset : offset+aes.IVSize]
	key := make([][]byte, 3)
	key[0] = privateKey
	key[1] = aesKey
	key[2] = aesIV
	return key, nil
}

func loadSelfCertificates(hash *bytes.Buffer, password []byte) ([]*cert.KeyPair, error) {
	// read PEM files
	certPEMBlock, err := ioutil.ReadFile("key/certs.pem")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	keyPEMBlock, err := ioutil.ReadFile("key/keys.pem")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var (
		block *pem.Block
		self  []*cert.KeyPair
	)
	for {
		if len(certPEMBlock) == 0 {
			break
		}

		// load CA certificate
		block, certPEMBlock = pem.Decode(certPEMBlock)
		if block == nil {
			return nil, errors.New("failed to decode key/certs.pem")
		}
		b, err := x509.DecryptPEMBlock(block, password)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		c, err := x509.ParseCertificate(b)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		hash.Write(b)

		// load private key
		block, keyPEMBlock = pem.Decode(keyPEMBlock)
		if block == nil {
			return nil, errors.New("failed to decode key/keys.pem")
		}
		b, err = x509.DecryptPEMBlock(block, password)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		key, err := certutil.ParsePrivateKeyBytes(b)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		hash.Write(b)
		self = append(self, &cert.KeyPair{
			Certificate: c,
			PrivateKey:  key,
		})
	}
	return self, nil
}

func loadSystemCertificates(hash *bytes.Buffer, password []byte) ([]*x509.Certificate, error) {
	systemPEMBlock, err := ioutil.ReadFile("key/system.pem")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var (
		block  *pem.Block
		system []*x509.Certificate
	)
	for {
		if len(systemPEMBlock) == 0 {
			break
		}
		// load CA certificate
		block, systemPEMBlock = pem.Decode(systemPEMBlock)
		if block == nil {
			return nil, errors.New("failed to decode key/system.pem")
		}
		b, err := x509.DecryptPEMBlock(block, password)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		c, err := x509.ParseCertificate(b)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		hash.Write(b)
		system = append(system, c)
	}
	return system, nil
}

// IsLoadSessionKey is used to check is load session key
func (global *global) IsLoadSessionKey() bool {
	return atomic.LoadInt32(&global.loadSessionKey) != 0
}

func (global *global) closeWaitLoadSessionKey() {
	global.closeOnce.Do(func() {
		close(global.waitLoadSessionKey)
	})
}

// LoadSessionKey is used to load session key
func (global *global) LoadSessionKey(data, password []byte) error {
	defer func() {
		security.CoverBytes(data)
		security.CoverBytes(password)
	}()

	global.objectsRWM.Lock()
	defer global.objectsRWM.Unlock()

	if global.IsLoadSessionKey() {
		return errors.New("already load session key")
	}
	key, err := loadSessionKey(data, password)
	if err != nil {
		return errors.WithMessage(err, "failed to load session key")
	}
	// ed25519
	pri := key[0]
	defer security.CoverBytes(pri)
	pub, _ := ed25519.ImportPublicKey(pri[32:])
	global.objects[objPublicKey] = pub
	// calculate key exchange public key
	keyExPub, err := curve25519.ScalarBaseMult(pri[:32])
	if err != nil {
		return errors.WithStack(err)
	}
	global.objects[objKeyExPub] = keyExPub
	// hide private key
	memory := security.NewMemory()
	defer memory.Flush()
	global.objects[objPrivateKey] = security.NewBytes(pri)
	security.CoverBytes(pri)

	// aes crypto about broadcast
	cbc, _ := aes.NewCBC(key[1], key[2])
	global.objects[objBroadcastKey] = cbc

	// load certificates
	PEMHash, err := ioutil.ReadFile("key/pem.hash")
	if err != nil {
		return errors.WithStack(err)
	}
	hashBuf := new(bytes.Buffer)
	hashBuf.Write(password)
	memory.Padding()
	self, err := loadSelfCertificates(hashBuf, password)
	if err != nil {
		return errors.WithMessage(err, "failed to load self CA certificates")
	}
	memory.Padding()
	system, err := loadSystemCertificates(hashBuf, password)
	if err != nil {
		return errors.WithMessage(err, "failed to load system CA certificates")
	}
	// compare hash
	memory.Padding()
	hash := sha256.New()
	hash.Write(hashBuf.Bytes())
	if subtle.ConstantTimeCompare(PEMHash, hash.Sum(nil)) != 1 {
		return errors.New("warning: PEM files has been tampered")
	}
	memory.Padding()
	global.objects[objSelfCA] = self
	memory.Padding()
	global.objects[objSystemCA] = system

	global.closeWaitLoadSessionKey()
	atomic.StoreInt32(&global.loadSessionKey, 1)
	return nil
}

// WaitLoadSessionKey is used to wait load session key
func (global *global) WaitLoadSessionKey() bool {
	<-global.waitLoadSessionKey
	return global.IsLoadSessionKey()
}

// Sign is used to verify controller(handshake) and sign message
func (global *global) Sign(message []byte) []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pri := global.objects[objPrivateKey].(*security.Bytes)
	b := pri.Get()
	defer pri.Put(b)
	return ed25519.Sign(b, message)
}

// Verify is used to verify node certificate
func (global *global) Verify(message, signature []byte) bool {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pub := global.objects[objPublicKey].(ed25519.PublicKey)
	return ed25519.Verify(pub, message, signature)
}

// KeyExchange is use to calculate session key
func (global *global) KeyExchange(publicKey []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pri := global.objects[objPrivateKey].(*security.Bytes)
	b := pri.Get()
	defer pri.Put(b)
	return curve25519.ScalarMult(b[:32], publicKey)
}

// PublicKey is used to get public key
func (global *global) PublicKey() ed25519.PublicKey {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objPublicKey].(ed25519.PublicKey)
}

// KeyExchangePub is used to get key exchange public key
func (global *global) KeyExchangePub() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objKeyExPub].([]byte)
}

// Encrypt is used to encrypt controller broadcast message
func (global *global) Encrypt(data []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	cbc := global.objects[objBroadcastKey].(*aes.CBC)
	return cbc.Encrypt(data)
}

// BroadcastKey is used to get broadcast key
func (global *global) BroadcastKey() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	key, iv := global.objects[objBroadcastKey].(*aes.CBC).KeyIV()
	return append(key, iv...)
}

// Close is used to close global
func (global *global) Close() {
	global.closeWaitLoadSessionKey()
	global.cancel()
	global.TimeSyncer.Stop()
}
