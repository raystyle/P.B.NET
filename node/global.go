package node

import (
	"encoding/base64"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
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
		global.generateInternalObjects()
		global.configErr = global.loadCtrlConfigs(cfg)
	})
	return global.configErr
}

func (global *global) loadCtrlConfigs(cfg *Config) error {
	global.secPaddingMemory()
	// controller ed25519 public key
	publicKey, err := ed25519.ImportPublicKey(cfg.CtrlED25519)
	if err != nil {
		return errors.WithStack(err)
	}
	global.object[ctrlED25519] = publicKey
	// controller aes
	key := cfg.CtrlAESKey
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
	global.object[ctrlAESCrypto] = cbc
	return nil
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
	global.object[nodeGUID] = guidSelected
	// generate database aes
	aesKey := rand.Bytes(aes.Bit256)
	aesIV := rand.Bytes(aes.IVSize)
	cbc, err := aes.NewCBC(aesKey, aesIV)
	if err != nil {
		panic(err)
	}
	security.FlushBytes(aesKey)
	security.FlushBytes(aesIV)
	global.object[dbAESCrypto] = cbc
	// encrypt guid
	guidEnc, err := global.DBEncrypt(global.GUID())
	if err != nil {
		panic(err)
	}
	str := base64.StdEncoding.EncodeToString(guidEnc)
	global.object[nodeGUIDEnc] = str
}

// about internal

func (global *global) StartTimeSyncer() error {
	return global.timeSyncer.Start()
}

func (global *global) Now() time.Time {
	return global.timeSyncer.Now().Local()
}

func (global *global) GUID() []byte {
	global.objectRWM.RLock()
	g := global.object[nodeGUID]
	global.objectRWM.RUnlock()
	return g.([]byte)
}

func (global *global) GUIDEnc() string {
	global.objectRWM.RLock()
	g := global.object[nodeGUIDEnc]
	global.objectRWM.RUnlock()
	return g.(string)
}

func (global *global) Certificate() []byte {
	global.objectRWM.RLock()
	c := global.object[certificate]
	global.objectRWM.RUnlock()
	if c != nil {
		return c.([]byte)
	} else {
		return nil
	}
}

// use controller public key to verify message
func (global *global) CTRLVerify(message, signature []byte) bool {
	global.objectRWM.RLock()
	p := global.object[ctrlED25519]
	global.objectRWM.RUnlock()
	return ed25519.Verify(p.(ed25519.PublicKey), message, signature)
}

func (global *global) CTRLDecrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[ctrlAESCrypto]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Decrypt(data)
}

func (global *global) DBEncrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[dbAESCrypto]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Encrypt(data)
}

func (global *global) DBDecrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	cbc := global.object[dbAESCrypto]
	global.objectRWM.RUnlock()
	return cbc.(*aes.CBC).Decrypt(data)
}

func (global *global) Destroy() {
	global.timeSyncer.Stop()
}
