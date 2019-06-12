package bootstrap

import (
	"time"

	"project/internal/crypto/aes"
	"project/internal/global/dnsclient"
	"project/internal/options"
)

type HTTP struct {
	Request   options.HTTP_Request   `toml:"request"`
	Transport options.HTTP_Transport `toml:"transport"`
	Timeout   time.Duration          `toml:"timeout"`
	Proxy     string                 `toml:"proxy"`
	DNS_Opts  dnsclient.Options      `toml:"dnsclient"`
	// encrypt&decrypt generate data hex
	AES_Key string `toml:"aes_key"`
	AES_IV  string `toml:"aes_iv"`
	// for resolve verify   not pem  hex
	PublicKey string `toml:"publickey"`
	// for generate&marshal not pem  hex
	PrivateKey string
	// runtime
	resolver dns_resolver
	proxy    proxy_pool
	opts_enc []byte // all options
	cryptor  *aes.CBC_Cryptor
}
