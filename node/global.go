package node

import (
	"encoding/base64"
	"sync"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/global/timesync"
	"project/internal/guid"
	"project/internal/random"
	"project/internal/security"
)

type global struct {
	ctx            *NODE
	proxy          *proxyclient.PROXY
	dns            *dnsclient.DNS
	timesync       *timesync.TIMESYNC
	object         map[uint32]interface{}
	object_rwm     sync.RWMutex
	configure_err  error
	configure_once sync.Once
	wg             sync.WaitGroup
}

func new_global(ctx *NODE) (*global, error) {
	config := ctx.config
	// <security> basic
	memory := security.New_Memory()
	memory.Padding()
	p, err := proxyclient.New(config.Proxy_Clients)
	if err != nil {
		return nil, errors.Wrap(err, "load proxy clients failed")
	}
	memory.Padding()
	d, err := dnsclient.New(p, config.DNS_Clients, config.DNS_Cache_Deadline)
	if err != nil {
		return nil, errors.Wrap(err, "load dns clients failed")
	}
	memory.Padding()
	t, err := timesync.New(p, d, ctx.logger, config.Timesync_Clients,
		config.Timesync_Interval)
	if err != nil {
		return nil, errors.Wrap(err, "load timesync clients failed")
	}
	g := &global{
		ctx:      ctx,
		proxy:    p,
		dns:      d,
		timesync: t,
	}
	memory.Flush()
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

func (this *global) configure() error {
	this.configure_once.Do(func() {
		this.sec_padding_memory()
		generator := random.New()
		// random object map
		this.object = make(map[uint32]interface{})
		for i := 0; i < 32+generator.Int(512); i++ { // 544 * 160 bytes
			key := object_key_max + uint32(1+generator.Int(512))
			this.object[key] = generator.Bytes(32 + generator.Int(128))
		}
		this.generate_internal_objects()
		this.configure_err = this.load_controller_config()
	})
	return this.configure_err
}

func (this *global) load_controller_config() error {
	this.sec_padding_memory()

	return nil
}

// 1. node guid
// 2. aes cryptor for database & self guid
func (this *global) generate_internal_objects() {
	// generate guid and select one
	this.sec_padding_memory()
	random_generator := random.New()
	guid_generator := guid.New(64, nil)
	var guid_pool [][]byte
	for i := 0; i < 1024; i++ {
		guid_pool = append(guid_pool, guid_generator.Get())
	}
	select_guid := make([]byte, guid.SIZE)
	copy(select_guid, guid_pool[random_generator.Int(1024)])
	this.object[node_guid] = select_guid
	// generate database aes
	aes_key := random_generator.Bytes(aes.BIT256)
	aes_iv := random_generator.Bytes(aes.IV_SIZE)
	cryptor, err := aes.New_CBC_Cryptor(aes_key, aes_iv)
	if err != nil {
		panic(err)
	}
	security.Flush_Bytes(aes_key)
	security.Flush_Bytes(aes_iv)
	this.object[database_aes] = cryptor
	// encrypt guid
	encrypt_guid, err := this.Database_Encrypt(this.GUID())
	if err != nil {
		panic(err)
	}
	str := base64.StdEncoding.EncodeToString(encrypt_guid)
	this.object[node_guid_encrypted] = str
}

// about internal
func (this *global) Start_Timesync() error {
	return this.timesync.Start()
}

func (this *global) GUID() []byte {
	this.object_rwm.RLock()
	g := this.object[node_guid]
	this.object_rwm.RUnlock()
	return g.([]byte)
}

func (this *global) GUID_Encrypted() string {
	this.object_rwm.RLock()
	g := this.object[node_guid_encrypted]
	this.object_rwm.RUnlock()
	return g.(string)
}

func (this *global) Certificate() []byte {
	this.object_rwm.RLock()
	c := this.object[certificate]
	this.object_rwm.RUnlock()
	if c != nil {
		return c.([]byte)
	} else {
		return nil
	}
}

func (this *global) Database_Encrypt(plaindata []byte) ([]byte, error) {
	this.object_rwm.RLock()
	c := this.object[database_aes].(*aes.CBC_Cryptor)
	this.object_rwm.RUnlock()
	return c.Encrypt(plaindata)
}
