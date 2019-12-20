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

// <warning> must < 1048576
const (
	// sign message
	// hide ed25519 private key
	objPrivateKey    uint32 = 0
	objPrivateKeyBuf uint32 = 64

	// check node certificate
	objPublicKey uint32 = iota + 65

	// for key exchange
	objKeyExPub

	// encrypt controller broadcast message
	// AES CBC
	objBroadcastKey

	// self CA certificates and private keys
	objSelfCA

	// system CA certificates
	objSystemCA
)

type global struct {
	// when configure Node or Beacon, these proxies will not appear
	proxyPool *proxy.Pool

	// when configure Node or Beacon, DNS Server in *dns.Client
	// will appear for select if database without DNS Server
	dnsClient *dns.Client

	// when configure Node or Beacon, time syncer client in
	// *timesync.Syncer will appear for select if database
	// without time syncer client
	timeSyncer *timesync.Syncer

	// objects include various things
	objects    map[uint32]interface{}
	objectsRWM sync.RWMutex

	loadSessionKey     int32
	waitLoadSessionKey chan struct{}

	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
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
	timeSyncer.SetSleep(cfg.TimeSyncFixed, cfg.TimeSyncRandom)
	g := global{
		proxyPool:          proxyPool,
		dnsClient:          dnsClient,
		timeSyncer:         timeSyncer,
		objects:            make(map[uint32]interface{}),
		waitLoadSessionKey: make(chan struct{}, 1),
	}
	g.ctx, g.cancel = context.WithCancel(context.Background())
	return &g, nil
}

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
	return global.proxyPool.Get(tag)
}

// ResolveWithContext is used to resolve domain name with context and options
func (global *global) ResolveWithContext(
	ctx context.Context,
	domain string,
	opts *dns.Options,
) ([]string, error) {
	return global.dnsClient.ResolveWithContext(ctx, domain, opts)
}

// DNSServers is used to get all DNS servers in DNS client
func (global *global) DNSServers() map[string]*dns.Server {
	return global.dnsClient.Servers()
}

// TestDNSOption is used to test client DNS option
func (global *global) TestDNSOption(opts *dns.Options) error {
	_, err := global.dnsClient.TestOption(global.ctx, "cloudflare.com", opts)
	return err
}

// TimeSyncerClients is used to get all time syncer clients in time syncer
func (global *global) TimeSyncerClients() map[string]*timesync.Client {
	return global.timeSyncer.Clients()
}

// StartTimeSyncer is used to start time syncer
func (global *global) StartTimeSyncer() error {
	return global.timeSyncer.Start()
}

// StartTimeSyncerAddLoop is used to start time syncer add loop
func (global *global) StartTimeSyncerAddLoop() {
	global.timeSyncer.StartWalker()
}

// Now is used to get current time
func (global *global) Now() time.Time {
	return global.timeSyncer.Now().Local()
}

// GenerateSessionKey is used to generate session key
func GenerateSessionKey(path string, password []byte) error {
	_, err := os.Stat(path)
	if !os.IsNotExist(err) {
		return errors.Errorf("file: %s already exist", path)
	}
	// generate ed25519 private key(for sign message)
	privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return errors.WithStack(err)
	}
	// generate aes key & iv(for broadcast message)
	aesKey := random.Bytes(aes.Key256Bit)
	aesIV := random.Bytes(aes.IVSize)
	// calculate hash
	buf := new(bytes.Buffer)
	buf.Write(privateKey)
	buf.Write(aesKey)
	buf.Write(aesIV)
	hash := sha256.New()
	hash.Write(buf.Bytes())
	hashData := hash.Sum(nil)
	// encrypt
	hash.Reset()
	hash.Write(password)
	key := hash.Sum(nil)
	hash.Reset()
	hash.Write(key)
	hash.Write([]byte{20, 18, 11, 27})
	iv := hash.Sum(nil)[:aes.IVSize]
	keyEnc, err := aes.CBCEncrypt(buf.Bytes(), key, iv)
	if err != nil {
		return errors.WithStack(err)
	}
	return ioutil.WriteFile(path, append(hashData, keyEnc...), 644)
}

const sessionKeySize = sha256.Size +
	ed25519.PrivateKeySize + aes.Key256Bit + aes.IVSize +
	aes.BlockSize

