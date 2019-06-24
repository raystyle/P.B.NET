package node

import (
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"

	"project/internal/bootstrap"
	"project/internal/crypto/aes"
	"project/internal/logger"
	"project/internal/security"
)

func (this *presenter) register() {
	var err error
	defer func() {
		if err != nil {
			this.log(logger.FATAL, "register error: ", err)
			os.Exit(0)
		}
	}()
	if this.ctx.config.Is_Genesis {
		err = this.ctx.global.Configure()
		if err != nil {
			err = errors.WithMessage(err, "global configure failed")
			return
		}
		this.ctx.server, err = new_server(this.ctx)
		if err != nil {
			err = errors.WithMessage(err, "create server failed")
			return
		}
	} else {
		err = this.auto_register()
	}
	this.ctx.config = nil
}

// success once
func (this *presenter) auto_register() error {
	config := this.ctx.config
	global := this.ctx.global
	key := config.Register_AES_Key
	l := len(key)
	if l < aes.BIT128+aes.IV_SIZE {
		return errors.New("invalid register aes key")
	}
	iv := key[l-aes.IV_SIZE:]
	key = key[:l-aes.IV_SIZE]
	bootstraps := config.Register_Bootstraps
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
			err = global.Configure()
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
