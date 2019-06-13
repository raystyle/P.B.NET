package bootstrap

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/vmihailenco/msgpack"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ecdsa"
	"project/internal/global/dnsclient"
	"project/internal/options"
	"project/internal/random"
)

type http_panic struct {
	Err error
}

func (this *http_panic) Error() string {
	return fmt.Sprintf("bootstrap http internal error: %s", this.Err)
}

type HTTP struct {
	Request   options.HTTP_Request   `toml:"request"`
	Transport options.HTTP_Transport `toml:"transport"`
	Timeout   time.Duration          `toml:"timeout"`
	Proxy     string                 `toml:"proxy"`
	DNS_Opts  dnsclient.Options      `toml:"dnsclient"`
	// encrypt&decrypt generate data(nodes) hex
	AES_Key string `toml:"aes_key"`
	AES_IV  string `toml:"aes_iv"`
	// for resolve verify pem
	PublicKey string `toml:"publickey"`
	// for generate&marshal not pem  hex
	PrivateKey *ecdsa.PrivateKey `toml:"-"`
	// runtime
	resolver dns_resolver
	proxy    proxy_pool
	// self encrypt all options
	opts_enc []byte
	cryptor  *aes.CBC_Cryptor
}

func New_HTTP(d dns_resolver, p proxy_pool) *HTTP {
	return &HTTP{
		resolver: d,
		proxy:    p,
	}
}

func (this *HTTP) Generate(nodes []*Node) (string, error) {
	// signature size + signature(nodes_data) + nodes_data
	data, err := msgpack.Marshal(nodes)
	if err != nil {
		panic(&http_panic{Err: err})
	}
	// confuse
	nodes_data := bytes.Buffer{}
	generator := random.New()
	i := 0
	for i = 4; i < len(data); i += 4 {
		nodes_data.Write(generator.Bytes(8))
		nodes_data.Write(data[i-4 : i])
	}
	end := data[i-4:]
	if end != nil {
		nodes_data.Write(generator.Bytes(8))
		nodes_data.Write(end)
	}
	// sign
	signature, err := ecdsa.Sign(this.PrivateKey, nodes_data.Bytes())
	if err != nil {
		panic(&http_panic{Err: err})
	}
	buffer := bytes.Buffer{}
	buffer.Write(convert.Uint16_Bytes(uint16(len(signature))))
	buffer.Write(signature)
	buffer.Write(nodes_data.Bytes())
	// encrypt
	key, err := hex.DecodeString(this.AES_Key)
	if err != nil {
		return "", err
	}
	iv, err := hex.DecodeString(this.AES_IV)
	if err != nil {
		return "", err
	}
	cipherdata, err := aes.CBC_Encrypt(buffer.Bytes(), key, iv)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cipherdata), nil
}

func (this *HTTP) Marshal() ([]byte, error) {

}
