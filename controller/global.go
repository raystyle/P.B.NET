package controller

import (
	"crypto/x509"
	"io/ioutil"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/rsa"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/timesync"
)

type global struct {
	keyDir string

	proxyPool    *proxy.Pool
	dnsClient    *dns.Client
	timeSyncer   *timesync.Syncer
	objects      map[uint32]interface{}
	objectsRWM   sync.RWMutex
	loadKeys     int32
	waitLoadKeys chan struct{}
}

func newGlobal(logger logger.Logger, config *Config) (*global, error) {
	cfg := config.Global
	proxyPool := proxy.NewPool()
	// load builtin proxy client

	// load builtin dns clients
	dnsServers := make(map[string]*dns.Server)
	b, err := ioutil.ReadFile(cfg.BuiltinDir + "/dnsserver.toml")
	if err != nil {
		return nil, errors.Wrap(err, "load builtin dns clients failed")
	}
	err = toml.Unmarshal(b, &dnsServers)
	if err != nil {
		return nil, errors.Wrap(err, "load builtin dns clients failed")
	}
	// add dns servers
	for tag, server := range dnsServers {
		dnsServers["builtin_"+tag] = server
		delete(dnsServers, tag) // rename
	}
	dnsClient := dns.NewClient(proxyPool)

	// add DNS clients

	// set expire time

	// load builtin time syncer config
	tsConfigs := make(map[string]*timesync.Client)
	b, err = ioutil.ReadFile(cfg.BuiltinDir + "/timesyncer.toml")
	if err != nil {
		return nil, errors.Wrap(err, "load builtin time syncer configs failed")
	}
	err = toml.Unmarshal(b, &tsConfigs)
	if err != nil {
		return nil, errors.Wrap(err, "load builtin time syncer configs failed")
	}
	// add time syncer configs
	for tag, config := range tsConfigs {
		tsConfigs["builtin_"+tag] = config
		delete(tsConfigs, tag) // rename
	}
	timeSyncer := timesync.New(proxyPool, dnsClient, logger)

	// set time sync interval

	return &global{
		proxyPool:    proxyPool,
		dnsClient:    dnsClient,
		timeSyncer:   timeSyncer,
		keyDir:       cfg.KeyDir,
		objects:      make(map[uint32]interface{}),
		waitLoadKeys: make(chan struct{}, 1),
	}, nil
}

func (global *global) StartTimeSyncer() error {
	return global.timeSyncer.Start()
}

func (global *global) Now() time.Time {
	return global.timeSyncer.Now().Local()
}

// <warning> must < 1048576
const (
	_ uint32 = iota

	objPrivateKey        // verify controller role & sign message
	objPublicKey         // for role
	objKeyExPub          // for key exchange
	objAESCrypto         // encrypt controller broadcast message
	objCACertificates    // x509.Certificate
	objCAPrivateKeys     // rsa.PrivateKey
	objCACertificatesStr // x509.Certificate
)

func (global *global) LoadKeys(password string) error {
	if global.isLoadKeys() {
		return errors.New("already load keys")
	}
	global.objectsRWM.Lock()
	defer global.objectsRWM.Unlock()
	// load CAs
	caData, err := ioutil.ReadFile(global.keyDir + "/ca.toml")
	if err != nil {
		return errors.WithStack(err)
	}
	caList := struct {
		CA []struct {
			Cert string `toml:"cert"`
			Key  string `toml:"key"`
		} `toml:"ca"`
	}{}
	err = toml.Unmarshal(caData, &caList)
	if err != nil {
		return errors.WithStack(err)
	}
	l := len(caList.CA)
	if l == 0 {
		return errors.New("no CA certificates")
	}
	caCerts := make([]*x509.Certificate, l)
	caCertsStr := make([]string, l)
	caKeys := make([]*rsa.PrivateKey, l)
	for i := 0; i < l; i++ {
		crt, err := cert.Parse([]byte(caList.CA[i].Cert))
		if err != nil {
			return errors.WithStack(err)
		}
		caCerts[i] = crt
		caCertsStr[i] = caList.CA[i].Cert
		pri, err := rsa.ImportPrivateKeyFromPEM([]byte(caList.CA[i].Key))
		if err != nil {
			return errors.WithStack(err)
		}
		caKeys[i] = pri
	}
	global.objects[objCACertificates] = caCerts
	global.objects[objCACertificatesStr] = caCertsStr
	global.objects[objCAPrivateKeys] = caKeys
	// keys
	keys, err := loadCtrlKeys(global.keyDir+"/ctrl.key", password)
	if err != nil {
		return errors.WithStack(err)
	}
	// ed25519
	pri, _ := ed25519.ImportPrivateKey(keys[0])
	global.objects[objPrivateKey] = pri
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

	atomic.StoreInt32(&global.loadKeys, 1)
	close(global.waitLoadKeys)
	return nil
}

func (global *global) isLoadKeys() bool {
	return atomic.LoadInt32(&global.loadKeys) != 0
}

func (global *global) WaitLoadKeys() bool {
	<-global.waitLoadKeys
	return global.isLoadKeys()
}

// Encrypt is used to encrypt controller broadcast message
func (global *global) Encrypt(data []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	cbc := global.objects[objAESCrypto]
	global.objectsRWM.RUnlock()
	return cbc.(*aes.CBC).Encrypt(data)
}

// Sign is used to verify controller(handshake) and sign message
func (global *global) Sign(message []byte) []byte {
	global.objectsRWM.RLock()
	pri := global.objects[objPrivateKey]
	global.objectsRWM.RUnlock()
	return ed25519.Sign(pri.(ed25519.PrivateKey), message)
}

// Verify is used to verify node certificate
func (global *global) Verify(message, signature []byte) bool {
	global.objectsRWM.RLock()
	pub := global.objects[objPublicKey]
	global.objectsRWM.RUnlock()
	return ed25519.Verify(pub.(ed25519.PublicKey), message, signature)
}

// KeyExchangePub is used to get key exchange public key
func (global *global) KeyExchangePub() []byte {
	global.objectsRWM.RLock()
	pub := global.objects[objKeyExPub]
	global.objectsRWM.RUnlock()
	return pub.([]byte)
}

// KeyExchange is use to calculate session key
func (global *global) KeyExchange(publicKey []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	pri := global.objects[objPrivateKey]
	global.objectsRWM.RUnlock()
	return curve25519.ScalarMult(pri.(ed25519.PrivateKey)[:32], publicKey)
}

func (global *global) CACertificates() []*x509.Certificate {
	global.objectsRWM.RLock()
	crt := global.objects[objCACertificates]
	global.objectsRWM.RUnlock()
	return crt.([]*x509.Certificate)
}

func (global *global) CAPrivateKeys() []*rsa.PrivateKey {
	global.objectsRWM.RLock()
	pri := global.objects[objCAPrivateKeys]
	global.objectsRWM.RUnlock()
	return pri.([]*rsa.PrivateKey)
}

func (global *global) CACertificatesStr() []string {
	global.objectsRWM.RLock()
	crt := global.objects[objCACertificatesStr]
	global.objectsRWM.RUnlock()
	return crt.([]string)
}

func (global *global) TestSetObject(key uint32, obj interface{}) {
	global.objectsRWM.Lock()
	global.objects[key] = obj
	global.objectsRWM.Unlock()
}

func (global *global) Close() {
	global.timeSyncer.Stop()
	if !global.isLoadKeys() {
		close(global.waitLoadKeys)
	}
}
