package bootstrap

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/vmihailenco/msgpack"
	"golang.org/x/crypto/ed25519"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/dns"
	"project/internal/global/dnsclient"
	"project/internal/options"
	"project/internal/random"
	"project/internal/security"
)

var (
	ERR_NO_RESPONSE            = errors.New("no response")
	ERR_INVALID_HEADER         = errors.New("invalid signature header")
	ERR_INVALID_SIGNATURE_SIZE = errors.New("invalid signature size")
	ERR_INVALID_SIGNATURE      = errors.New("invalid signature")
)

type HTTP struct {
	Request   options.HTTP_Request   `toml:"request"`
	Transport options.HTTP_Transport `toml:"transport"`
	Timeout   time.Duration          `toml:"timeout"`
	Proxy     string                 `toml:"proxy"`
	DNS_Opts  dnsclient.Options      `toml:"dnsclient"`

	// encrypt&decrypt generate data(nodes) hex
	AES_Key string `toml:"aes_key"`
	AES_IV  string `toml:"aes_iv"`

	// for resolve verify  hex
	PublicKey string `toml:"publickey"`

	// for generate&marshal
	PrivateKey ed25519.PrivateKey `toml:"-"`

	// runtime
	proxy    proxy_pool
	resolver dns_resolver

	// self encrypt all options
	opts_enc []byte
	cryptor  *aes.CBC_Cryptor
}

func New_HTTP(p proxy_pool, d dns_resolver) *HTTP {
	return &HTTP{
		resolver: d,
		proxy:    p,
	}
}

func (this *HTTP) Validate() error {
	_, err := this.Request.Apply()
	if err != nil {
		return err
	}
	_, err = this.Transport.Apply()
	if err != nil {
		return err
	}
	aes_key, err := hex.DecodeString(this.AES_Key)
	if err != nil {
		return err
	}
	aes_iv, err := hex.DecodeString(this.AES_IV)
	if err != nil {
		return err
	}
	_, err = aes.New_CBC_Cryptor(aes_key, aes_iv)
	security.Flush_Bytes(aes_key)
	security.Flush_Bytes(aes_iv)
	return err
}

func (this *HTTP) Generate(nodes []*Node) (string, error) {
	err := this.Validate()
	if err != nil {
		return "", err
	}
	data, err := msgpack.Marshal(nodes)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
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
	signature := ed25519.Sign(this.PrivateKey, nodes_data.Bytes())
	buffer := bytes.Buffer{}
	// signature size + signature(nodes_data) + nodes_data
	buffer.Write(convert.Uint16_Bytes(uint16(len(signature))))
	buffer.Write(signature)
	buffer.Write(nodes_data.Bytes())
	// encrypt
	key, err := hex.DecodeString(this.AES_Key)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	iv, err := hex.DecodeString(this.AES_IV)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	cipherdata, err := aes.CBC_Encrypt(buffer.Bytes(), key, iv)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	return base64.StdEncoding.EncodeToString(cipherdata), nil
}

func (this *HTTP) Marshal() ([]byte, error) {
	err := this.Validate()
	if err != nil {
		return nil, err
	}
	publickey := this.PrivateKey[ed25519.PublicKeySize:]
	this.PublicKey = hex.EncodeToString(publickey)
	return toml.Marshal(this)
}

func (this *HTTP) Unmarshal(data []byte) error {
	h := &HTTP{}
	err := toml.Unmarshal(data, h)
	if err != nil {
		return err
	}
	err = h.Validate()
	if err != nil {
		return err
	}
	// encrypt all options
	memory := security.New_Memory()
	defer memory.Flush()
	rand := random.New()
	key := rand.Bytes(aes.BIT256)
	iv := rand.Bytes(aes.IV_SIZE)
	this.cryptor, err = aes.New_CBC_Cryptor(key, iv)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	security.Flush_Bytes(key)
	security.Flush_Bytes(iv)
	memory.Padding()
	b, err := msgpack.Marshal(h)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	h = nil // <security>
	memory.Padding()
	this.opts_enc, err = this.cryptor.Encrypt(b)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	security.Flush_Bytes(b)
	return nil
}

