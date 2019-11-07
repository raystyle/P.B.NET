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

type DNS struct {
	DomainName      string      `toml:"domain_name"`
	ListenerMode    string      `toml:"listener_mode"`
	ListenerNetwork string      `toml:"listener_network"`
	ListenerPort    string      `toml:"listener_port"`
	Options         dns.Options `toml:"options"`
	// runtime
	ctx       context.Context
	dnsClient *dns.Client
	// self store all encrypted options by msgpack
	optsEnc []byte
	cbc     *aes.CBC
}

// input ctx for resolve
func NewDNS(ctx context.Context, client *dns.Client) *DNS {
	return &DNS{
		ctx:       ctx,
		dnsClient: client,
	}
}

func (d *DNS) Validate() error {
	if d.DomainName == "" {
		return errors.New("domain name is empty")
	}
	err := xnet.CheckModeNetwork(d.ListenerMode, d.ListenerNetwork)
	if err != nil {
		return err
	}
	err = xnetutil.CheckPortString(d.ListenerPort)
	if err != nil {
		return err
	}
	return nil
}

func (d *DNS) Marshal() ([]byte, error) {
	err := d.Validate()
	if err != nil {
		return nil, err
	}
	return toml.Marshal(d)
}

// Unmarshal
// store encrypted data to d.optsEnc
func (d *DNS) Unmarshal(data []byte) error {
	tempDNS := &DNS{}
	err := toml.Unmarshal(data, tempDNS)
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
	rand := random.New(0)
	key := rand.Bytes(aes.Key256Bit)
	iv := rand.Bytes(aes.IVSize)
	d.cbc, err = aes.NewCBC(key, iv)
	if err != nil {
		panic(&fPanic{Mode: ModeDNS, Err: err})
	}
	security.FlushBytes(key)
	security.FlushBytes(iv)
	memory.Padding()
	b, err := msgpack.Marshal(tempDNS)
	if err != nil {
		panic(&fPanic{Mode: ModeDNS, Err: err})
	}
	tempDNS = nil // <security>
	memory.Padding()
	d.optsEnc, err = d.cbc.Encrypt(b)
	if err != nil {
		panic(&fPanic{Mode: ModeDNS, Err: err})
	}
	security.FlushBytes(b)
	return nil
}

func (d *DNS) Resolve() ([]*Node, error) {
	// decrypt all options
	memory := security.NewMemory()
	defer memory.Flush()
	b, err := d.cbc.Decrypt(d.optsEnc)
	if err != nil {
		panic(&fPanic{Mode: ModeDNS, Err: err})
	}
	tDNS := &DNS{}
	err = msgpack.Unmarshal(b, tDNS)
	if err != nil {
		panic(&fPanic{Mode: ModeDNS, Err: err})
	}
	security.FlushBytes(b)
	memory.Padding()
	// resolve dns
	dn := tDNS.DomainName
	dnsOpts := tDNS.Options
	result, err := d.dnsClient.ResolveWithContext(d.ctx, dn, &dnsOpts)
	if err != nil {
		return nil, err
	}
	l := len(result)
	nodes := make([]*Node, l)
	for i := 0; i < l; i++ {
		nodes[i] = &Node{
			Mode:    tDNS.ListenerMode,
			Network: tDNS.ListenerNetwork,
		}
	}
	for i := 0; i < l; i++ {
		nodes[i].Address = net.JoinHostPort(result[i], tDNS.ListenerPort)
	}
	tDNS = nil
	for i := 0; i < l; i++ {
		result[i] = ""
	}
	return nodes, nil
}
