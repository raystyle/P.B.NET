package node

import (
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
	ctx        *NODE
	proxy      *proxyclient.PROXY
	dns        *dnsclient.DNS
	timesync   *timesync.TIMESYNC
	object     map[object_key]interface{}
	object_rwm sync.RWMutex
	wg         sync.WaitGroup
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
	err = g.configure()
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

func (this *global) configure() error {
	this.sec_padding_memory()
	generator := random.New()
	// random object map
	this.object = make(map[object_key]interface{})
	for i := 0; i < 32+generator.Int(512); i++ { // 544 * 160 bytes
		key := object_key_max + 1 + generator.Int(512)
		this.object[key] = generator.Bytes(32 + generator.Int(128))
	}
	err := this.generate_objects()
	if err != nil {
		return err
	}
	return nil
}

// 1. node guid
// 2.
func (this *global) generate_objects() error {
	// generate guid and select one
	this.sec_padding_memory()
	random_generator := random.New()
	guid_generator := guid.New(64, nil)
	var guid_pool [][]byte
	for i := 0; i < 1024; i++ {
		guid_pool = append(guid_pool, guid_generator.Get())
	}
	select_guid := guid_pool[random_generator.Int(1024)]
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

	return nil
}
