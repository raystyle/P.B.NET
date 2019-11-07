package bootstrap

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/convert"
	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/options"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/security"
)

const (
	defaultMaxBodySize = 65535
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
	ProxyTag  string                `toml:"proxy_tag"`
	DNSOpts   dns.Options           `toml:"dns_options"`

	MaxBodySize int64 `toml:"max_body_size"` // <security>

	// encrypt&decrypt generate data(nodes) hex
	AESKey string `toml:"aes_key"`
	AESIV  string `toml:"aes_iv"`

	// for resolve verify  hex
	PublicKey string `toml:"public_key"`

	// for generate&marshal
	PrivateKey ed25519.PrivateKey `toml:"-"` // <security>

	// runtime
	ctx       context.Context
	proxyPool *proxy.Pool
	dnsClient *dns.Client

	// self encrypt all options
	optsEnc []byte
	cbc     *aes.CBC
}

func NewHTTP(ctx context.Context, pool *proxy.Pool, client *dns.Client) *HTTP {
	return &HTTP{
		ctx:       ctx,
		dnsClient: client,
		proxyPool: pool,
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

func (h *HTTP) Generate(nodes []*Node) string {
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
	cipherData, err := aes.CBCEncrypt(buffer.Bytes(), key, iv)
	if err != nil {
		panic(&fPanic{Mode: ModeHTTP, Err: err})
	}
	return base64.StdEncoding.EncodeToString(cipherData)
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
	key := rand.Bytes(aes.Key256Bit)
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
	p, err := h.proxyPool.Get(h.ProxyTag)
	if err != nil {
		return nil, err
	}
	p.HTTP(tr)
	timeout := tempHTTP.Timeout
	if timeout < 1 {
		timeout = options.DefaultDialTimeout
	}
	return &httpOpts{
		req: req,
		hc: &http.Client{
			Transport: tr,
			Timeout:   timeout,
		},
		h: tempHTTP,
	}, nil
}

func (h *HTTP) Resolve() ([]*Node, error) {
	opts, err := h.applyOptions()
	if err != nil {
		return nil, err
	}
	hostname := opts.req.URL.Hostname()

	// resolve domain name
	result, err := h.dnsClient.Resolve(hostname, &opts.h.DNSOpts)
	if err != nil {
		return nil, err
	}

	port := opts.req.URL.Port()

	maxBodySize := h.MaxBodySize
	if maxBodySize < 1 {
		maxBodySize = defaultMaxBodySize
	}

	for i := 0; i < len(result); i++ {
		req := opts.req.Clone(h.ctx)
		// replace to ip
		if port != "" {
			req.URL.Host = net.JoinHostPort(result[i], port)
		} else {
			req.URL.Host = result[i]
		}

		// set Host header
		// http://www.msfconnecttest.com/ -> http://96.126.123.244/
		// http will set host that not show domain name
		// but https useless, because TLS
		if req.Host == "" && req.URL.Scheme == "http" {
			req.Host = req.URL.Host
		}

		info, err := do(req, opts.hc, maxBodySize)
		if err == nil {
			return resolve(opts.h, info)
		}
	}
	return nil, ErrNoResponse
}

func do(req *http.Request, client *http.Client, length int64) (string, error) {
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := ioutil.ReadAll(io.LimitReader(resp.Body, length))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func resolve(h *HTTP, info string) ([]*Node, error) {
	cipherData, err := base64.StdEncoding.DecodeString(info)
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
	data, err := aes.CBCDecrypt(cipherData, aesKey, aesIV)
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
	nodesBuf := bytes.Buffer{}
	l = len(nodesData)
	i := 0
	for i = 0; i < l; i += 12 {
		if len(nodesData[i:]) > 11 {
			nodesBuf.Write(nodesData[i+8 : i+12])
		}
	}
	if i != l {
		if len(nodesData[i-12:]) > 8 {
			nodesBuf.Write(nodesData[i-4:]) // i+8-12
		}
	}
	nodesBytes := nodesBuf.Bytes()
	var nodes []*Node
	err = msgpack.Unmarshal(nodesBytes, &nodes)
	if err != nil {
		return nil, err
	}
	security.FlushBytes(nodesBytes)
	return nodes, nil
}
