package bootstrap

import (
	"errors"
	"fmt"

	"github.com/pelletier/go-toml"
	"github.com/vmihailenco/msgpack"

	"project/internal/crypto/aes"
	"project/internal/dns"
	"project/internal/global/dnsclient"
	"project/internal/netx"
	"project/internal/random"
	"project/internal/security"
)

var (
	ERR_EMPTY_DOMAIN = errors.New("domain is empty")
)

type dns_panic struct {
	Err error
}

func (this *dns_panic) Error() string {
	return fmt.Sprintf("bootstrap dns internal error: %s", this.Err)
}

type DNS struct {
	Domain    string            `toml:"domain"`
	L_Mode    netx.Mode         `toml:"l_mode"`
	L_Network string            `toml:"l_network"`
	L_Port    string            `toml:"l_port"`
	Options   dnsclient.Options `toml:"dnsclient"`
	// runtime
	resolver dns_resolver
	// self encrypt all options
	opts_enc []byte
	cryptor  *aes.CBC_Cryptor
}

// input ctx for resolve
func New_DNS(d dns_resolver) *DNS {
	return &DNS{
		resolver: d,
	}
}

func (this *DNS) validate() error {
	if this.Domain == "" {
		return ERR_EMPTY_DOMAIN
	}
	err := netx.Inspect_Mode_Network(this.L_Mode, this.L_Network)
	if err != nil {
		return err
	}
	err = netx.Inspect_Port_string(this.L_Port)
	if err != nil {
		return err
	}
	return nil
}

func (this *DNS) Generate(_ []*Node) (string, error) {
	return "", nil
}

func (this *DNS) Marshal() ([]byte, error) {
	err := this.validate()
	if err != nil {
		return nil, err
	}
	return toml.Marshal(this)
}

func (this *DNS) Unmarshal(data []byte) error {
	d := &DNS{}
	err := toml.Unmarshal(data, d)
	if err != nil {
		return err
	}
	err = d.validate()
	if err != nil {
		return err
	}
	memory := security.New_Memory()
	defer memory.Flush()
	rand := random.New()
	key := rand.Bytes(32)
	iv := rand.Bytes(aes.IV_SIZE)
	this.cryptor, err = aes.New_CBC_Cryptor(key, iv)
	if err != nil {
		panic(&dns_panic{Err: err})
	}
	security.Flush_Bytes(key)
	security.Flush_Bytes(iv)
	memory.Padding()
	b, err := msgpack.Marshal(d)
	if err != nil {
		panic(&dns_panic{Err: err})
	}
	d.Domain = ""
	d = nil
	memory.Padding()
	this.opts_enc, err = this.cryptor.Encrypt(b)
	if err != nil {
		panic(&dns_panic{Err: err})
	}
	security.Flush_Bytes(b)
	return nil
}

func (this *DNS) Resolve() ([]*Node, error) {
	memory := security.New_Memory()
	defer memory.Flush()
	b, err := this.cryptor.Decrypt(this.opts_enc)
	if err != nil {
		panic(&dns_panic{Err: err})
	}
	d := &DNS{}
	err = msgpack.Unmarshal(b, d)
	if err != nil {
		panic(&dns_panic{Err: err})
	}
	memory.Padding()
	ip_list, err := this.resolver.Resolve(d.Domain, &d.Options)
	if err != nil {
		return nil, err
	}
	d.Domain = "" // <security>
	l := len(ip_list)
	nodes := make([]*Node, l)
	for i := 0; i < l; i++ {
		nodes[i] = &Node{
			Mode:    d.L_Mode,
			Network: d.L_Network,
		}
	}
	switch d.Options.Type {
	case "", dns.IPV4:
		for i := 0; i < l; i++ {
			nodes[i].Address = ip_list[i] + ":" + this.L_Port
		}
	case dns.IPV6:
		for i := 0; i < l; i++ {
			nodes[i].Address = "[" + ip_list[i] + "]:" + this.L_Port
		}
	default:
		panic(&dns_panic{Err: dns.ERR_INVALID_TYPE})
	}
	d = nil
	for i := 0; i < l; i++ { // <security>
		ip_list[i] = ""
	}
	return nodes, nil
}
