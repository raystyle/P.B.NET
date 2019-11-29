package controller

import (
	"bytes"
	"crypto/subtle"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/sha256"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/security"
	"project/internal/timesync"
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

	objects    map[uint32]interface{}
	objectsRWM sync.RWMutex

	loadSessionKey     int32
	waitLoadSessionKey chan struct{}
	closeOnce          sync.Once
}

func newGlobal(logger logger.Logger, config *Config) (*global, error) {
	cfg := config.Global

	// load builtin proxy clients
	const errProxy = "failed to load builtin proxy clients"
	b, err := ioutil.ReadFile("builtin/proxy.toml")
	if err != nil {
		return nil, errors.Wrap(err, errProxy)
	}
	var proxyClients []*proxy.Client
	err = toml.Unmarshal(b, &proxyClients)
	if err != nil {
		return nil, errors.Wrap(err, errProxy)
	}
	proxyPool := proxy.NewPool()
	for _, client := range proxyClients {
		err = proxyPool.Add(client)
		if err != nil {
			return nil, errors.Wrap(err, errProxy)
		}
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
	tsClients := make(map[string]*timesync.Client)
	err = toml.Unmarshal(b, &tsClients)
	if err != nil {
		return nil, errors.Wrap(err, errTSC)
	}
	timeSyncer := timesync.New(proxyPool, dnsClient, logger)
	for tag, client := range tsClients {
		err = timeSyncer.Add("builtin_"+tag, client)
		if err != nil {
			return nil, errors.Wrap(err, errTSC)
		}
	}
	err = timeSyncer.SetSyncInterval(cfg.TimeSyncInterval)
	if err != nil {
		return nil, errors.Wrap(err, errTSC)
	}

	return &global{
		proxyPool:          proxyPool,
		dnsClient:          dnsClient,
		timeSyncer:         timeSyncer,
		objects:            make(map[uint32]interface{}),
		waitLoadSessionKey: make(chan struct{}, 1),
	}, nil
}

// StartTimeSyncer is used to start time syncer
func (global *global) StartTimeSyncer() error {
	return global.timeSyncer.Start()
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
	if len(password) < 12 {
		return errors.New("password is too short")
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
	hash := sha256.Bytes(buf.Bytes())

	// encrypt
	key := sha256.Bytes(password)
	iv := sha256.Bytes(append(key, []byte{20, 18, 11, 27}...))[:aes.IVSize]
	keyEnc, err := aes.CBCEncrypt(buf.Bytes(), key, iv)
	if err != nil {
		return errors.WithStack(err)
	}
	return ioutil.WriteFile(path, append(hash, keyEnc...), 644)
}

const sessionKeySize = sha256.Size + ed25519.PrivateKeySize + aes.Key256Bit + aes.IVSize

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
	key := sha256.Bytes(password)
	iv := sha256.Bytes(append(key, []byte{20, 18, 11, 27}...))[:aes.IVSize]
	keyDec, err := aes.CBCDecrypt(file[sha256.Size:], key, iv)
	if err != nil {
		err = errors.WithStack(err)
		return
	}

	// compare hash
	if subtle.ConstantTimeCompare(file[:sha256.Size], keyDec) != 1 {
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

	keys = make([][]byte, 3)
	keys[0] = privateKey
	keys[1] = aesKey
	keys[2] = aesIV
	return
}

// <warning> must < 1048576

const (
	// verify controller role & sign message
	// hide ed25519 private key
	objPrivateKey uint32 = 0

	// check node certificate
	objPublicKey uint32 = iota + 64

	// for key exchange
	objKeyExPub

	// encrypt controller broadcast message
	objAESCrypto
)

func (global *global) isLoadSessionKey() bool {
	return atomic.LoadInt32(&global.loadSessionKey) != 0
}

// LoadSessionKey is used to load session key
func (global *global) LoadSessionKey(password []byte) error {
	defer security.FlushBytes(password)

	global.objectsRWM.Lock()
	defer global.objectsRWM.Unlock()

	if global.isLoadSessionKey() {
		return errors.New("already load session key")
	}
	// load session keys
	keys, err := loadSessionKey("key/session.key", password)
	if err != nil {
		return errors.WithStack(err)
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
	global.objects[objAESCrypto] = cbc

	// hide ed25519 private key, not continuity in memory
	rand := random.New(0)
	memory := security.NewMemory()
	defer memory.Flush()
	for i := 0; i < ed25519.PrivateKeySize; i++ {
		memory.Padding()
		global.objects[objPrivateKey+uint32(i)] = pri[i]
		pri[i] = byte(rand.Int64())
	}

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
	cbc := global.objects[objAESCrypto]
	return cbc.(*aes.CBC).Encrypt(data)
}

// Sign is used to verify controller(handshake) and sign message
func (global *global) Sign(message []byte) []byte {
	pri := make([]byte, ed25519.PrivateKeySize)
	defer security.FlushBytes(pri)
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
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

// KeyExchangePub is used to get key exchange public key
func (global *global) KeyExchangePub() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pub := global.objects[objKeyExPub]
	return pub.([]byte)
}

// KeyExchange is use to calculate session key
func (global *global) KeyExchange(publicKey []byte) ([]byte, error) {
	pri := make([]byte, 32)
	defer security.FlushBytes(pri)
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	for i := 0; i < 32; i++ {
		pri[i] = global.objects[objPrivateKey+uint32(i)].(byte)
	}
	return curve25519.ScalarMult(pri, publicKey)
}

// Close is used to close global
func (global *global) Close() {
	global.timeSyncer.Stop()
	global.closeOnce.Do(func() { close(global.waitLoadSessionKey) })
}
