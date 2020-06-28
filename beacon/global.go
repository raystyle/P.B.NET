package beacon

import (
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/cert"
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
	CertPool   *cert.Pool
	ProxyPool  *proxy.Pool
	DNSClient  *dns.Client
	TimeSyncer *timesync.Syncer

	objects    map[uint32]interface{}
	objectsRWM sync.RWMutex

	// paddingMemory execute time
	spmCount int
	rand     *random.Rand
	wg       sync.WaitGroup
}

func newGlobal(logger logger.Logger, config *Config) (*global, error) {
	cfg := config.Global

	memory := security.NewMemory()
	defer memory.Flush()

	// certificate pool
	certPool, err := cfg.CertPool.ToPool()
	if err != nil {
		return nil, err
	}
	// proxy client
	proxyPool := proxy.NewPool(certPool)
	for i := 0; i < len(cfg.ProxyClients); i++ {
		memory.Padding()
		err := proxyPool.Add(cfg.ProxyClients[i])
		if err != nil {
			return nil, err
		}
	}
	// check client config
	_, err = proxyPool.Get(config.Client.ProxyTag)
	if err != nil {
		return nil, err
	}
	// DNS client
	dnsClient := dns.NewClient(certPool, proxyPool)
	for tag, server := range cfg.DNSServers {
		memory.Padding()
		err := dnsClient.Add(tag, server)
		if err != nil {
			return nil, err
		}
	}
	err = dnsClient.SetCacheExpireTime(cfg.DNSCacheExpire)
	if err != nil {
		return nil, err
	}
	// time syncer
	timeSyncer := timesync.New(certPool, proxyPool, dnsClient, logger)
	for tag, client := range cfg.TimeSyncerClients {
		memory.Padding()
		err = timeSyncer.Add(tag, client)
		if err != nil {
			return nil, err
		}
	}
	err = timeSyncer.SetSyncInterval(cfg.TimeSyncInterval)
	if err != nil {
		return nil, err
	}
	err = timeSyncer.SetSleep(cfg.TimeSyncSleepFixed, cfg.TimeSyncSleepRandom)
	if err != nil {
		return nil, err
	}

	global := global{
		CertPool:   certPool,
		ProxyPool:  proxyPool,
		DNSClient:  dnsClient,
		TimeSyncer: timeSyncer,
		rand:       random.NewRand(),
	}
	err = global.configure(config)
	if err != nil {
		return nil, err
	}
	return &global, nil
}

const (
	// verify controller role & message
	objCtrlPublicKey uint32 = iota

	// after key exchange (aes crypto)
	objSessionKey

	// after key exchange, key is session key
	objSessionKeyData

	// global.configure() time
	objStartupTime

	// identification
	objBeaconGUID

	// for sign message
	objPrivateKey

	// for role verify message
	objPublicKey

	// for key exchange
	objKexPublicKey
)

