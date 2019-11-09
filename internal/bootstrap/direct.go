package bootstrap

import (
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/aes"
	"project/internal/random"
	"project/internal/security"
)

// Direct is used to resolve bootstrap nodes
// from local config
type Direct struct {
	Nodes []*Node `toml:"nodes"`

	// self store all encrypted nodes
	// by msgpack and AES CBC
	enc []byte
	cbc *aes.CBC
}

// NewDirect is used to
func NewDirect() *Direct {
	return new(Direct)
}

func (d *Direct) Validate() error { return nil }

func (d *Direct) Marshal() ([]byte, error) {
	if len(d.Nodes) == 0 {
		return nil, errors.New("no bootstrap nodes")
	}
	return toml.Marshal(d)
}

func (d *Direct) Unmarshal(config []byte) error {
	memory := security.NewMemory()
	defer memory.Flush()
	err := toml.Unmarshal(config, d)
	if err != nil {
		return err
	}
	rand := random.New(0)
	memory.Padding()
	key := rand.Bytes(aes.Key256Bit)
	iv := rand.Bytes(aes.IVSize)
	d.cbc, _ = aes.NewCBC(key, iv)
	security.FlushBytes(key)
	security.FlushBytes(iv)
	memory.Padding()
	b, _ := msgpack.Marshal(d.Nodes)
	defer security.FlushBytes(b)
	memory.Padding()
	d.enc, _ = d.cbc.Encrypt(b)
	return nil
}

func (d *Direct) Resolve() ([]*Node, error) {
	memory := security.NewMemory()
	defer memory.Flush()
	b, err := d.cbc.Decrypt(d.enc)
	defer security.FlushBytes(b)
	if err != nil {
		panic(&bPanic{Mode: ModeDirect, Err: err})
	}
	memory.Padding()
	var nodes []*Node
	err = msgpack.Unmarshal(b, &nodes)
	if err != nil {
		panic(&bPanic{Mode: ModeDirect, Err: err})
	}
	return nodes, nil
}
