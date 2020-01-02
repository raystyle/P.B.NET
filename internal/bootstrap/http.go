package bootstrap

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/options"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/security"
)

const (
	defaultTimeout     = 30 * time.Second
	defaultMaxBodySize = 65535
)

// errors
var (
	ErrNoResponse           = fmt.Errorf("no response")
	ErrInvalidSignatureSize = fmt.Errorf("invalid signature size")
	ErrInvalidSignature     = fmt.Errorf("invalid signature")
)

// HTTP is used to resolve bootstrap nodes from HTTP response body
type HTTP struct {
	Request   options.HTTPRequest   `toml:"request"`
	Transport options.HTTPTransport `toml:"transport"`
	Timeout   time.Duration         `toml:"timeout"`
	ProxyTag  string                `toml:"proxy_tag"`
	DNSOpts   dns.Options           `toml:"dns"`

	MaxBodySize int64 `toml:"max_body_size"` // <security>

	// encrypt & decrypt generate data(nodes) ,hex encoded
	AESKey string `toml:"aes_key"`
	AESIV  string `toml:"aes_iv"`

	// for verify resolve nodes data, hex encoded
	PublicKey string `toml:"public_key"`

	// for generate & marshal, controller set it
	PrivateKey ed25519.PrivateKey `toml:"-"`

	// runtime
	ctx       context.Context
	proxyPool *proxy.Pool
	dnsClient *dns.Client

	// self encrypt all options
	enc []byte
	cbc *aes.CBC
}

// NewHTTP is used to create a HTTP mode bootstrap
func NewHTTP(ctx context.Context, pool *proxy.Pool, client *dns.Client) *HTTP {
	return &HTTP{
		ctx:       ctx,
		dnsClient: client,
		proxyPool: pool,
	}
}

// Validate is used to check HTTP config correct
func (h *HTTP) Validate() error {
	_, err := h.Request.Apply()
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = h.Transport.Apply()
	if err != nil {
		return errors.WithStack(err)
	}
	aesKey, err := hex.DecodeString(h.AESKey)
	if err != nil {
		return errors.WithStack(err)
	}
	defer security.CoverBytes(aesKey)
	aesIV, err := hex.DecodeString(h.AESIV)
	if err != nil {
		return errors.WithStack(err)
	}
	defer security.CoverBytes(aesIV)
	_, err = aes.NewCBC(aesKey, aesIV)
	if err != nil {
		return errors.WithStack(err)
	}
	publicKey, err := hex.DecodeString(h.PublicKey)
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = ed25519.ImportPublicKey(publicKey)
	return errors.WithStack(err)
}

// Generate is used to generate bootstrap info
func (h *HTTP) Generate(nodes []*Node) ([]byte, error) {
	if len(nodes) == 0 {
		return nil, errors.New("no bootstrap nodes")
	}
	data, _ := msgpack.Marshal(nodes)
	// confuse
	nodesData := bytes.Buffer{}
	generator := random.New()
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
	// signature + nodesData
	buffer.Write(signature)
	buffer.Write(nodesData.Bytes())
	// encrypt
	key, err := hex.DecodeString(h.AESKey)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	iv, err := hex.DecodeString(h.AESIV)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	cipherData, err := aes.CBCEncrypt(buffer.Bytes(), key, iv)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	dst := make([]byte, 2*len(cipherData))
	hex.Encode(dst, cipherData)
	return dst, nil
}

// Marshal is used to marshal HTTP to []byte
func (h *HTTP) Marshal() ([]byte, error) {
	publicKey := h.PrivateKey.PublicKey()
	h.PublicKey = hex.EncodeToString(publicKey)
	err := h.Validate()
	if err != nil {
		return nil, err
	}
	return toml.Marshal(h)
}

// flushRequestOption is used to cover string field if has secret
func flushRequestOption(r *options.HTTPRequest) {
	security.CoverString(&r.URL)
	security.CoverString(&r.Post)
	security.CoverString(&r.Host)
}

