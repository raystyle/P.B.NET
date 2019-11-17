package node

import (
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/messages"
	"project/internal/modules/info"
)

/*
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
*/

func (node *Node) packOnlineRequest() []byte {
	req := messages.NodeRegisterRequest{
		GUID:         node.global.GUID(),
		PublicKey:    node.global.PublicKey(),
		KexPublicKey: node.global.KeyExchangePublicKey(),
		SystemInfo:   info.GetSystemInfo(),
		RequestTime:  node.global.Now(),
	}
	b, err := msgpack.Marshal(&req)
	if err != nil {
		panic(err)
	}
	return b
}
