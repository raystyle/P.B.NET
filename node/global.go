package node

import (
	"encoding/base64"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/random"
	"project/internal/security"
)

type global struct {
	proxy      *proxyclient.PROXY
	dns        *dnsclient.DNS
	timesync   *timesync.TIMESYNC
	object     map[uint32]interface{}
	object_rwm sync.RWMutex
	conf_err   error
	conf_once  sync.Once
	wg         sync.WaitGroup
}

func new_global(lg logger.Logger, c *Config) (*global, error) {
	// <security> basic
	memory := security.New_Memory()
	memory.Padding()
	p, err := proxyclient.New(c.Proxy_Clients)
	if err != nil {
		return nil, errors.Wrap(err, "load proxy clients failed")
	}
	memory.Padding()
	d, err := dnsclient.New(p, c.DNS_Clients, c.DNS_Cache_Deadline)
	if err != nil {
		return nil, errors.Wrap(err, "load dns clients failed")
	}
	memory.Padding()
	var l logger.Logger
	if c.Check_Mode {
		l = logger.Discard
	} else {
		l = lg
	}
	t, err := timesync.New(p, d, l, c.Timesync_Clients, c.Timesync_Interval)
	if err != nil {
		return nil, errors.Wrap(err, "load timesync clients failed")
	}
	memory.Flush()
	g := &global{
		proxy:    p,
		dns:      d,
		timesync: t,
	}
	err = g.configure(c)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// <security>
func (this *global) sec_padding_memory() {
	generator := random.New()
	memory := security.New_Memory()
	security.Padding_Memory()
	padding := func() {
		for i := 0; i < 32+generator.Int(256); i++ {
			memory.Padding()
		}
	}
	this.wg.Add(1)
	go func() {
		padding()
		this.wg.Done()
	}()
	padding()
	this.wg.Wait()
}

func (this *global) configure(c *Config) error {
	this.conf_once.Do(func() {
		this.sec_padding_memory()
		rand := random.New()
		// random object map
		this.object = make(map[uint32]interface{})
		for i := 0; i < 32+rand.Int(512); i++ { // 544 * 160 bytes
			key := object_key_max + uint32(1+rand.Int(512))
			this.object[key] = rand.Bytes(32 + rand.Int(128))
		}
		this.gen_internal_objects()
		this.conf_err = this.load_ctrl_configs(c)
	})
	return this.conf_err
}

func (this *global) load_ctrl_configs(c *Config) error {
	this.sec_padding_memory()
	// controller ed25519 public key
	pub := c.CTRL_ED25519
	publickey, err := ed25519.Import_PublicKey(pub)
	if err != nil {
		return errors.WithStack(err)
	}
	this.object[ctrl_ed25519] = publickey
	// controller aes
	key := c.CTRL_AES_Key
	l := len(key)
	if l < aes.BIT128+aes.IV_SIZE {
		return errors.New("invalid controller aes key")
	}
	iv := key[l-aes.IV_SIZE:]
	key = key[:l-aes.IV_SIZE]
	cryptor, err := aes.New_CBC_Cryptor(key, iv)
	if err != nil {
		return errors.WithStack(err)
	}
	this.object[ctrl_aes_cryptor] = cryptor
	return nil
}

// 1. node guid
// 2. aes cryptor for database & self guid
func (this *global) gen_internal_objects() {
	// generate guid and select one
	this.sec_padding_memory()
	rand := random.New()
	guid_generator := guid.New(64, nil)
	var guid_pool [1024][]byte
	for i := 0; i < len(guid_pool); i++ {
		guid_pool[i] = guid_generator.Get()
	}
	guid_generator.Close()
	guid_selected := make([]byte, guid.SIZE)
	copy(guid_selected, guid_pool[rand.Int(1024)])
	this.object[node_guid] = guid_selected
	// generate database aes
	aes_key := rand.Bytes(aes.BIT256)
	aes_iv := rand.Bytes(aes.IV_SIZE)
	cryptor, err := aes.New_CBC_Cryptor(aes_key, aes_iv)
	if err != nil {
		panic(err)
	}
	security.Flush_Bytes(aes_key)
	security.Flush_Bytes(aes_iv)
	this.object[db_aes_cryptor] = cryptor
	// encrypt guid
	guid_enc, err := this.DB_Encrypt(this.GUID())
	if err != nil {
		panic(err)
	}
	str := base64.StdEncoding.EncodeToString(guid_enc)
	this.object[node_guid_enc] = str
}

// about internal

func (this *global) Start_Timesync() error {
	return this.timesync.Start()
}

func (this *global) Now() time.Time {
	return this.timesync.Now().Local()
}

func (this *global) GUID() []byte {
	this.object_rwm.RLock()
	g := this.object[node_guid]
	this.object_rwm.RUnlock()
	return g.([]byte)
}

func (this *global) GUID_Enc() string {
	this.object_rwm.RLock()
	g := this.object[node_guid_enc]
	this.object_rwm.RUnlock()
	return g.(string)
}

func (this *global) Cert() []byte {
	this.object_rwm.RLock()
	c := this.object[certificate]
	this.object_rwm.RUnlock()
	if c != nil {
		return c.([]byte)
	} else {
		return nil
	}
}

// use controller publickey to verify message
func (this *global) CTRL_Verify(message, signature []byte) bool {
	this.object_rwm.RLock()
	p := this.object[ctrl_ed25519]
	this.object_rwm.RUnlock()
	return ed25519.Verify(p.(ed25519.PublicKey), message, signature)
}

func (this *global) CTRL_Decrypt(cipherdata []byte) ([]byte, error) {
	this.object_rwm.RLock()
	k := this.object[ctrl_aes_cryptor]
	this.object_rwm.RUnlock()
	return k.(*aes.CBC_Cryptor).Decrypt(cipherdata)
}

func (this *global) DB_Encrypt(plaindata []byte) ([]byte, error) {
	this.object_rwm.RLock()
	c := this.object[db_aes_cryptor]
	this.object_rwm.RUnlock()
	return c.(*aes.CBC_Cryptor).Encrypt(plaindata)
}

func (this *global) DB_Decrypt(cipherdata []byte) ([]byte, error) {
	this.object_rwm.RLock()
	c := this.object[db_aes_cryptor]
	this.object_rwm.RUnlock()
	return c.(*aes.CBC_Cryptor).Decrypt(cipherdata)
}

func (this *global) Close() {
	this.timesync.Stop()
}
