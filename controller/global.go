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
	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
	"project/internal/logger"
)

type global struct {
	proxy          *proxyclient.PROXY
	dns            *dnsclient.DNS
	timesync       *timesync.TIMESYNC
	key_dir        string
	object         map[uint32]interface{}
	object_rwm     sync.RWMutex
	is_load_keys   int32
	wait_load_keys chan struct{}
}

func new_global(lg logger.Logger, c *Config) (*global, error) {
	proxy, _ := proxyclient.New(nil)
	// load builtin dns clients
	tdcs := make(map[string]*dnsclient.Client)
	b, err := ioutil.ReadFile(c.Builtin_Dir + "/dnsclient.toml")
	if err != nil {
		return nil, errors.Wrap(err, "load builtin dns clients failed")
	}
	err = toml.Unmarshal(b, &tdcs)
	if err != nil {
		return nil, errors.Wrap(err, "load builtin dns clients failed")
	}
	// add tag
	for tag, client := range tdcs {
		tdcs["builtin_"+tag] = client
		delete(tdcs, tag)
	}
	dns, err := dnsclient.New(proxy, tdcs, c.DNS_Cache_Deadline)
	if err != nil {
		return nil, errors.Wrap(err, "new dns failed")
	}
	// load builtin timesync client
	tts := make(map[string]*timesync.Client)
	b, err = ioutil.ReadFile(c.Builtin_Dir + "/timesync.toml")
	if err != nil {
		return nil, errors.Wrap(err, "load builtin timesync clients failed")
	}
	err = toml.Unmarshal(b, &tts)
	if err != nil {
		return nil, errors.Wrap(err, "load builtin timesync clients failed")
	}
	// add tag
	for tag, client := range tts {
		tts["builtin_"+tag] = client
		delete(tdcs, tag)
	}
	tsync, err := timesync.New(proxy, dns, lg, tts, c.Timesync_Interval)
	if err != nil {
		return nil, errors.Wrap(err, "new timesync failed")
	}
	g := &global{
		proxy:          proxy,
		dns:            dns,
		timesync:       tsync,
		key_dir:        c.Key_Dir,
		object:         make(map[uint32]interface{}),
		wait_load_keys: make(chan struct{}, 1),
	}
	return g, nil
}

func (this *global) Start_Timesync() error {
	return this.timesync.Start()
}

func (this *global) Now() time.Time {
	return this.timesync.Now().Local()
}

func (this *global) Wait_Load_Keys() {
	<-this.wait_load_keys
}

func (this *global) Add_Proxy_Client(tag string, c *proxyclient.Client) error {
	return this.proxy.Add(tag, c)
}

func (this *global) Add_DNS_Client(tag string, c *dnsclient.Client) error {
	return this.dns.Add(tag, c)
}

func (this *global) Add_Timesync_Client(tag string, c *timesync.Client) error {
	return this.timesync.Add(tag, c)
}

func (this *global) Load_Keys(password string) error {
	this.object_rwm.Lock()
	defer this.object_rwm.Unlock()
	if this.object[ed25519_privatekey] != nil {
		return errors.New("already load keys")
	}
	keys, err := Load_CTRL_Keys(this.key_dir+"/ctrl.key", password)
	if err != nil {
		return errors.WithStack(err)
	}
	// ed25519
	pri, _ := ed25519.Import_PrivateKey(keys[0])
	this.object[ed25519_privatekey] = pri
	pub, _ := ed25519.Import_PublicKey(pri[32:])
	this.object[ed25519_publickey] = pub
	// curve25519
	p, err := curve25519.Scalar_Base_Mult(pri)
	if err != nil {
		return errors.WithStack(err)
	}
	this.object[curve25519_publickey] = p
	// aes
	cryptor, _ := aes.New_CBC_Cryptor(keys[1], keys[2])
	this.object[aes_cryptor] = cryptor
	atomic.StoreInt32(&this.is_load_keys, 1)
	close(this.wait_load_keys)
	return nil
}

func (this *global) Is_Load_Keys() bool {
	return atomic.LoadInt32(&this.is_load_keys) != 0
}

// verify controller(handshake) and sign message
func (this *global) Sign(message []byte) []byte {
	this.object_rwm.RLock()
	p := this.object[ed25519_privatekey].(ed25519.PrivateKey)
	this.object_rwm.RUnlock()
	return ed25519.Sign(p, message)
}

// verify node certificate
func (this *global) Verify(message, signature []byte) bool {
	this.object_rwm.RLock()
	p := this.object[ed25519_publickey].(ed25519.PublicKey)
	this.object_rwm.RUnlock()
	return ed25519.Verify(p, message, signature)
}

func (this *global) Curve25519_Publickey() []byte {
	this.object_rwm.RLock()
	p := this.object[curve25519_publickey].([]byte)
	this.object_rwm.RUnlock()
	return p
}

func (this *global) Key_Exchange(publickey []byte) ([]byte, error) {
	this.object_rwm.RLock()
	pri := this.object[ed25519_privatekey].(ed25519.PrivateKey)
	this.object_rwm.RUnlock()
	return curve25519.Scalar_Mult(pri, publickey)
}

func (this *global) Close() {
	this.timesync.Stop()
}
