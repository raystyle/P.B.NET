package bootstrap

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/dns"
	"project/internal/patch/msgpack"
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

// Bootstrap is used to resolve bootstrap Node listeners.
type Bootstrap interface {
	// Validate is used to check bootstrap configuration is correct
	Validate() error

	// Marshal is used to marshal bootstrap to []byte
	Marshal() ([]byte, error)

	// Unmarshal is used to unmarshal []byte to bootstrap
	Unmarshal([]byte) error

	// Resolve is used to resolve bootstrap Node listeners
	Resolve() ([]*Listener, error)
}

// Load is used to load a bootstrap from configuration.
func Load(
	ctx context.Context,
	mode string,
	config []byte,
	certPool *cert.Pool,
	proxyPool *proxy.Pool,
	dnsClient *dns.Client,
) (Bootstrap, error) {
	var bootstrap Bootstrap
	switch mode {
	case ModeHTTP:
		bootstrap = NewHTTP(ctx, certPool, proxyPool, dnsClient)
	case ModeDNS:
		bootstrap = NewDNS(ctx, dnsClient)
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

// Listener is the bootstrap Node listener.
// Node or Beacon register will use bootstrap to resolve Node listeners,
// you can reference internal/xnet/net.go.
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

// Equal is used to compare two listeners, must be encrypted.
func (l *Listener) Equal(listener *Listener) bool {
	tl1 := l.Decrypt()
	defer tl1.Destroy()
	tl2 := listener.Decrypt()
	defer tl2.Destroy()
	return tl1.Mode == tl2.Mode &&
		tl1.Network == tl2.Network &&
		tl1.Address == tl2.Address
}

// String is used to return information about listener.
// tls (tcp 127.0.0.1:443)
func (l *Listener) String() string {
	return fmt.Sprintf("%s (%s %s)", l.Mode, l.Network, l.Address)
}

// EncryptListeners is used to encrypt raw listeners.
func EncryptListeners(listeners []*Listener) []*Listener {
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
