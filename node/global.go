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
	certs      []*x509.Certificate
	certASN1s  [][]byte
	proxyPool  *proxy.Pool
	dnsClient  *dns.Client
	timeSyncer *timesync.Syncer

	object    map[uint32]interface{}
	objectRWM sync.RWMutex

	spmCount int // secPaddingMemory execute time
	wg       sync.WaitGroup
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
	timeSyncer.SetSleep(cfg.TimeSyncFixed, cfg.TimeSyncRandom)

	g := global{
		certs:      certs,
		certASN1s:  certASN1s,
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

	objStartupTime // global.configure() time
	objNodeGUID    // identification
	objDBAESCrypto // encrypt self data(database)
	objCertificate // for server.handshake
	objPrivateKey  // for sign message
	objPublicKey   // for role verify message
	objKeyExPub    // for key exchange
)

// <security>
func (global *global) secPaddingMemory() {
	rand := random.New(0)
	memory := security.NewMemory()
	security.PaddingMemory()
	defer security.FlushMemory()
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
	}
	delete(global.object, objCertificate)
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
	global.object[objKeyExPub] = pub
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
	// controller broadcast key
	global.secPaddingMemory()
	aesKey = cfg.CTRL.BroadcastKey
	l := len(aesKey)
	if l != aes.Key256Bit+aes.IVSize {
		return errors.New("invalid controller aes key size")
	}
	key := aesKey[:aes.Key256Bit]
	iv := aesKey[aes.Key256Bit:]
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
	return global.proxyPool.Get(tag)
}

// ProxyClients is used to get all proxy client in proxy pool
func (global *global) ProxyClients() map[string]*proxy.Client {
	return global.proxyPool.Clients()
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

// StartupTime is used to get Node startup time
func (global *global) StartupTime() time.Time {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	return global.object[objStartupTime].(time.Time)
}

// GUID is used to get Node GUID
func (global *global) GUID() []byte {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	return global.object[objNodeGUID].([]byte)
}

// Certificate is used to get Node certificate
func (global *global) Certificate() []byte {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	c := global.object[objCertificate]
	if c != nil {
		return c.([]byte)
	} else {
		return nil
	}
}

// SetCertificate is used to set Node certificate
// it can be set once
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

// PublicKey is used to get node public key
func (global *global) PublicKey() ed25519.PublicKey {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	return global.object[objPublicKey].(ed25519.PublicKey)
}

// KeyExchangePub is used to get node key exchange public key
func (global *global) KeyExchangePub() []byte {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	return global.object[objKeyExPub].([]byte)
}

// Sign is used to sign node message
func (global *global) Sign(message []byte) []byte {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	k := global.object[objPrivateKey]
	return ed25519.Sign(k.(ed25519.PrivateKey), message)
}

// Encrypt is used to encrypt session data
func (global *global) Encrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	return global.object[objCtrlSessionKey].(*aes.CBC).Encrypt(data)
}

// Decrypt is used to decrypt session data
func (global *global) Decrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	return global.object[objCtrlSessionKey].(*aes.CBC).Decrypt(data)
}

// CtrlVerify is used to verify controller message
func (global *global) CtrlVerify(message, signature []byte) bool {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	p := global.object[objCtrlPublicKey]
	return ed25519.Verify(p.(ed25519.PublicKey), message, signature)
}

// CtrlDecrypt is used to decrypt controller broadcast message
func (global *global) CtrlDecrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	return global.object[objCtrlAESCrypto].(*aes.CBC).Decrypt(data)
}

// DBEncrypt is used to encrypt database data
func (global *global) DBEncrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	return global.object[objDBAESCrypto].(*aes.CBC).Encrypt(data)
}

// DBDecrypt is used to decrypt database data
func (global *global) DBDecrypt(data []byte) ([]byte, error) {
	global.objectRWM.RLock()
	defer global.objectRWM.RUnlock()
	return global.object[objDBAESCrypto].(*aes.CBC).Decrypt(data)
}

// Close is used to close global
func (global *global) Close() {
	global.timeSyncer.Stop()
}