// Unmarshal is used to unmarshal []byte to HTTP
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
	rand := random.New()
	key := rand.Bytes(aes.Key256Bit)
	iv := rand.Bytes(aes.IVSize)
	h.cbc, _ = aes.NewCBC(key, iv)
	security.CoverBytes(key)
	security.CoverBytes(iv)
	memory.Padding()
	b, _ := msgpack.Marshal(tempHTTP)
	defer security.CoverBytes(b)
	flushRequestOption(&tempHTTP.Request)
	memory.Padding()
	h.enc, err = h.cbc.Encrypt(b)
	return err
}

// Resolve is used to get bootstrap nodes
func (h *HTTP) Resolve() ([]*Node, error) {
	// decrypt all options
	memory := security.NewMemory()
	defer memory.Flush()
	b, err := h.cbc.Decrypt(h.enc)
	if err != nil {
		panic(err)
	}
	tHTTP := &HTTP{}
	err = msgpack.Unmarshal(b, tHTTP)
	if err != nil {
		panic(err)
	}
	defer flushRequestOption(&tHTTP.Request)
	security.CoverBytes(b)
	memory.Padding()

	// apply options
	req, err := tHTTP.Request.Apply()
	if err != nil {
		panic(err)
	}
	defer security.CoverHTTPRequest(req)
	tr, err := tHTTP.Transport.Apply()
	if err != nil {
		panic(err)
	}
	tr.TLSClientConfig.ServerName = req.URL.Hostname()

	// set proxy
	p, err := h.proxyPool.Get(tHTTP.ProxyTag)
	if err != nil {
		return nil, err
	}
	p.HTTP(tr)

	hostname := req.URL.Hostname()
	defer security.CoverString(&hostname)

	// resolve domain name
	result, err := h.dnsClient.ResolveWithContext(h.ctx, hostname, &tHTTP.DNSOpts)
	if err != nil {
		return nil, err
	}

	port := req.URL.Port()

	maxBodySize := tHTTP.MaxBodySize
	if maxBodySize < 1 {
		maxBodySize = defaultMaxBodySize
	}

	// timeout
	timeout := tHTTP.Timeout
	if timeout < 1 {
		timeout = defaultTimeout
	}

	// make http client
	hc := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	for i := 0; i < len(result); i++ {
		req := req.Clone(h.ctx)

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

		info, err := do(req, hc, maxBodySize)
		if err == nil {
			return resolve(tHTTP, info), nil
		}
	}
	return nil, ErrNoResponse
}

func do(req *http.Request, client *http.Client, length int64) ([]byte, error) {
	defer security.CoverHTTPRequest(req)
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return ioutil.ReadAll(io.LimitReader(resp.Body, length))
}

func resolve(h *HTTP, info []byte) []*Node {
	// decrypt data
	cipherData := make([]byte, len(info)/2)
	_, err := hex.Decode(cipherData, info)
	if err != nil {
		panic(err)
	}
	aesKey, _ := hex.DecodeString(h.AESKey)
	security.CoverString(&h.AESKey)
	aesIV, _ := hex.DecodeString(h.AESIV)
	security.CoverString(&h.AESIV)
	data, err := aes.CBCDecrypt(cipherData, aesKey, aesIV)
	security.CoverBytes(aesKey)
	security.CoverBytes(aesIV)
	if err != nil {
		panic(err)
	}

	// verify
	l := len(data)
	if l < ed25519.SignatureSize {
		panic(ErrInvalidSignatureSize)
	}
	signature := data[:ed25519.SignatureSize]
	nodesData := data[ed25519.SignatureSize:]
	pub, err := hex.DecodeString(h.PublicKey)
	security.CoverString(&h.PublicKey)
	if err != nil {
		panic(err)
	}
	publicKey, err := ed25519.ImportPublicKey(pub)
	security.CoverBytes(pub)
	if err != nil {
		panic(err)
	}
	if !ed25519.Verify(publicKey, nodesData, signature) {
		panic(ErrInvalidSignature)
	}

	// remove confuse
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

	// resolve bootstrap nodes
	nodesBytes := nodesBuf.Bytes()
	var nodes []*Node
	err = msgpack.Unmarshal(nodesBytes, &nodes)
	if err != nil {
		panic(err)
	}
	security.CoverBytes(nodesBytes)
	return nodes
}
