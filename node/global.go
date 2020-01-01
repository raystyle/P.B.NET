package node

import (
	"crypto/x509"
	"encoding/pem"
	"sync"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/context"

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
	// about certificate
	certs     []*x509.Certificate
	certASN1s [][]byte

	ProxyPool  *proxy.Pool
	DNSClient  *dns.Client
	TimeSyncer *timesync.Syncer

	objects    map[uint32]interface{}
	objectsRWM sync.RWMutex
	rand       *random.Rand
	spmCount   int // paddingMemory execute time
	wg         sync.WaitGroup

	// TODO client test
}

func newGlobal(logger logger.Logger, config *Config) (*global, error) {
	cfg := config.Global

	memory := security.NewMemory()
	defer memory.Flush()

	// load certificates
	var (
		certs     []*x509.Certificate
		certASN1s [][]byte
	)
	for i := 0; i < len(cfg.Certificates); i++ {
		memory.Padding()
		cert, err := x509.ParseCertificate(cfg.Certificates[i])
		if err != nil {
			return nil, errors.WithStack(err)
		}
		certs = append(certs, cert)
		certASN1s = append(certASN1s, cfg.Certificates[i])
	}

	// proxy client
	proxyPool := proxy.NewPool()
	for i := 0; i < len(cfg.ProxyClients); i++ {
		memory.Padding()
		err := proxyPool.Add(cfg.ProxyClients[i])
		if err != nil {
			return nil, err
		}
	}

	// DNS client
	dnsClient := dns.NewClient(proxyPool)
	for tag, server := range cfg.DNSServers {
		memory.Padding()
		err := dnsClient.Add(tag, server)
		if err != nil {
			return nil, err
		}
	}
	err := dnsClient.SetCacheExpireTime(cfg.DNSCacheExpire)
	if err != nil {
		return nil, err
	}

	// time syncer
	timeSyncer := timesync.New(proxyPool, dnsClient, logger)
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
	timeSyncer.SetSleep(cfg.TimeSyncSleepFixed, cfg.TimeSyncSleepRandom)

	g := global{
		certs:      certs,
		certASN1s:  certASN1s,
		ProxyPool:  proxyPool,
		DNSClient:  dnsClient,
		TimeSyncer: timeSyncer,
		rand:       random.New(),
	}
	err = g.configure(config)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// <warning> must < 1048576
const (
	objCtrlPublicKey    uint32 = iota // verify controller role & message
	objCtrlBroadcastKey               // decrypt controller broadcast message
	objCtrlSessionKey                 // after key exchange (aes crypto)

	objStartupTime // global.configure() time
	objNodeGUID    // identification

	objCertificate // for server.handshake need protect
	objPrivateKey  // for sign message
	objPublicKey   // for role verify message
	objKeyExPub    // for key exchange
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
	delete(global.objects, objCertificate)
	// -----------------generate internal objects-----------------
	// set startup time
	global.objects[objStartupTime] = time.Now()
	// generate guid and select one
	global.paddingMemory()
	g := guid.New(64, nil)
	defer g.Close()
	var guidPool [1024][]byte
	for i := 0; i < len(guidPool); i++ {
		guidPool[i] = g.Get()
	}
	guidSelected := make([]byte, guid.Size)
	copy(guidSelected, guidPool[global.rand.Int(1024)])
	global.objects[objNodeGUID] = guidSelected
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
	keyExPub, err := curve25519.ScalarBaseMult(pri[:32])
	if err != nil {
		panic(err)
	}
	global.objects[objKeyExPub] = keyExPub
	global.paddingMemory()
	global.objects[objPrivateKey] = security.NewBytes(pri)
	security.CoverBytes(pri)
	// -----------------load controller configs-----------------
	// controller public key
	global.paddingMemory()
	publicKey, err := ed25519.ImportPublicKey(cfg.CTRL.PublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	global.objects[objCtrlPublicKey] = publicKey
	// controller broadcast key
	global.paddingMemory()
	if len(cfg.CTRL.BroadcastKey) != aes.Key256Bit+aes.IVSize {
		return errors.New("invalid controller aes key size")
	}
	aesKey := cfg.CTRL.BroadcastKey[:aes.Key256Bit]
	aesIV := cfg.CTRL.BroadcastKey[aes.Key256Bit:]
	cbc, err := aes.NewCBC(aesKey, aesIV)
	if err != nil {
		return errors.WithStack(err)
	}
	security.CoverBytes(aesKey)
	security.CoverBytes(aesIV)
	global.objects[objCtrlBroadcastKey] = cbc
	// calculate session key and set aes crypto
	global.paddingMemory()
	sb := global.objects[objPrivateKey].(*security.Bytes)
	b := sb.Get()
	defer sb.Put(b)
	sessionKey, err := curve25519.ScalarMult(b[:32], cfg.CTRL.ExPublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	cbc, err = aes.NewCBC(sessionKey, sessionKey[:aes.IVSize])
	if err != nil {
		return errors.WithStack(err)
	}
	global.objects[objCtrlSessionKey] = cbc
	return nil
}

const spmCount = 9 // global.paddingMemory() execute count

// OK is used to check debug
func (global *global) OK() bool {
	return global.spmCount == spmCount
}

// Certificates is used to get all certificates
func (global *global) Certificates() []*x509.Certificate {
	return global.certs
}

// CertificatePEMs is used to get all certificates that encode to PEM
func (global *global) CertificatePEMs() []string {
	var certPEMs []string
	block := new(pem.Block)
	block.Type = "CERTIFICATE"
	for i := 0; i < len(global.certASN1s); i++ {
		block.Bytes = global.certASN1s[i]
		certPEMs = append(certPEMs, string(pem.EncodeToMemory(block)))
	}
	return certPEMs
}

// GetProxyClient is used to get proxy client from proxy pool
func (global *global) GetProxyClient(tag string) (*proxy.Client, error) {
	return global.ProxyPool.Get(tag)
}

// ProxyClients is used to get all proxy client in proxy pool
func (global *global) ProxyClients() map[string]*proxy.Client {
	return global.ProxyPool.Clients()
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

// TimeSyncerClients is used to get all time syncer clients in time syncer
func (global *global) TimeSyncerClients() map[string]*timesync.Client {
	return global.TimeSyncer.Clients()
}

// StartTimeSyncer is used to start time syncer
func (global *global) StartTimeSyncer() error {
	return global.TimeSyncer.Start()
}

// StartTimeSyncerWalker is used to start time syncer add loop
func (global *global) StartTimeSyncerWalker() {
	global.TimeSyncer.StartWalker()
}

// Now is used to get current time
func (global *global) Now() time.Time {
	return global.TimeSyncer.Now().Local()
}

// StartupTime is used to get Node startup time
func (global *global) StartupTime() time.Time {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objStartupTime].(time.Time)
}

// GUID is used to get Node GUID
func (global *global) GUID() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objNodeGUID].([]byte)
}

// Certificate is used to get Node certificate
func (global *global) GetCertificate() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	c := global.objects[objCertificate]
	if c != nil {
		return c.([]byte)
	}
	return nil
}

