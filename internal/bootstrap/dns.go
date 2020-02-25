package bootstrap

import (
	"context"
	"net"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/dns"
	"project/internal/patch/msgpack"
	"project/internal/patch/toml"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xnet"
	"project/internal/xnet/xnetutil"
)

// DNS is used to resolve bootstrap node listeners from DNS resolve result.
type DNS struct {
	ctx       context.Context
	dnsClient *dns.Client

	Host    string      `toml:"host"`    // domain name
	Mode    string      `toml:"mode"`    // listener mode (see xnet)
	Network string      `toml:"network"` // listener network
	Port    string      `toml:"port"`    // listener port
	Options dns.Options `toml:"options"` // dns options

	// self encrypt all options
	cbc *aes.CBC
	enc []byte
}

// NewDNS is used to create a DNS mode bootstrap.
func NewDNS(ctx context.Context, dnsClient *dns.Client) *DNS {
	return &DNS{
		ctx:       ctx,
		dnsClient: dnsClient,
	}
}

// Validate is used to check DNS config correct.
func (d *DNS) Validate() error {
	if d.Host == "" {
		return errors.New("empty host")
	}
	if !dns.IsDomainName(d.Host) {
		return errors.Errorf("invalid domain name: %s", d.Host)
	}
	err := xnet.CheckModeNetwork(d.Mode, d.Network)
	if err != nil {
		return errors.WithStack(err)
	}
	err = xnetutil.CheckPortString(d.Port)
	return errors.WithStack(err)
}

// Marshal is used to marshal DNS to []byte.
func (d *DNS) Marshal() ([]byte, error) {
	err := d.Validate()
	if err != nil {
		return nil, err
	}
	return toml.Marshal(d)
}

// Unmarshal is used to unmarshal []byte to DNS.
// store encrypted data to d.enc
func (d *DNS) Unmarshal(config []byte) error {
	tempDNS := &DNS{}
	err := toml.Unmarshal(config, tempDNS)
	if err != nil {
		return err
	}
	err = tempDNS.Validate()
	if err != nil {
		return err
	}
	// encrypt all options
	memory := security.NewMemory()
	defer memory.Flush()
	rand := random.New()
	key := rand.Bytes(aes.Key256Bit)
	iv := rand.Bytes(aes.IVSize)
	d.cbc, _ = aes.NewCBC(key, iv)
	security.CoverBytes(key)
	security.CoverBytes(iv)
	memory.Padding()
	listenerData, _ := msgpack.Marshal(tempDNS)
	defer security.CoverBytes(listenerData)
	security.CoverString(&tempDNS.Host)
	memory.Padding()
	d.enc, err = d.cbc.Encrypt(listenerData)
	return err
}

// Resolve is used to get bootstrap node listeners.
func (d *DNS) Resolve() ([]*Listener, error) {
	// decrypt all options
	memory := security.NewMemory()
	defer memory.Flush()
	dec, err := d.cbc.Decrypt(d.enc)
	defer security.CoverBytes(dec)
	if err != nil {
		panic(err)
	}
	tempDNS := &DNS{}
	err = msgpack.Unmarshal(dec, tempDNS)
	if err != nil {
		panic(err)
	}
	security.CoverBytes(dec)
	memory.Padding()
	// resolve dns
	domain := tempDNS.Host
	defer func() {
		security.CoverString(&tempDNS.Host)
		security.CoverString(&domain)
	}()
	result, err := d.dnsClient.ResolveContext(d.ctx, domain, &tempDNS.Options)
	if err != nil {
		return nil, err
	}
	l := len(result)
	listeners := make([]*Listener, l)
	for i := 0; i < l; i++ {
		listeners[i] = &Listener{
			Mode:    tempDNS.Mode,
			Network: tempDNS.Network,
		}
		listeners[i].Address = net.JoinHostPort(result[i], tempDNS.Port)
		security.CoverString(&result[i])
	}
	return EncryptListeners(listeners), nil
}
