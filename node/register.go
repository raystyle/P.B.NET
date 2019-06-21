package node

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/security"
)

// success once
func (this *NODE) register() error {
	register := this.config.Register_Config
	key := this.config.Register_AES_Key
	iv := this.config.Register_AES_IV
	l := len(register)
	defer func() {
		for i := 0; i < l; i++ {
			security.Flush_Bytes(register[i].Config)
		}
		security.Flush_Bytes(key)
		security.Flush_Bytes(iv)
	}()
	for i := 0; i < l; i++ {
		config, err := aes.CBC_Decrypt(register[i].Config, key, iv)
		if err != nil {
			panic(err)
		}
		c := &bootstrap.Config{
			Mode:   register[i].Mode,
			Config: config,
		}
		boot, err := bootstrap.Load(c, this.global.proxy, this.global.dns)
		if err != nil {
			return errors.Wrap(err, "load bootstrap failed")
		}
		security.Flush_Bytes(config)
		for i := 0; i < 10; i++ {
			nodes, err := boot.Resolve()
			if err == nil {
				spew.Dump(nodes)
				return nil
			}
		}

	}
	return nil
}
