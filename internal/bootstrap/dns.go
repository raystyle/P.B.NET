package bootstrap

import (
	"context"
	"net"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/aes"
	"project/internal/dns"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xnet"
	"project/internal/xnet/xnetutil"
)

// DNS is used to resolve bootstrap nodes from DNS resolve result
// network and port can't change
type DNS struct {
	Host    string      `toml:"host"`    // domain name
	Mode    string      `toml:"mode"`    // listener mode (see xnet)
	Network string      `toml:"network"` // listener network
	Port    string      `toml:"port"`    // listener port
	Options dns.Options `toml:"options"` // dns options

	// runtime
	ctx       context.Context
	dnsClient *dns.Client

	// self encrypt all options
	enc []byte
	cbc *aes.CBC
}

// NewDNS is used to create a DNS mode bootstrap
func NewDNS(ctx context.Context, client *dns.Client) *DNS {
	return &DNS{
		ctx:       ctx,
		dnsClient: client,
	}
}

// Validate is used to check DNS config correct
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

// Marshal is used to marshal DNS to []byte
func (d *DNS) Marshal() ([]byte, error) {
	err := d.Validate()
	if err != nil {
		return nil, err
	}
	return toml.Marshal(d)
}

// Unmarshal is used to unmarshal []byte to DNS
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
	b, _ := msgpack.Marshal(tempDNS)
	defer security.CoverBytes(b)
	security.CoverString(&tempDNS.Host)
	memory.Padding()
	d.enc, err = d.cbc.Encrypt(b)
	return err
}

// Resolve is used to get bootstrap nodes
func (d *DNS) Resolve() ([]*Node, error) {
	// decrypt all options
	memory := security.NewMemory()
	defer memory.Flush()
	b, err := d.cbc.Decrypt(d.enc)
	defer security.CoverBytes(b)
	if err != nil {
		panic(err)
	}
	tDNS := &DNS{}
	err = msgpack.Unmarshal(b, tDNS)
	if err != nil {
		panic(err)
	}
	security.CoverBytes(b)
	memory.Padding()
	// resolve dns
	dn := tDNS.Host
	dnsOpts := tDNS.Options
	defer func() {
		security.CoverString(&tDNS.Host)
		security.CoverString(&dn)
	}()
	result, err := d.dnsClient.ResolveWithContext(d.ctx, dn, &dnsOpts)
	if err != nil {
		return nil, err
	}
	l := len(result)
	nodes := make([]*Node, l)
	for i := 0; i < l; i++ {
		nodes[i] = &Node{
			Mode:    tDNS.Mode,
			Network: tDNS.Network,
		}
		nodes[i].Address = net.JoinHostPort(result[i], tDNS.Port)
		security.CoverString(&result[i])
	}
	return nodes, nil
}
