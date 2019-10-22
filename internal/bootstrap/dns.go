package bootstrap

import (
	"errors"

	"github.com/pelletier/go-toml"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/aes"
	"project/internal/dns"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xnet"
)

var (
	ErrEmptyDomainName = errors.New("domain name is empty")
)

type DNS struct {
	DomainName      string      `toml:"domain_name"`
	ListenerMode    xnet.Mode   `toml:"listener_mode"`
	ListenerNetwork string      `toml:"listener_network"`
	ListenerPort    string      `toml:"listener_port"`
	Options         dns.Options `toml:"dns_options"`
	// runtime
	dnsClient DNSClient
	// self store all encrypted options by msgpack
	optsEnc []byte
	cbc     *aes.CBC
}

// input ctx for resolve
func NewDNS(client DNSClient) *DNS {
	return &DNS{
		dnsClient: client,
	}
}

func (d *DNS) Validate() error {
	if d.DomainName == "" {
		return ErrEmptyDomainName
	}
	err := xnet.CheckModeNetwork(d.ListenerMode, d.ListenerNetwork)
	if err != nil {
		return err
	}
	err = xnet.CheckPortString(d.ListenerPort)
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
	tempDNS := &DNS{}
	err = msgpack.Unmarshal(b, tempDNS)
	if err != nil {
		panic(&fPanic{Mode: ModeDNS, Err: err})
	}
	security.FlushBytes(b)
	memory.Padding()
	// resolve dns
	ipList, err := d.dnsClient.Resolve(tempDNS.DomainName, &tempDNS.Options)
	if err != nil {
		return nil, err
	}
	tempDNS.DomainName = "" // <security>
	l := len(ipList)
	nodes := make([]*Node, l)
	for i := 0; i < l; i++ {
		nodes[i] = &Node{
			Mode:    tempDNS.ListenerMode,
			Network: tempDNS.ListenerNetwork,
		}
	}
	switch tempDNS.Options.Type {
	case "", dns.IPv4:
		for i := 0; i < l; i++ {
			nodes[i].Address = ipList[i] + ":" + tempDNS.ListenerPort
		}
	case dns.IPv6:
		for i := 0; i < l; i++ {
			nodes[i].Address = "[" + ipList[i] + "]:" + tempDNS.ListenerPort
		}
	default:
		panic(&fPanic{Mode: ModeDNS, Err: err})
	}
	tempDNS = nil
	for i := 0; i < l; i++ { // <security>
		ipList[i] = ""
	}
	return nodes, nil
}
