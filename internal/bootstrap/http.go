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

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/cert"
	"project/internal/crypto/ed25519"
	"project/internal/dns"
	"project/internal/option"
	"project/internal/patch/msgpack"
	"project/internal/patch/toml"
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
	ErrInvalidSignatureSize = fmt.Errorf("invalid signature size")
	ErrInvalidSignature     = fmt.Errorf("invalid signature")
)

// HTTP is used to resolve bootstrap node listeners from HTTP response body.
type HTTP struct {
	ctx       context.Context
	certPool  *cert.Pool
	proxyPool *proxy.Pool
	dnsClient *dns.Client

	Request   option.HTTPRequest   `toml:"request" check:"-"`
	Transport option.HTTPTransport `toml:"transport" check:"-"`
	Timeout   time.Duration        `toml:"timeout"`
	ProxyTag  string               `toml:"proxy_tag"`
	DNSOpts   dns.Options          `toml:"dns" check:"-"`

	MaxBodySize int64 `toml:"max_body_size"` // <security>

	// encrypt & decrypt generate data(node listeners) ,hex encoded
	AESKey string `toml:"aes_key"`
	AESIV  string `toml:"aes_iv"`

	// for verify resolved node listeners data, hex encoded
	PublicKey string `toml:"public_key"`

	// for generate & marshal, controller set it
	PrivateKey ed25519.PrivateKey `toml:"-" check:"-"`

	// self encrypt all options
	cbc *aes.CBC
	enc []byte
}

// NewHTTP is used to create a HTTP mode bootstrap.
func NewHTTP(
	ctx context.Context,
	certPool *cert.Pool,
	proxyPool *proxy.Pool,
	dnsClient *dns.Client,
) *HTTP {
	return &HTTP{
		ctx:       ctx,
		certPool:  certPool,
		dnsClient: dnsClient,
		proxyPool: proxyPool,
	}
}

// Validate is used to check HTTP config correct.
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

