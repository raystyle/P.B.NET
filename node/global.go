package node

import (
	"encoding/base64"
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

// runtime env
// 0 < key < 1048576
const objectKeyMax uint32 = 1048575

type objectKey = uint32

const (
	// controller
	okCtrlPublicKey  objectKey = iota // verify controller role & message
	okCtrlAESCrypto                   // decrypt controller broadcast message
	okCtrlSessionKey                  // after key exchange (aes crypto)

	okStartupTime    // global.configure() time
	okNodeGUID       // identification
	okNodeGUIDEnc    // update self syncSendHeight
	okDBAESCrypto    // encrypt self data(database)
	okCertificate    // for listener
	okPrivateKey     // for sign message
	okPublicKey      // for role verify message
	okKeyExPublicKey // for key exchange

	// sync message
	okSyncSendHeight // sync send

	// confuse object
	okConfusion00
	okConfusion01
	okConfusion02
	okConfusion03
	okConfusion04
	okConfusion05
	okConfusion06
)

type global struct {
	proxyPool  *proxy.Pool
	dnsClient  *dns.Client
	timeSyncer *timesync.TimeSyncer
	object     map[uint32]interface{}
	objectRWM  sync.RWMutex
	configErr  error
	configOnce sync.Once
	wg         sync.WaitGroup
}

func newGlobal(lg logger.Logger, cfg *Config) (*global, error) {
	// <security> basic
	memory := security.NewMemory()
	memory.Padding()
	proxyPool, err := proxy.NewPool(cfg.ProxyClients)
	if err != nil {
		return nil, errors.Wrap(err, "new proxy pool failed")
	}
	memory.Padding()
	dnsClient, err := dns.NewClient(proxyPool, cfg.DNSServers, cfg.DnsCacheDeadline)
	if err != nil {
		return nil, errors.Wrap(err, "new dns client failed")
	}
	memory.Padding()
	// replace logger
	if cfg.CheckMode {
		lg = logger.Discard
	}
	timeSyncer, err := timesync.NewTimeSyncer(
		proxyPool,
		dnsClient,
		lg,
		cfg.TimeSyncerConfigs,
		cfg.TimeSyncerInterval)
	if err != nil {
		return nil, errors.Wrap(err, "new time syncer failed")
	}
	memory.Flush()
	g := global{
		proxyPool:  proxyPool,
		dnsClient:  dnsClient,
		timeSyncer: timeSyncer,
	}
	err = g.configure(cfg)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

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
}

func (global *global) configure(cfg *Config) error {
	global.configOnce.Do(func() {
		global.secPaddingMemory()
		rand := random.New(0)
		// random object map
		global.object = make(map[uint32]interface{})
		for i := 0; i < 32+rand.Int(512); i++ { // 544 * 160 bytes
			key := objectKeyMax + uint32(1+rand.Int(512))
			global.object[key] = rand.Bytes(32 + rand.Int(128))
		}
		global.object[okStartupTime] = global.Now() // set startup time
		global.generateInternalObjects()
		global.configErr = global.loadCtrlConfigs(cfg) // load controller configs
	})
	return global.configErr
}

// 1. node guid
// 2. aes cbc for database & self guid
func (global *global) generateInternalObjects() {
	// generate guid and select one
	global.secPaddingMemory()
	rand := random.New(0)
	g := guid.New(64, nil)
	var guidPool [1024][]byte
	for i := 0; i < len(guidPool); i++ {
		guidPool[i] = g.Get()
	}
	g.Close()
	guidSelected := make([]byte, guid.SIZE)
	copy(guidSelected, guidPool[rand.Int(1024)])
	global.object[okNodeGUID] = guidSelected
	// generate database aes
	aesKey := rand.Bytes(aes.Bit256)
	aesIV := rand.Bytes(aes.IVSize)
	cbc, err := aes.NewCBC(aesKey, aesIV)
	if err != nil {
		panic(err)
	}
	security.FlushBytes(aesKey)
	security.FlushBytes(aesIV)
	global.object[okDBAESCrypto] = cbc
	// encrypt guid
	guidEnc, err := global.DBEncrypt(global.GUID())
	if err != nil {
		panic(err)
	}
	str := base64.StdEncoding.EncodeToString(guidEnc)
	global.object[okNodeGUIDEnc] = str
	// generate private key and public key
	pri, err := ed25519.GenerateKey()
	if err != nil {
		panic(err)
	}
	global.object[okPrivateKey] = pri
	global.object[okPublicKey] = pri.PublicKey()
	// calculate key exchange public key
	pub, err := curve25519.ScalarBaseMult(pri[:32])
	if err != nil {
		panic(err)
	}
	global.object[okKeyExPublicKey] = pub
}

func (global *global) loadCtrlConfigs(cfg *Config) error {
	global.secPaddingMemory()
	// controller public key
	publicKey, err := ed25519.ImportPublicKey(cfg.CtrlPublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	global.object[okCtrlPublicKey] = publicKey
	// controller aes
	key := cfg.CtrlAESCrypto
	l := len(key)
	if l < aes.Bit128+aes.IVSize {
		return errors.New("invalid controller aes key size")
	}
	iv := key[l-aes.IVSize:]
	key = key[:l-aes.IVSize]
	cbc, err := aes.NewCBC(key, iv)
	if err != nil {
		return errors.WithStack(err)
	}
	global.object[okCtrlAESCrypto] = cbc
	// calculate session key and set aes crypto
	pri := global.object[okPrivateKey].(ed25519.PrivateKey)[:32]
	sKey, err := curve25519.ScalarMult(pri, cfg.CtrlExPublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	sCBC, err := aes.NewCBC(sKey, sKey[:aes.IVSize])
	if err != nil {
		return errors.WithStack(err)
	}
	global.object[okCtrlSessionKey] = sCBC
	return nil
}

func (global *global) StartTimeSyncer() error {
	return global.timeSyncer.Start()
}

func (global *global) Now() time.Time {
	return global.timeSyncer.Now().Local()
}

func (global *global) StartupTime() time.Time {
	global.objectRWM.RLock()
	t := global.object[okStartupTime]
	global.objectRWM.RUnlock()
	return t.(time.Time)
}

func (global *global) GUID() []byte {
	global.objectRWM.RLock()
	g := global.object[okNodeGUID]
	global.objectRWM.RUnlock()
	return g.([]byte)
}

func (global *global) GUIDEnc() string {
	global.objectRWM.RLock()
	g := global.object[okNodeGUIDEnc]
	global.objectRWM.RUnlock()
	return g.(string)
}

func (global *global) Certificate() []byte {
	global.objectRWM.RLock()
	c := global.object[okCertificate]
	global.objectRWM.RUnlock()
	if c != nil {
		return c.([]byte)
	} else {
		return nil
	}
}

// Sign is used to get node key exchange public key
func (global *global) KeyExchangePub() []byte {
	global.objectRWM.RLock()
	pub := global.object[okKeyExPublicKey]
	global.objectRWM.RUnlock()
	return pub.([]byte)
}

// Sign is used to get node public key
func (global *global) PublicKey() ed25519.PublicKey {
	global.objectRWM.RLock()
	k := global.object[okPublicKey]
	global.objectRWM.RUnlock()
	return k.(ed25519.PublicKey)
}

// Sign is used to sign node message
func (global *global) Sign(message []byte) []byte {
	global.objectRWM.RLock()
	k := global.object[okPrivateKey]
	global.objectRWM.RUnlock()
	return ed25519.Sign(k.(ed25519.PrivateKey), message)
}

// Encrypt is used to encrypt session data
func (global *global) Encrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[okCtrlSessionKey]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Encrypt(data)
}

// Decrypt is used to decrypt session data
func (global *global) Decrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[okCtrlSessionKey]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Decrypt(data)
}

// DBEncrypt is used to encrypt database data
func (global *global) DBEncrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[okDBAESCrypto]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Encrypt(data)
}

// DBDecrypt is used to decrypt database data
func (global *global) DBDecrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[okDBAESCrypto]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Decrypt(data)
}

// CtrlVerify is used to verify controller message
func (global *global) CtrlVerify(message, signature []byte) bool {
	global.objectRWM.RLock()
	p := global.object[okCtrlPublicKey]
	global.objectRWM.RUnlock()
	return ed25519.Verify(p.(ed25519.PublicKey), message, signature)
}

// CtrlDecrypt is used to decrypt controller broadcast message
func (global *global) CtrlDecrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[okCtrlAESCrypto]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Decrypt(data)
}

func (global *global) Destroy() {
	global.timeSyncer.Stop()
}
