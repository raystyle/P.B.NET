package bootstrap

import (
	"time"

	"project/internal/crypto/aes"
	"project/internal/global/dnsclient"
	"project/internal/options"
)

type HTTP struct {
	Request    *options.HTTP_Request
	Transport  *options.HTTP_Transport
	Timeout    time.Duration
	Proxy      string
	DNS        *dnsclient.Options
	AES_Key    string // encrypt&decrypt generate data hex
	AES_IV     string
	PublicKey  string // for resolve verify   not pem  hex
	PrivateKey string // for generate&marshal not pem  hex
	// runtime
	resolver dns_resolver
	proxy    proxy_pool
	opts_enc []byte // all options
	cryptor  *aes.CBC_Cryptor
}
