package controller

import (
	"io/ioutil"
	"sync"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
)

type global struct {
	proxy      *proxyclient.PROXY
	dns        *dnsclient.DNS
	timesync   *timesync.TIMESYNC
	object     map[uint32]interface{}
	object_rwm sync.RWMutex
	load_keys  chan struct{}
}

func new_global(ctx *CTRL, c *Config) (*global, error) {
	// load proxy clients
	pcs, err := ctx.Select_Proxy_Client()
	if err != nil {
		return nil, errors.Wrap(err, "load proxy clients failed")
	}
	l := len(pcs)
	tpcs := make(map[string]*proxyclient.Client, l)
	for i := 0; i < l; i++ {
		tpcs[pcs[i].Tag] = &proxyclient.Client{
			Mode:   pcs[i].Mode,
			Config: pcs[i].Config,
		}
	}
	p, err := proxyclient.New(tpcs)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// load dns clients
	// load builtin
	tdcs := make(map[string]*dnsclient.Client)
	b, err := ioutil.ReadFile("builtin/dnsclient.toml")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	err = toml.Unmarshal(b, &tdcs)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	dcs, err := ctx.Select_DNS_Client()
	if err != nil {
		return nil, errors.Wrap(err, "load dns clients failed")
	}
	// database records will cover builtin
	for i := 0; i < len(dcs); i++ {
		tdcs[dcs[i].Tag] = &dnsclient.Client{
			Method:  dcs[i].Method,
			Address: dcs[i].Address,
		}
	}
	d, err := dnsclient.New(p, tdcs, c.DNS_Cache_Deadline)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// load timesync
	// load builtin
	tts := make(map[string]*timesync.Client)
	b, err = ioutil.ReadFile("builtin/timesync.toml")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	err = toml.Unmarshal(b, &tts)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	ts, err := ctx.Select_Timesync()
	if err != nil {
		return nil, errors.Wrap(err, "load timesync clients failed")
	}
	for i := 0; i < len(ts); i++ {
		c := &timesync.Client{}
		err := toml.Unmarshal([]byte(ts[i].Config), c)
		if err != nil {
			return nil, errors.Wrapf(err, "load timesync: %s failed", ts[i].Tag)
		}
		tts[ts[i].Tag] = c
	}
	t, err := timesync.New(p, d, ctx, tts, c.Timesync_Interval)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	g := &global{
		proxy:     p,
		dns:       d,
		timesync:  t,
		object:    make(map[uint32]interface{}),
		load_keys: make(chan struct{}, 1),
	}
	return g, nil
}

// about internal

func (this *global) Start_Timesync() error {
	return this.timesync.Start()
}

func (this *global) Now() time.Time {
	return this.timesync.Now().Local()
}

func (this *global) Load_Keys(password string) error {
	this.object_rwm.RLock()
	o := this.object[ed25519_privatekey]
	this.object_rwm.RUnlock()
	if o != nil {
		return errors.New("already load keys")
	}
	keys, err := Load_CTRL_Keys(Key_Path, password)
	if err != nil {
		return errors.WithStack(err)
	}
	// ed25519
	pri, _ := ed25519.Import_PrivateKey(keys[0])
	this.object_rwm.Lock()
	this.object[ed25519_privatekey] = pri
	this.object_rwm.Unlock()
	// aes
	cryptor, _ := aes.New_CBC_Cryptor(keys[1], keys[2])
	this.object_rwm.Lock()
	this.object[aes_cryptor] = cryptor
	this.object_rwm.Unlock()
	close(this.load_keys)
	return nil
}

func (this *global) Wait_Load_Keys() {
	<-this.load_keys
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