// SetCertificate is used to set Node certificate
// it can be set once
func (global *global) SetCertificate(cert []byte) error {
	global.objectsRWM.Lock()
	defer global.objectsRWM.Unlock()
	if _, ok := global.objects[objCertificate]; !ok {
		c := make([]byte, len(cert))
		copy(c, cert)
		global.objects[objCertificate] = c
		return nil
	}
	return errors.New("certificate has been set")
}

// Sign is used to sign node message
func (global *global) Sign(message []byte) []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	pri := global.objects[objPrivateKey].(*security.Bytes)
	b := pri.Get()
	defer pri.Put(b)
	return ed25519.Sign(b, message)
}

// PublicKey is used to get node public key
func (global *global) PublicKey() ed25519.PublicKey {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objPublicKey].(ed25519.PublicKey)
}

// KeyExchangePub is used to get node key exchange public key
func (global *global) KeyExchangePub() []byte {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	return global.objects[objKeyExPub].([]byte)
}

// Encrypt is used to encrypt session data
func (global *global) Encrypt(data []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	cbc := global.objects[objCtrlSessionKey].(*aes.CBC)
	return cbc.Encrypt(data)
}

// Decrypt is used to decrypt session data
func (global *global) Decrypt(data []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	cbc := global.objects[objCtrlSessionKey].(*aes.CBC)
	return cbc.Decrypt(data)
}

// CtrlVerify is used to verify controller message
func (global *global) CtrlVerify(message, signature []byte) bool {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	p := global.objects[objCtrlPublicKey]
	return ed25519.Verify(p.(ed25519.PublicKey), message, signature)
}

// CtrlDecrypt is used to decrypt controller broadcast message
func (global *global) CtrlDecrypt(data []byte) ([]byte, error) {
	global.objectsRWM.RLock()
	defer global.objectsRWM.RUnlock()
	cbc := global.objects[objCtrlBroadcastKey].(*aes.CBC)
	return cbc.Decrypt(data)
}

// Close is used to close global
func (global *global) Close() {
	global.TimeSyncer.Stop()
}
