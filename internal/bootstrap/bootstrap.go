package bootstrap

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/aes"
	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/security"
)

// supported modes
const (
	ModeHTTP   = "http"
	ModeDNS    = "dns"
	ModeDirect = "direct"
)

// Listener is the bootstrap node listener
// Node or Beacon register will use bootstrap to resolve node listeners
// you can reference internal/xnet/net.go
type Listener struct {
	Mode    string `toml:"mode"    msgpack:"a"`
	Network string `toml:"network" msgpack:"b"`
	Address string `toml:"address" msgpack:"c"`

	// self encrypted
	cbc *aes.CBC
	enc []byte
}

// NewListener is used to create a self encrypted listener, all parameter will be covered.
func NewListener(mode, network, address string) *Listener {
	defer func() {
		security.CoverString(&mode)
		security.CoverString(&network)
		security.CoverString(&address)
	}()
	memory := security.NewMemory()
	defer memory.Flush()
	rand := random.New()
	memory.Padding()
	key := rand.Bytes(aes.Key256Bit)
	iv := rand.Bytes(aes.IVSize)
	cbc, _ := aes.NewCBC(key, iv)
	security.CoverBytes(key)
	security.CoverBytes(iv)
	memory.Padding()
	listener := Listener{
		Mode:    mode,
		Network: network,
		Address: address,
	}
	defer func() {
		security.CoverString(&listener.Mode)
		security.CoverString(&listener.Network)
		security.CoverString(&listener.Address)
	}()
	listenerData, _ := msgpack.Marshal(listener)
	security.CoverString(&listener.Mode)
	security.CoverString(&listener.Network)
	security.CoverString(&listener.Address)
	defer security.CoverBytes(listenerData)
	memory.Padding()
	enc, _ := cbc.Encrypt(listenerData)
	listener.enc = enc
	listener.cbc = cbc
	return &listener
}

// Decrypt is used to decrypt self encrypt data, it will create a new Listener
// must call Encrypt to encrypt data after use the created new Listener.
func (l *Listener) Decrypt() *Listener {
	dec, err := l.cbc.Decrypt(l.enc)
	if err != nil {
		panic(err)
	}
	defer security.CoverBytes(dec)
	tempListener := new(Listener)
	err = msgpack.Unmarshal(dec, tempListener)
	if err != nil {
		panic(err)
	}
	return tempListener
}

// Destroy is used to clean structure field.
func (l *Listener) Destroy() {
	security.CoverString(&l.Mode)
	security.CoverString(&l.Network)
	security.CoverString(&l.Address)
}

// String is used to return information about listener.
// tls (tcp 127.0.0.1:443)
func (l *Listener) String() string {
	return fmt.Sprintf("%s (%s %s)", l.Mode, l.Network, l.Address)
}

// encryptListeners is used to encrypt listeners after Bootstrap.Resolve()
func encryptListeners(listeners []*Listener) []*Listener {
	l := len(listeners)
	newListeners := make([]*Listener, l)
	for i := 0; i < l; i++ {
		newListeners[i] = NewListener(
			listeners[i].Mode,
			listeners[i].Network,
			listeners[i].Address,
		)
		listeners[i].Destroy()
	}
	return newListeners
}

// Bootstrap is used to resolve bootstrap node listeners
type Bootstrap interface {
	// Validate is used to check bootstrap config correct
	Validate() error

	// Marshal is used to marshal bootstrap to []byte
	Marshal() ([]byte, error)

	// Unmarshal is used to unmarshal []byte to bootstrap
	Unmarshal([]byte) error

	// Resolve is used to resolve bootstrap node listeners
	Resolve() ([]*Listener, error)
}

// Load is used to create a bootstrap from configuration.
func Load(
	ctx context.Context,
	mode string,
	config []byte,
	pool *proxy.Pool,
	client *dns.Client,
) (Bootstrap, error) {
	var bootstrap Bootstrap
	switch mode {
	case ModeHTTP:
		bootstrap = NewHTTP(ctx, pool, client)
	case ModeDNS:
		bootstrap = NewDNS(ctx, client)
	case ModeDirect:
		bootstrap = NewDirect()
	default:
		return nil, errors.Errorf("unknown mode: %s", mode)
	}
	err := bootstrap.Unmarshal(config)
	if err != nil {
		return nil, err
	}
	return bootstrap, nil
}
