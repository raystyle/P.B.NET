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
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/options"
	"project/internal/random"
	"project/internal/security"
)

var (
	ErrNoResponse           = errors.New("no response")
	ErrInvalidHeader        = errors.New("invalid signature header")
	ErrInvalidSignatureSize = errors.New("invalid signature size")
	ErrInvalidSignature     = errors.New("invalid signature")
)

type HTTP struct {
	Request   options.HTTPRequest   `toml:"request"`
	Transport options.HTTPTransport `toml:"transport"`
	Timeout   time.Duration         `toml:"timeout"`
	Proxy     string                `toml:"proxy"`
	DNSOpts   dns.Options           `toml:"dns_options"`

	// encrypt&decrypt generate data(nodes) hex
	AESKey string `toml:"aes_key"`
	AESIV  string `toml:"aes_iv"`

	// for resolve verify  hex
	PublicKey string `toml:"public_key"`

	// for generate&marshal
	PrivateKey ed25519.PrivateKey `toml:"-"` // <security>

	// runtime
	proxy    proxyPool
	resolver dnsResolver

	// self encrypt all options
	optsEnc []byte
	cbc     *aes.CBC
}

func NewHTTP(p proxyPool, r dnsResolver) *HTTP {
	return &HTTP{
		resolver: r,
		proxy:    p,
	}
}

func (h *HTTP) Validate() error {
	_, err := h.Request.Apply()
	if err != nil {
		return err
	}
	_, err = h.Transport.Apply()
	if err != nil {
		return err
	}
	aesKey, err := hex.DecodeString(h.AESKey)
	if err != nil {
		return err
	}
	aesIV, err := hex.DecodeString(h.AESIV)
	if err != nil {
		return err
	}
	_, err = aes.NewCBC(aesKey, aesIV)
	security.FlushBytes(aesKey)
	security.FlushBytes(aesIV)
	return err
}

func (h *HTTP) Generate(nodes []*Node) (string, error) {
	err := h.Validate()
	if err != nil {
		return "", err
	}
	data, err := msgpack.Marshal(nodes)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	// confuse
	nodesData := bytes.Buffer{}
	generator := random.New(0)
	i := 0
	for i = 4; i < len(data); i += 4 {
		nodesData.Write(generator.Bytes(8))
		nodesData.Write(data[i-4 : i])
	}
	end := data[i-4:]
	if end != nil {
		nodesData.Write(generator.Bytes(8))
		nodesData.Write(end)
	}
	// sign
	signature := ed25519.Sign(h.PrivateKey, nodesData.Bytes())
	buffer := bytes.Buffer{}
	// signature size + signature(nodesData) + nodesData
	buffer.Write(convert.Uint16ToBytes(uint16(len(signature))))
	buffer.Write(signature)
	buffer.Write(nodesData.Bytes())
	// encrypt
	key, err := hex.DecodeString(h.AESKey)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	iv, err := hex.DecodeString(h.AESIV)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	cipherdata, err := aes.CBCEncrypt(buffer.Bytes(), key, iv)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	return base64.StdEncoding.EncodeToString(cipherdata), nil
}

func (h *HTTP) Marshal() ([]byte, error) {
	err := h.Validate()
	if err != nil {
		return nil, err
	}
	publicKey := h.PrivateKey.PublicKey()
	h.PublicKey = hex.EncodeToString(publicKey)
	return toml.Marshal(h)
}

func (h *HTTP) Unmarshal(data []byte) error {
	tempHTTP := &HTTP{}
	err := toml.Unmarshal(data, tempHTTP)
	if err != nil {
		return err
	}
	err = tempHTTP.Validate()
	if err != nil {
		return err
	}
	// encrypt all options
	memory := security.NewMemory()
	defer memory.Flush()
	rand := random.New(0)
	key := rand.Bytes(aes.Bit256)
	iv := rand.Bytes(aes.IVSize)
	h.cbc, err = aes.NewCBC(key, iv)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	security.FlushBytes(key)
	security.FlushBytes(iv)
	memory.Padding()
	b, err := msgpack.Marshal(tempHTTP)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	tempHTTP = nil // <security>
	memory.Padding()
	h.optsEnc, err = h.cbc.Encrypt(b)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	security.FlushBytes(b)
	return nil
}

