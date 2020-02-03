package beacon

import (
	"io"
	"time"

	"project/internal/dns"
	"project/internal/proxy"
	"project/internal/timesync"
)

// Config contains configuration about Beacon
// use extra msgpack tag to hide raw field name
type Config struct {
	Test Test `toml:"-" msgpack:"-"`

	Logger struct {
		Level     string    `toml:"level"      msgpack:"a"`
		QueueSize int       `toml:"queue_size" msgpack:"b"`
		Writer    io.Writer `toml:"-"          msgpack:"-"`
	} `toml:"logger" msgpack:"aa"`

	Global struct {
		DNSCacheExpire      time.Duration `toml:"dns_cache_expire"      msgpack:"a"`
		TimeSyncSleepFixed  uint          `toml:"timesync_sleep_fixed"  msgpack:"b"`
		TimeSyncSleepRandom uint          `toml:"timesync_sleep_random" msgpack:"c"`
		TimeSyncInterval    time.Duration `toml:"timesync_interval"     msgpack:"d"`

		// generate from controller
		Certificates      [][]byte                    `toml:"-" msgpack:"w"`
		ProxyClients      []*proxy.Client             `toml:"-" msgpack:"x"`
		DNSServers        map[string]*dns.Server      `toml:"-" msgpack:"y"`
		TimeSyncerClients map[string]*timesync.Client `toml:"-" msgpack:"z"`
	} `toml:"global" msgpack:"bb"`

	Client struct {
		ProxyTag string        `toml:"proxy_tag" msgpack:"a"`
		Timeout  time.Duration `toml:"timeout"   msgpack:"b"`
		DNSOpts  dns.Options   `toml:"dns"       msgpack:"c"`
	} `toml:"client" msgpack:"cc"`

	Register struct {
		SleepFixed  uint `toml:"sleep_fixed"  msgpack:"a"` // about register failed
		SleepRandom uint `toml:"sleep_random" msgpack:"b"`

		// generate configs from controller
		FirstBoot []byte `toml:"-" msgpack:"w"` // type: *messages.Bootstrap
		FirstKey  []byte `toml:"-" msgpack:"x"` // decrypt the first bootstrap data, AES CBC
		RestBoots []byte `toml:"-" msgpack:"y"` // type: []*messages.Bootstrap
		RestKey   []byte `toml:"-" msgpack:"z"` // decrypt rest bootstraps data, AES CBC
	} `toml:"register" msgpack:"dd"`

	Sender struct {
		MaxConns      int           `toml:"max_conns"       msgpack:"a"`
		Worker        int           `toml:"worker"          msgpack:"b"`
		Timeout       time.Duration `toml:"timeout"         msgpack:"c"`
		QueueSize     int           `toml:"queue_size"      msgpack:"d"`
		MaxBufferSize int           `toml:"max_buffer_size" msgpack:"e"`
	} `toml:"sender" msgpack:"ff"`

	Syncer struct {
		ExpireTime time.Duration `toml:"expire_time" msgpack:"a"`
	} `toml:"syncer" msgpack:"gg"`

	Worker struct {
		Number        int `toml:"number"          msgpack:"a"`
		QueueSize     int `toml:"queue_size"      msgpack:"b"`
		MaxBufferSize int `toml:"max_buffer_size" msgpack:"c"`
	} `toml:"worker" msgpack:"hh"`

	// generate from controller
	CTRL struct {
		KexPublicKey []byte `msgpack:"x"` // key exchange curve25519
		PublicKey    []byte `msgpack:"y"` // verify message ed25519
		BroadcastKey []byte `msgpack:"z"` // decrypt broadcast, key + iv
	} `toml:"-" msgpack:"jj"`

	// about service
	Service struct {
		Name        string `toml:"name"         msgpack:"a"`
		DisplayName string `toml:"display_name" msgpack:"b"`
		Description string `toml:"description"  msgpack:"c"`
	} `toml:"service" msgpack:"kk"`
}