// return ed25519 private key & aes key & aes iv
func loadSessionKey(path string, password []byte) (keys [][]byte, err error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	if len(file) != sessionKeySize {
		err = errors.New("invalid session key size")
		return
	}
	// decrypt
	memory := security.NewMemory()
	defer memory.Flush()
	memory.Padding()
	hash := sha256.New()
	hash.Write(password)
	key := hash.Sum(nil)
	hash.Reset()
	hash.Write(key)
	hash.Write([]byte{20, 18, 11, 27})
	iv := hash.Sum(nil)[:aes.IVSize]
	keyDec, err := aes.CBCDecrypt(file[sha256.Size:], key, iv)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	// compare hash
	hash.Reset()
	hash.Write(keyDec)
	if subtle.ConstantTimeCompare(file[:sha256.Size], hash.Sum(nil)) != 1 {
		err = errors.New("invalid password")
		return
	}
	// ed25519 private key
	memory.Padding()
	privateKey := keyDec[:ed25519.PrivateKeySize]
	// aes key & aes iv
	memory.Padding()
	offset := ed25519.PrivateKeySize
	aesKey := keyDec[offset : offset+aes.Key256Bit]
	memory.Padding()
	offset += aes.Key256Bit
	aesIV := keyDec[offset : offset+aes.IVSize]
	// return keys
	keys = make([][]byte, 3)
	keys[0] = privateKey
	keys[1] = aesKey
	keys[2] = aesIV
	return
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

func (global *global) isLoadSessionKey() bool {
	return atomic.LoadInt32(&global.loadSessionKey) != 0
}

// LoadSessionKey is used to load session key
func (global *global) LoadSessionKey(password []byte) error {
	defer security.CoverBytes(password)

	global.objectsRWM.Lock()
	defer global.objectsRWM.Unlock()

	if global.isLoadSessionKey() {
		return errors.New("already load session key")
	}
	// load session keys
	keys, err := loadSessionKey("key/session.key", password)
	if err != nil {
		return errors.WithMessage(err, "failed to load session key")
	}
	// ed25519
	pri := keys[0]
	pub, _ := ed25519.ImportPublicKey(pri[32:])
	global.objects[objPublicKey] = pub
	// curve25519
	keyEXPub, err := curve25519.ScalarBaseMult(pri[:32])
	if err != nil {
		return errors.WithStack(err)
	}
	global.objects[objKeyExPub] = keyEXPub
	// aes crypto
	cbc, _ := aes.NewCBC(keys[1], keys[2])
	global.objects[objBroadcastKey] = cbc

	// hide ed25519 private key, not continuity in memory
	rand := random.New()
	memory := security.NewMemory()
	defer memory.Flush()
	for i := 0; i < ed25519.PrivateKeySize; i++ {
		memory.Padding()
		global.objects[objPrivateKey+uint32(i)] = pri[i]
		pri[i] = byte(rand.Int64())
	}
	global.objects[objPrivateKeyBuf] = make([]byte, ed25519.PrivateKeySize)
	// load certificates
	PEMHash, err := ioutil.ReadFile("key/pem.hash")
	if err != nil {
		return errors.WithStack(err)
	}
	hashBuf := new(bytes.Buffer)
	hashBuf.Write(password)

	memory.Padding()
	kps, err := loadSelfCertificates(hashBuf, password)
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
	global.objects[objSelfCA] = kps
	memory.Padding()
	global.objects[objSystemCA] = system

	global.closeOnce.Do(func() { close(global.waitLoadSessionKey) })
	atomic.StoreInt32(&global.loadSessionKey, 1)
	return nil
}

// WaitLoadSessionKey is used to wait load session key
func (global *global) WaitLoadSessionKey() bool {
	<-global.waitLoadSessionKey
	return global.isLoadSessionKey()
}

// Encrypt is used to encrypt controller broadcast message
func (global *global) Encrypt(data []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	cbc := global.objects[objBroadcastKey]
	return cbc.(*aes.CBC).Encrypt(data)
}

// TODO sync pool

// Sign is used to verify controller(handshake) and sign message
func (global *global) Sign(message []byte) []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pri := make([]byte, ed25519.PrivateKeySize)
	defer security.CoverBytesFast(pri)
	for i := 0; i < ed25519.PrivateKeySize; i++ {
		pri[i] = global.objects[objPrivateKey+uint32(i)].(byte)
	}
	return ed25519.Sign(pri, message)
}

// Sign is used to verify controller(handshake) and sign message
func (global *global) SignO(message []byte) []byte {
	global.objectsRWM.Lock()
	defer global.objectsRWM.Unlock()
	pri := global.objects[objPrivateKeyBuf].([]byte)
	defer func() {
		for i := 0; i < ed25519.PrivateKeySize; i++ {
			pri[i] = 0
		}
	}()
	for i := 0; i < ed25519.PrivateKeySize; i++ {
		pri[i] = global.objects[objPrivateKey+uint32(i)].(byte)
	}
	return ed25519.Sign(pri, message)
}

// Verify is used to verify node certificate
func (global *global) Verify(message, signature []byte) bool {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pub := global.objects[objPublicKey]
	return ed25519.Verify(pub.(ed25519.PublicKey), message, signature)
}

// PublicKey is used to get public key
func (global *global) PublicKey() ed25519.PublicKey {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objPublicKey].(ed25519.PublicKey)
}

// BroadcastKey is used to get broadcast key
func (global *global) BroadcastKey() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	key, iv := global.objects[objBroadcastKey].(*aes.CBC).KeyIV()
	return append(key, iv...)
}

// KeyExchangePub is used to get key exchange public key
func (global *global) KeyExchangePub() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objKeyExPub].([]byte)
}

// KeyExchange is use to calculate session key
func (global *global) KeyExchange(publicKey []byte) ([]byte, error) {
	pri := make([]byte, 32)
	defer security.CoverBytes(pri)
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	for i := 0; i < 32; i++ {
		pri[i] = global.objects[objPrivateKey+uint32(i)].(byte)
	}
	return curve25519.ScalarMult(pri, publicKey)
}

// Close is used to close global
func (global *global) Close() {
	global.cancel()
	global.timeSyncer.Stop()
	global.closeOnce.Do(func() { close(global.waitLoadSessionKey) })
}