// <security>
func (global *global) paddingMemory() {
	memory := security.NewMemory()
	security.PaddingMemory()
	defer security.FlushMemory()
	padding := func() {
		for i := 0; i < 32+global.rand.Int(256); i++ {
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
	global.spmCount++
}

func (global *global) configure(cfg *Config) error {
	// random objects map
	global.paddingMemory()
	global.objects = make(map[uint32]interface{})
	for i := 0; i < 32+global.rand.Int(512); i++ { // 544 * 160 bytes
		key := uint32(1 + global.rand.Int(512))
		global.objects[key] = global.rand.Bytes(32 + global.rand.Int(128))
	}
	// -----------------generate internal objects-----------------
	// set startup time
	global.objects[objStartupTime] = time.Now()
	// generate guid and select one
	global.paddingMemory()
	g := guid.New(64, nil)
	defer g.Close()
	var guidPool [1024]guid.GUID
	for i := 0; i < len(guidPool); i++ {
		copy(guidPool[i][:], g.Get()[:])
	}
	guidSelected := new(guid.GUID)
	err := guidSelected.Write(guidPool[global.rand.Int(1024)][:])
	if err != nil {
		panic(err)
	}
	global.objects[objBeaconGUID] = guidSelected
	// generate private key and public key
	global.paddingMemory()
	pri, err := ed25519.GenerateKey()
	if err != nil {
		panic(err)
	}
	defer security.CoverBytes(pri)
	global.objects[objPublicKey] = pri.PublicKey()
	// calculate key exchange public key
	global.paddingMemory()
	kexPublicKey, err := curve25519.ScalarBaseMult(pri[:curve25519.ScalarSize])
	if err != nil {
		panic(err)
	}
	global.objects[objKexPublicKey] = kexPublicKey
	global.paddingMemory()
	global.objects[objPrivateKey] = security.NewBytes(pri)
	security.CoverBytes(pri)
	// -----------------load controller configs-----------------
	// controller public key
	global.paddingMemory()
	publicKey, err := ed25519.ImportPublicKey(cfg.Ctrl.PublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	global.objects[objCtrlPublicKey] = publicKey
	// calculate session key and set aes crypto
	global.paddingMemory()
	sb := global.objects[objPrivateKey].(*security.Bytes)
	b := sb.Get()
	defer sb.Put(b)
	in := b[:curve25519.ScalarSize]
	sessionKey, err := curve25519.ScalarMult(in, cfg.Ctrl.KexPublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	defer security.CoverBytes(sessionKey)
	cbc, err := aes.NewCBC(sessionKey, sessionKey[:aes.IVSize])
	if err != nil {
		return errors.WithStack(err)
	}
	global.objects[objSessionKey] = cbc
	// for HMAC-SHA256
	global.objects[objSessionKeyData] = security.NewBytes(sessionKey)
	return nil
}

const spmCount = 9 // global.paddingMemory() execute count

// OK is used to check debug.
func (global *global) OK() bool {
	return global.spmCount == spmCount
}

// Now is used to get current time.
func (global *global) Now() time.Time {
	return global.TimeSyncer.Now()
}

// SetStartupTime is used to set startup time.
func (global *global) SetStartupTime(t time.Time) {
	global.objectsRWM.Lock()
	defer global.objectsRWM.Unlock()
	global.objects[objStartupTime] = t
}

// GetStartupTime is used to get startup time.
func (global *global) GetStartupTime() time.Time {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objStartupTime].(time.Time)
}

// GUID is used to get Beacon GUID.
func (global *global) GUID() *guid.GUID {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objBeaconGUID].(*guid.GUID)
}

// Sign is used to sign message.
func (global *global) Sign(message []byte) []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pri := global.objects[objPrivateKey].(*security.Bytes)
	b := pri.Get()
	defer pri.Put(b)
	return ed25519.Sign(b, message)
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

// Encrypt is used to encrypt session data.
func (global *global) Encrypt(data []byte) ([]byte, error) {
	cbc := func() *aes.CBC {
		global.objectsRWM.RLock()
		defer global.objectsRWM.RUnlock()
		return global.objects[objSessionKey].(*aes.CBC)
	}()
	return cbc.Encrypt(data)
}

// Decrypt is used to decrypt session data.
func (global *global) Decrypt(data []byte) ([]byte, error) {
	cbc := func() *aes.CBC {
		global.objectsRWM.RLock()
		defer global.objectsRWM.RUnlock()
		return global.objects[objSessionKey].(*aes.CBC)
	}()
	return cbc.Decrypt(data)
}

// SessionKey is used to get session key.
func (global *global) SessionKey() *security.Bytes {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objSessionKeyData].(*security.Bytes)
}

// CtrlPublicKey is used to get Controller public key.
func (global *global) CtrlPublicKey() ed25519.PublicKey {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objCtrlPublicKey].(ed25519.PublicKey)
}

// CtrlVerify is used to verify controller message.
func (global *global) CtrlVerify(message, signature []byte) bool {
	return ed25519.Verify(global.CtrlPublicKey(), message, signature)
}

// Close is used to close global.
func (global *global) Close() {
	global.TimeSyncer.Stop()
}
