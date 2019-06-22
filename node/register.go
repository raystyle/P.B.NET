package node

import (
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/logger"
	"project/internal/security"
)

func (this *NODE) switch_register() {
	var err error
	if this.config.Is_Genesis {
		err = this.global.configure()
		if err != nil {
			err = errors.WithMessage(err, "global configure failed")
			goto exit
		}
		this.server, err = new_server(this)
		if err != nil {
			err = errors.WithMessage(err, "create server failed")
			goto exit
		}
	} else {
		err = this.auto_register()
	}
exit:
	if err != nil {
		this.logger.Println(logger.FATAL, "register", err)
	}
	this.config = nil
}

// success once
func (this *NODE) auto_register() error {
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
	for {
		for i := 0; i < l; i++ {
			c, err := aes.CBC_Decrypt(register[i].Config, key, iv)
			if err != nil {
				panic(err)
			}
			m := register[i].Mode
			boot, err := bootstrap.Load(m, c, this.global.proxy, this.global.dns)
			if err != nil {
				return errors.Wrap(err, "load bootstrap failed")
			}
			security.Flush_Bytes(c)
			err = this.global.configure()
			if err != nil {
				return errors.WithMessage(err, "global configure failed")
			}
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
