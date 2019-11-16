package node

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/security"
	"project/internal/timesync"
)

type global struct {
	proxyPool  *proxy.Pool
	dnsClient  *dns.Client
	timeSyncer *timesync.Syncer
	object     map[uint32]interface{}
	objectRWM  sync.RWMutex
	spmCount   int // secPaddingMemory execute time
	wg         sync.WaitGroup
}

func newGlobal(lg logger.Logger, config *Config) (*global, error) {
	cfg := config.Global
	memory := security.NewMemory()
	memory.Padding()
	// add proxy client
	proxyPool := proxy.NewPool()

	memory.Padding()
	// set cache expire
	dnsClient, err := dns.NewClient(proxyPool, cfg.DNSServers, cfg.DNSCacheExpire)
	if err != nil {
		return nil, errors.Wrap(err, "new dns client failed")
	}
	memory.Padding()
	// replace logger
	if config.CheckMode {
		lg = logger.Discard
	}
	timeSyncer, err := timesync.New(
		proxyPool,
		dnsClient,
		lg,
		cfg.TimeSyncerClients,
		cfg.TimeSyncInterval)
	if err != nil {
		return nil, errors.Wrap(err, "new time syncer failed")
	}
	memory.Flush()
	g := global{
		proxyPool:  proxyPool,
		dnsClient:  dnsClient,
		timeSyncer: timeSyncer,
	}
	err = g.configure(config)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// <warning> must < 1048576
const (
	_ uint32 = iota

	objCtrlPublicKey  // verify controller role & message
	objCtrlAESCrypto  // decrypt controller broadcast message
	objCtrlSessionKey // after key exchange (aes crypto)

	objStartupTime    // global.configure() time
	objNodeGUID       // identification
	objDBAESCrypto    // encrypt self data(database)
	objCertificate    // for server.handshake
	objPrivateKey     // for sign message
	objPublicKey      // for role verify message
	objKeyExPublicKey // for key exchange
)

// <security>
func (global *global) secPaddingMemory() {
	rand := random.New(0)
	memory := security.NewMemory()
	security.PaddingMemory()
	padding := func() {
		for i := 0; i < 32+rand.Int(256); i++ {
			memory.Padding()
		}
	}
	global.wg.Add(1)
	go func() {
		padding()
		global.wg.Done()
	}()
	padding()
	global.wg.Wait()
	global.spmCount += 1
}

func (global *global) configure(cfg *Config) error {
	// random object map
	global.secPaddingMemory()
	rand := random.New(0)
	global.object = make(map[uint32]interface{})
	for i := 0; i < 32+rand.Int(512); i++ { // 544 * 160 bytes
		key := uint32(1 + rand.Int(512))
		global.object[key] = rand.Bytes(32 + rand.Int(128))
		// clean certificate
		global.object[objCertificate] = nil
	}
	// -----------------generate internal objects-----------------
	// set startup time
	global.object[objStartupTime] = time.Now()
	// generate guid and select one
	global.secPaddingMemory()
	rand = random.New(0)
	g := guid.New(64, nil)
	var guidPool [1024][]byte
	for i := 0; i < len(guidPool); i++ {
		guidPool[i] = g.Get()
	}
	g.Close()
	guidSelected := make([]byte, guid.Size)
	copy(guidSelected, guidPool[rand.Int(1024)])
	global.object[objNodeGUID] = guidSelected
	// generate private key and public key
	global.secPaddingMemory()
	pri, err := ed25519.GenerateKey()
	if err != nil {
		panic(err)
	}
	global.object[objPrivateKey] = pri
	global.object[objPublicKey] = pri.PublicKey()
	// calculate key exchange public key
	global.secPaddingMemory()
	pub, err := curve25519.ScalarBaseMult(pri[:32])
	if err != nil {
		panic(err)
	}
	global.object[objKeyExPublicKey] = pub
	// generate database aes
	global.secPaddingMemory()
	rand = random.New(0)
	aesKey := rand.Bytes(aes.Key256Bit)
	aesIV := rand.Bytes(aes.IVSize)
	cbc, err := aes.NewCBC(aesKey, aesIV)
	if err != nil {
		panic(err)
	}
	security.FlushBytes(aesKey)
	security.FlushBytes(aesIV)
	global.object[objDBAESCrypto] = cbc
	// -----------------load controller configs-----------------
	// controller public key
	global.secPaddingMemory()
	publicKey, err := ed25519.ImportPublicKey(cfg.CTRL.PublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	global.object[objCtrlPublicKey] = publicKey
	// controller aes
	global.secPaddingMemory()
	key := cfg.CTRL.AESCrypto
	l := len(key)
	if l < aes.Key128Bit+aes.IVSize {
		return errors.New("invalid controller aes key size")
	}
	iv := key[l-aes.IVSize:]
	key = key[:l-aes.IVSize]
	cbc, err = aes.NewCBC(key, iv)
	if err != nil {
		return errors.WithStack(err)
	}
	global.object[objCtrlAESCrypto] = cbc
	// calculate session key and set aes crypto
	global.secPaddingMemory()
	pri = global.object[objPrivateKey].(ed25519.PrivateKey)[:32]
	sKey, err := curve25519.ScalarMult(pri, cfg.CTRL.ExPublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	sCBC, err := aes.NewCBC(sKey, sKey[:aes.IVSize])
	if err != nil {
		return errors.WithStack(err)
	}
	global.object[objCtrlSessionKey] = sCBC
	return nil
}

const spmCount = 8 // secPaddingMemory execute time

// check secPaddingMemory
func (global *global) OK() bool {
	return global.spmCount == spmCount
}

func (global *global) StartTimeSyncer() error {
	return global.timeSyncer.Start()
}

func (global *global) Now() time.Time {
	return global.timeSyncer.Now().Local()
}

func (global *global) StartupTime() time.Time {
	global.objectRWM.RLock()
	t := global.object[objStartupTime]
	global.objectRWM.RUnlock()
	return t.(time.Time)
}

func (global *global) GUID() []byte {
	global.objectRWM.RLock()
	g := global.object[objNodeGUID]
	global.objectRWM.RUnlock()
	return g.([]byte)
}

func (global *global) Certificate() []byte {
	global.objectRWM.RLock()
	c := global.object[objCertificate]
	global.objectRWM.RUnlock()
	if c != nil {
		return c.([]byte)
	} else {
		return nil
	}
}

func (global *global) SetCertificate(cert []byte) error {
	global.objectRWM.Lock()
	defer global.objectRWM.Unlock()
	if _, ok := global.object[objCertificate]; !ok {
		c := make([]byte, len(cert))
		copy(c, cert)
		global.object[objCertificate] = c
		return nil
	} else {
		return errors.New("certificate has been set")
	}
}

// KeyExchangePub is used to get node key exchange public key
func (global *global) KeyExchangePub() []byte {
	global.objectRWM.RLock()
	pub := global.object[objKeyExPublicKey]
	global.objectRWM.RUnlock()
	return pub.([]byte)
}

// PublicKey is used to get node public key
func (global *global) PublicKey() ed25519.PublicKey {
	global.objectRWM.RLock()
	k := global.object[objPublicKey]
	global.objectRWM.RUnlock()
	return k.(ed25519.PublicKey)
}

// DBEncrypt is used to encrypt database data
func (global *global) DBEncrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[objDBAESCrypto]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Encrypt(data)
}

// DBDecrypt is used to decrypt database data
func (global *global) DBDecrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[objDBAESCrypto]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Decrypt(data)
}

// Sign is used to sign node message
func (global *global) Sign(message []byte) []byte {
	global.objectRWM.RLock()
	k := global.object[objPrivateKey]
	global.objectRWM.RUnlock()
	return ed25519.Sign(k.(ed25519.PrivateKey), message)
}

// Encrypt is used to encrypt session data
func (global *global) Encrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[objCtrlSessionKey]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Encrypt(data)
}

// Decrypt is used to decrypt session data
func (global *global) Decrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[objCtrlSessionKey]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Decrypt(data)
}

// CtrlVerify is used to verify controller message
func (global *global) CtrlVerify(message, signature []byte) bool {
	global.objectRWM.RLock()
	p := global.object[objCtrlPublicKey]
	global.objectRWM.RUnlock()
	return ed25519.Verify(p.(ed25519.PublicKey), message, signature)
}

// CtrlDecrypt is used to decrypt controller broadcast message
func (global *global) CtrlDecrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[objCtrlAESCrypto]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Decrypt(data)
}

func (global *global) Close() {
	global.timeSyncer.Stop()
}