// Generate is used to generate bootstrap information.
func (h *HTTP) Generate(listeners []*Listener) ([]byte, error) {
	if len(listeners) == 0 {
		return nil, errors.New("no bootstrap listeners")
	}
	data, _ := msgpack.Marshal(listeners)
	// confuse
	listenersData := bytes.Buffer{}
	rand := random.NewRand()
	i := 0
	for i = 4; i < len(data); i += 4 {
		listenersData.Write(rand.Bytes(8))
		listenersData.Write(data[i-4 : i])
	}
	end := data[i-4:]
	if end != nil {
		listenersData.Write(rand.Bytes(8))
		listenersData.Write(end)
	}
	// sign
	signature := ed25519.Sign(h.PrivateKey, listenersData.Bytes())
	buffer := bytes.Buffer{}
	// signature + listenersData
	buffer.Write(signature)
	buffer.Write(listenersData.Bytes())
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

// Marshal is used to marshal HTTP to []byte.
func (h *HTTP) Marshal() ([]byte, error) {
	publicKey := h.PrivateKey.PublicKey()
	h.PublicKey = hex.EncodeToString(publicKey)
	err := h.Validate()
	if err != nil {
		return nil, err
	}
	return toml.Marshal(h)
}

// flushRequestOption is used to cover string field if has secret.
func flushRequestOption(r *option.HTTPRequest) {
	security.CoverString(r.URL)
	security.CoverString(r.Post)
	security.CoverString(r.Host)
	for _, value := range r.Header {
		for i := 0; i < len(value); i++ {
			security.CoverString(value[i])
		}
	}
}

// coverHTTPRequest is used to cover http.Request string field if has secret.
func coverHTTPRequest(r *http.Request) {
	security.CoverString(r.Host)
	security.CoverString(r.URL.Host)
	security.CoverString(r.URL.Path)
	security.CoverString(r.URL.RawPath)
	for _, value := range r.Header {
		for i := 0; i < len(value); i++ {
			security.CoverString(value[i])
		}
	}
}

// Unmarshal is used to unmarshal []byte to HTTP.
func (h *HTTP) Unmarshal(data []byte) error {
	memory := security.NewMemory()
	defer memory.Flush()

	memory.Padding()
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
	memory.Padding()
	rand := random.NewRand()
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

// Resolve is used to get bootstrap node listeners.
func (h *HTTP) Resolve() ([]*Listener, error) {
	memory := security.NewMemory()
	defer memory.Flush()

	// decrypt all options
	memory.Padding()
	dec, err := h.cbc.Decrypt(h.enc)
	if err != nil {
		panic(err)
	}

	memory.Padding()
	tempHTTP := &HTTP{}
	err = msgpack.Unmarshal(dec, tempHTTP)
	if err != nil {
		panic(err)
	}
	defer flushRequestOption(&tempHTTP.Request)
	security.CoverBytes(dec)

	// apply options
	memory.Padding()
	req, err := tempHTTP.Request.Apply()
	if err != nil {
		panic(err)
	}
	defer coverHTTPRequest(req)
	tempHTTP.Transport.TLSClientConfig.CertPool = h.certPool
	transport, err := tempHTTP.Transport.Apply()
	if err != nil {
		panic(err)
	}
	transport.TLSClientConfig.ServerName = req.URL.Hostname()

	// set proxy
	proxyClient, err := h.proxyPool.Get(tempHTTP.ProxyTag)
	if err != nil {
		return nil, err
	}
	proxyClient.HTTP(transport)

	// resolve domain name
	hostname := req.URL.Hostname()
	defer security.CoverString(hostname)
	result, err := h.dnsClient.ResolveContext(h.ctx, hostname, &tempHTTP.DNSOpts)
	if err != nil {
		return nil, err
	}

	// set max body size
	maxBodySize := tempHTTP.MaxBodySize
	if maxBodySize < 1 {
		maxBodySize = defaultMaxBodySize
	}
	// timeout
	timeout := tempHTTP.Timeout
	if timeout < 1 {
		timeout = defaultTimeout
	}
	// make http client
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	port := req.URL.Port()
	var info []byte
	for i := 0; i < len(result); i++ {
		req := req.Clone(h.ctx)
		// replace host to IP address
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
		info, err = do(req, httpClient, maxBodySize)
		if err == nil {
			break
		}
	}
	if err == nil {
		return resolve(tempHTTP, info), nil
	}
	return nil, err
}

func do(req *http.Request, client *http.Client, length int64) ([]byte, error) {
	defer coverHTTPRequest(req)
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return ioutil.ReadAll(io.LimitReader(resp.Body, length))
}

func resolve(h *HTTP, info []byte) []*Listener {
	memory := security.NewMemory()
	defer memory.Flush()

	// decrypt data
	memory.Padding()
	cipherData := make([]byte, len(info)/2)
	_, err := hex.Decode(cipherData, info)
	if err != nil {
		panic(err)
	}
	aesKey, _ := hex.DecodeString(h.AESKey)
	security.CoverString(h.AESKey)
	aesIV, _ := hex.DecodeString(h.AESIV)
	security.CoverString(h.AESIV)
	data, err := aes.CBCDecrypt(cipherData, aesKey, aesIV)
	security.CoverBytes(aesKey)
	security.CoverBytes(aesIV)
	if err != nil {
		panic(err)
	}

	// verify, if appear error, call panic to log this error.
	memory.Padding()
	l := len(data)
	if l < ed25519.SignatureSize {
		panic(ErrInvalidSignatureSize)
	}
	signature := data[:ed25519.SignatureSize]
	listenersData := data[ed25519.SignatureSize:]
	pub, err := hex.DecodeString(h.PublicKey)
	security.CoverString(h.PublicKey)
	if err != nil {
		panic(err)
	}
	publicKey, err := ed25519.ImportPublicKey(pub)
	security.CoverBytes(pub)
	if err != nil {
		panic(err)
	}
	if !ed25519.Verify(publicKey, listenersData, signature) {
		panic(ErrInvalidSignature)
	}

	// remove confuse
	memory.Padding()
	listenersBuf := bytes.Buffer{}
	l = len(listenersData)
	i := 0
	for i = 0; i < l; i += 12 {
		if len(listenersData[i:]) > 11 {
			listenersBuf.Write(listenersData[i+8 : i+12])
		}
	}
	if i != l {
		if len(listenersData[i-12:]) > 8 {
			listenersBuf.Write(listenersData[i-4:]) // i+8-12
		}
	}

	// resolve bootstrap node listeners
	memory.Padding()
	listenersBytes := listenersBuf.Bytes()
	defer security.CoverBytes(listenersBytes)
	var listeners []*Listener
	err = msgpack.Unmarshal(listenersBytes, &listeners)
	if err != nil {
		panic(err)
	}
	return EncryptListeners(listeners)
}
