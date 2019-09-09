package controller

import (
	"io/ioutil"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/timesync"
)

type global struct {
	proxyPool    *proxy.Pool
	dnsClient    *dns.Client
	timeSyncer   *timesync.TimeSyncer
	keyDir       string
	object       map[uint32]interface{}
	objectRWM    sync.RWMutex
	isLoadKeys   int32
	waitLoadKeys chan struct{}
}

func newGlobal(lg logger.Logger, cfg *Config) (*global, error) {
	proxyPool, _ := proxy.NewPool(nil)
	// load builtin dns clients
	dnsServers := make(map[string]*dns.Server)
	b, err := ioutil.ReadFile(cfg.BuiltinDir + "/dnsclient.toml")
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
	dnsClient, err := dns.NewClient(proxyPool, dnsServers, cfg.DNSCacheDeadline)
	if err != nil {
		return nil, errors.Wrap(err, "new dns client failed")
	}
	// load builtin time syncer config
	tsConfigs := make(map[string]*timesync.Config)
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
	timeSyncer, err := timesync.NewTimeSyncer(
		proxyPool,
		dnsClient,
		lg,
		tsConfigs,
		cfg.TimeSyncerInterval)
	if err != nil {
		return nil, errors.Wrap(err, "new time syncer failed")
	}
	return &global{
		proxyPool:    proxyPool,
		dnsClient:    dnsClient,
		timeSyncer:   timeSyncer,
		keyDir:       cfg.KeyDir,
		object:       make(map[uint32]interface{}),
		waitLoadKeys: make(chan struct{}, 1),
	}, nil
}

func (global *global) StartTimeSyncer() error {
	return global.timeSyncer.Start()
}

func (global *global) Now() time.Time {
	return global.timeSyncer.Now().Local()
}

func (global *global) WaitLoadKeys() {
	<-global.waitLoadKeys
}

func (global *global) AddProxyClient(tag string, client *proxy.Client) error {
	return global.proxyPool.Add(tag, client)
}

func (global *global) AddDNSSever(tag string, server *dns.Server) error {
	return global.dnsClient.Add(tag, server)
}

func (global *global) AddTimeSyncerConfig(tag string, config *timesync.Config) error {
	return global.timeSyncer.Add(tag, config)
}

func (global *global) LoadKeys(password string) error {
	global.objectRWM.Lock()
	defer global.objectRWM.Unlock()
	if global.object[ed25519PrivateKey] != nil {
		return errors.New("already load keys")
	}
	keys, err := loadCtrlKeys(global.keyDir+"/ctrl.key", password)
	if err != nil {
		return errors.WithStack(err)
	}
	// ed25519
	pri, _ := ed25519.ImportPrivateKey(keys[0])
	global.object[ed25519PrivateKey] = pri
	pub, _ := ed25519.ImportPublicKey(pri[32:])
	global.object[ed25519PublicKey] = pub
	// curve25519
	p, err := curve25519.ScalarBaseMult(pri[:32])
	if err != nil {
		return errors.WithStack(err)
	}
	global.object[curve25519PublicKey] = p
	// aes
	cbc, _ := aes.NewCBC(keys[1], keys[2])
	global.object[aesCrypto] = cbc
	atomic.StoreInt32(&global.isLoadKeys, 1)
	close(global.waitLoadKeys)
	return nil
}

func (global *global) IsLoadKeys() bool {
	return atomic.LoadInt32(&global.isLoadKeys) != 0
}

// verify controller(handshake) and sign message
func (global *global) Sign(message []byte) []byte {
	global.objectRWM.RLock()
	p := global.object[ed25519PrivateKey].(ed25519.PrivateKey)
	global.objectRWM.RUnlock()
	return ed25519.Sign(p, message)
}

// verify node certificate
func (global *global) Verify(message, signature []byte) bool {
	global.objectRWM.RLock()
	p := global.object[ed25519PublicKey].(ed25519.PublicKey)
	global.objectRWM.RUnlock()
	return ed25519.Verify(p, message, signature)
}

func (global *global) Curve25519PublicKey() []byte {
	global.objectRWM.RLock()
	p := global.object[curve25519PublicKey].([]byte)
	global.objectRWM.RUnlock()
	return p
}

func (global *global) KeyExchange(publicKey []byte) ([]byte, error) {
	global.objectRWM.RLock()
	pri := global.object[ed25519PrivateKey].(ed25519.PrivateKey)
	global.objectRWM.RUnlock()
	return curve25519.ScalarMult(pri, publicKey)
}

func (global *global) Destroy() {
	global.timeSyncer.Stop()
}
