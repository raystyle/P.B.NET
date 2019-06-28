package controller

import (
	"sync"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"

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
	dcs, err := ctx.Select_DNS_Client()
	if err != nil {
		return nil, errors.Wrap(err, "load dns clients failed")
	}
	l = len(dcs)
	tdcs := make(map[string]*dnsclient.Client, l)
	for i := 0; i < l; i++ {
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
	ts, err := ctx.Select_Timesync()
	if err != nil {
		return nil, errors.Wrap(err, "load timesync clients failed")
	}
	l = len(ts)
	tts := make(map[string]*timesync.Client, l)
	for i := 0; i < l; i++ {
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
		proxy:    p,
		dns:      d,
		timesync: t,
		object:   make(map[uint32]interface{}),
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

// controller and sign message
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
