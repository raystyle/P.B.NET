package node

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/security"
)

// success once
func (node *NODE) register(c *Config) error {
	global := node.global
	key := c.RegisterAESKey
	l := len(key)
	if l < aes.Bit128+aes.IVSize {
		return errors.New("invalid register aes key")
	}
	iv := key[l-aes.IVSize:]
	key = key[:l-aes.IVSize]
	bootstraps := c.RegisterBootstraps
	l = len(bootstraps)
	defer func() {
		for i := 0; i < l; i++ {
			security.FlushBytes(bootstraps[i].Config)
		}
		security.FlushBytes(key)
		security.FlushBytes(iv)
	}()
	for {
		for i := 0; i < l; i++ {
			c, err := aes.CBCDecrypt(bootstraps[i].Config, key, iv)
			if err != nil {
				panic(err)
			}
			m := bootstraps[i].Mode
			boot, err := bootstrap.Load(m, c, global.proxyPool, global.dnsClient)
			if err != nil {
				return errors.Wrap(err, "load bootstrap failed")
			}
			security.FlushBytes(c)
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
