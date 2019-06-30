package node

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/security"
)

// success once
func (this *NODE) register(c *Config) error {
	global := this.global
	key := c.Register_AES_Key
	l := len(key)
	if l < aes.BIT128+aes.IV_SIZE {
		return errors.New("invalid register aes key")
	}
	iv := key[l-aes.IV_SIZE:]
	key = key[:l-aes.IV_SIZE]
	bootstraps := c.Register_Bootstraps
	l = len(bootstraps)
	defer func() {
		for i := 0; i < l; i++ {
			security.Flush_Bytes(bootstraps[i].Config)
		}
		security.Flush_Bytes(key)
		security.Flush_Bytes(iv)
	}()
	for {
		for i := 0; i < l; i++ {
			c, err := aes.CBC_Decrypt(bootstraps[i].Config, key, iv)
			if err != nil {
				panic(err)
			}
			m := bootstraps[i].Mode
			boot, err := bootstrap.Load(m, c, global.proxy, global.dns)
			if err != nil {
				return errors.Wrap(err, "load bootstrap failed")
			}
			security.Flush_Bytes(c)
			// TODO more time
			for i := 0; i < 10; i++ {
				nodes, err := boot.Resolve()
				if err == nil {
					spew.Dump(nodes)
					return nil
				}
			}
		}
	}
}