func (this *HTTP) Resolve() ([]*Node, error) {
	opts, err := this.apply_options()
	if err != nil {
		return nil, err
	}
	// dns
	hostname := opts.req.URL.Hostname()
	dns_opts := &opts.h.DNS_Opts
	ip_list, err := this.resolver.Resolve(hostname, dns_opts)
	if err != nil {
		return nil, err
	}
	if opts.req.URL.Scheme == "https" {
		if opts.req.Host == "" {
			opts.req.Host = opts.req.URL.Host
		}
	}
	port := opts.req.URL.Port()
	if port != "" {
		port = ":" + port
	}
	switch dns_opts.Type {
	case "", dns.IPV4:
		for i := 0; i < len(ip_list); i++ {
			opts.req.URL.Host = ip_list[i] + port
			info, err := this.do(opts.req, opts.hc)
			if err == nil {
				return this.resolve(opts.h, info)
			}
		}
	case dns.IPV6:
		for i := 0; i < len(ip_list); i++ {
			opts.req.URL.Host = "[" + ip_list[i] + "]" + port
			info, err := this.do(opts.req, opts.hc)
			if err == nil {
				return this.resolve(opts.h, info)
			}
		}
	default:
		panic(&fpanic{Mode: M_HTTP, Err: dns.ERR_INVALID_TYPE})
	}
	return nil, ERR_NO_RESPONSE
}

type http_opts struct {
	req *http.Request
	hc  *http.Client
	h   *HTTP
}

func (this *HTTP) apply_options() (*http_opts, error) {
	// decrypt all options
	memory := security.New_Memory()
	defer memory.Flush()
	b, err := this.cryptor.Decrypt(this.opts_enc)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	h := &HTTP{}
	err = msgpack.Unmarshal(b, h)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	security.Flush_Bytes(b)
	memory.Padding()
	// apply options
	req, err := h.Request.Apply()
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	tr, err := h.Transport.Apply()
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	tr.TLSClientConfig.ServerName = req.URL.Hostname()
	// set proxy
	proxy, err := this.proxy.Get(this.Proxy)
	if err != nil {
		return nil, err
	}
	if proxy != nil {
		proxy.HTTP(tr)
	}
	return &http_opts{
		req: req,
		hc: &http.Client{
			Transport: tr,
			Timeout:   h.Timeout,
		},
		h: h,
	}, nil
}

func (this *HTTP) do(req *http.Request, c *http.Client) (string, error) {
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
		c.CloseIdleConnections()
	}()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (this *HTTP) resolve(h *HTTP, info string) ([]*Node, error) {
	cipherdata, err := base64.StdEncoding.DecodeString(info)
	if err != nil {
		return nil, err
	}
	aes_key, err := hex.DecodeString(h.AES_Key)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	h.AES_Key = ""
	aes_iv, err := hex.DecodeString(h.AES_IV)
	if err != nil {
		panic(&fpanic{Mode: M_HTTP, Err: err})
	}
	h.AES_IV = ""
	data, err := aes.CBC_Decrypt(cipherdata, aes_key, aes_iv)
	if err != nil {
		return nil, err
	}
	security.Flush_Bytes(aes_key)
	security.Flush_Bytes(aes_iv)
	// signature size + signature(nodes_data) + nodes_data
	l := len(data)
	if l < 2 {
		return nil, ERR_INVALID_HEADER
	}
	signature_size := int(convert.Bytes_Uint16(data[:2]))
	if l < 2+signature_size {
		return nil, ERR_INVALID_SIGNATURE_SIZE
	}
	signature := data[2 : 2+signature_size]
	nodes_data := data[2+signature_size:]
	// verify
	publickey, err := hex.DecodeString(h.PublicKey)
	if err != nil {
		return nil, err
	}
	h.PublicKey = ""
	if !ed25519.Verify(ed25519.PublicKey(publickey), nodes_data, signature) {
		return nil, ERR_INVALID_SIGNATURE
	}
	security.Flush_Bytes(publickey)
	// deconfuse
	nodes_buffer := bytes.Buffer{}
	l = len(nodes_data)
	i := 0
	for i = 0; i < l; i += 12 {
		if len(nodes_data[i:]) > 11 {
			nodes_buffer.Write(nodes_data[i+8 : i+12])
		}
	}
	if i != l {
		if len(nodes_data[i-12:]) > 8 {
			nodes_buffer.Write(nodes_data[i-4:]) // i+8-12
		}
	}
	nodes_bytes := nodes_buffer.Bytes()
	var nodes []*Node
	err = msgpack.Unmarshal(nodes_bytes, &nodes)
	if err != nil {
		return nil, err
	}
	security.Flush_Bytes(nodes_bytes)
	return nodes, nil
}
