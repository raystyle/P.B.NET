package bootstrap

import (
	"fmt"

	"github.com/pelletier/go-toml"

	"project/internal/connection"
	"project/internal/crypto/aes"
	"project/internal/dns"
	"project/internal/global/dnsclient"
	"project/internal/random"
	"project/internal/security"
)

type DNS struct {
	Domain    string            `toml:"domain"`
	L_Mode    connection.Mode   `toml:"l_mode"`
	L_Network string            `toml:"l_network"`
	L_Port    string            `toml:"l_port"`
	Options   dnsclient.Options `toml:"dnsclient"`
	// runtime
	resolver   *dnsclient.DNS
	domain_enc []byte
	cryptor    *aes.CBC_Cryptor
}

// input ctx for resolve
func New_DNS(d *dnsclient.DNS) *DNS {
	return &DNS{
		resolver: d,
	}
}

func (this *DNS) Generate(_ []*Node) (string, error) {
	return "", nil
}

func (this *DNS) Marshal() ([]byte, error) {
	return toml.Marshal(this)
}

func (this *DNS) Unmarshal(data []byte) error {
	err := toml.Unmarshal(data, this)
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
		panic(fmt.Errorf("internal error: %s", err))
	}
	security.Flush_Bytes(key)
	security.Flush_Bytes(iv)
	memory.Padding()
	this.domain_enc, err = this.cryptor.Encrypt([]byte(this.Domain))
	if err != nil {
		panic(fmt.Errorf("internal error: %s", err))
	}
	this.Domain = "" // <security>
	return nil
}

func (this *DNS) Resolve() ([]*Node, error) {
	memory := security.New_Memory()
	defer memory.Flush()
	b, err := this.cryptor.Decrypt(this.domain_enc)
	if err != nil {
		return nil, err
	}
	memory.Padding()
	domain := string(b)
	ip_list, err := this.resolver.Resolve(domain, &this.Options)
	if err != nil {
		return nil, err
	}
	domain = "" // <security>
	l := len(ip_list)
	nodes := make([]*Node, l)
	for i := 0; i < l; i++ {
		nodes[i] = &Node{
			Mode:    this.L_Mode,
			Network: this.L_Network,
		}
	}
	switch this.Options.Opts.Type {
	case "", dns.IPV4:
		for i := 0; i < l; i++ {
			nodes[i].Address = ip_list[i] + ":" + this.L_Port
		}
	case dns.IPV6:
		for i := 0; i < l; i++ {
			nodes[i].Address = "[" + ip_list[i] + "]:" + this.L_Port
		}
	}
	for i := 0; i < l; i++ { // <security>
		ip_list[i] = ""
	}
	return nodes, nil
}