func (h *HTTP) Resolve() ([]*Node, error) {
	opts, err := h.applyOptions()
	if err != nil {
		return nil, err
	}
	// dns
	hostname := opts.req.URL.Hostname()
	ipList, err := h.resolver.Resolve(hostname, &opts.h.DNSOpts)
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
	switch opts.h.DNSOpts.Type {
	case "", dns.IPv4:
		for i := 0; i < len(ipList); i++ {
			opts.req.URL.Host = ipList[i] + port
			info, err := do(opts.req, opts.hc)
			if err == nil {
				return resolve(opts.h, info)
			}
		}
	case dns.IPv6:
		for i := 0; i < len(ipList); i++ {
			opts.req.URL.Host = "[" + ipList[i] + "]" + port
			info, err := do(opts.req, opts.hc)
			if err == nil {
				return resolve(opts.h, info)
			}
		}
	default:
		panic(&fPanic{Mode: ModeHTTP, Err: dns.ErrInvalidType})
	}
	return nil, ErrNoResponse
}

type httpOpts struct {
	req *http.Request
	hc  *http.Client
	h   *HTTP
}

func (h *HTTP) applyOptions() (*httpOpts, error) {
	// decrypt all options
	memory := security.NewMemory()
	defer memory.Flush()
	b, err := h.cbc.Decrypt(h.optsEnc)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	tempHTTP := &HTTP{}
	err = msgpack.Unmarshal(b, tempHTTP)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	security.FlushBytes(b)
	memory.Padding()
	// apply options
	req, err := tempHTTP.Request.Apply()
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	tr, err := tempHTTP.Transport.Apply()
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	tr.TLSClientConfig.ServerName = req.URL.Hostname()
	// set proxy
	proxy, err := h.proxy.Get(h.Proxy)
	if err != nil {
		return nil, err
	}
	if proxy != nil {
		proxy.HTTP(tr)
	}
	return &httpOpts{
		req: req,
		hc: &http.Client{
			Transport: tr,
			Timeout:   tempHTTP.Timeout,
		},
		h: tempHTTP,
	}, nil
}

func do(req *http.Request, c *http.Client) (string, error) {
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

func resolve(h *HTTP, info string) ([]*Node, error) {
	cipherdata, err := base64.StdEncoding.DecodeString(info)
	if err != nil {
		return nil, err
	}
	aesKey, err := hex.DecodeString(h.AESKey)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	h.AESKey = ""
	aesIV, err := hex.DecodeString(h.AESIV)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	h.AESIV = ""
	data, err := aes.CBCDecrypt(cipherdata, aesKey, aesIV)
	if err != nil {
		return nil, err
	}
	security.FlushBytes(aesKey)
	security.FlushBytes(aesIV)
	// signature size + signature(nodesData) + nodesData
	l := len(data)
	if l < 2 {
		return nil, ErrInvalidHeader
	}
	signatureSize := int(convert.BytesToUint16(data[:2]))
	if l < 2+signatureSize {
		return nil, ErrInvalidSignatureSize
	}
	signature := data[2 : 2+signatureSize]
	nodesData := data[2+signatureSize:]
	// verify
	pub, err := hex.DecodeString(h.PublicKey)
	if err != nil {
		return nil, err
	}
	h.PublicKey = ""
	publicKey, err := ed25519.ImportPublicKey(pub)
	if err != nil {
		return nil, err
	}
	if !ed25519.Verify(publicKey, nodesData, signature) {
		return nil, ErrInvalidSignature
	}
	security.FlushBytes(pub)
	// confuse
	nodesBuffer := bytes.Buffer{}
	l = len(nodesData)
	i := 0
	for i = 0; i < l; i += 12 {
		if len(nodesData[i:]) > 11 {
			nodesBuffer.Write(nodesData[i+8 : i+12])
		}
	}
	if i != l {
		if len(nodesData[i-12:]) > 8 {
			nodesBuffer.Write(nodesData[i-4:]) // i+8-12
		}
	}
	nodesBytes := nodesBuffer.Bytes()
	var nodes []*Node
	err = msgpack.Unmarshal(nodesBytes, &nodes)
	if err != nil {
		return nil, err
	}
	security.FlushBytes(nodesBytes)
	return nodes, nil
}
