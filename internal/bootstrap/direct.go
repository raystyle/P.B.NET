package bootstrap

import (
	"github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/aes"
	"project/internal/random"
	"project/internal/security"
)

// Direct is used to resolve bootstrap nodes from local config
type Direct struct {
	Nodes []*Node `toml:"nodes"`

	// self encrypt all options
	enc []byte
	cbc *aes.CBC
}

// NewDirect is used to create a direct mode bootstrap
func NewDirect() *Direct {
	return new(Direct)
}

// Validate is a padding function
func (d *Direct) Validate() error { return nil }

// Marshal is used to marshal Direct to []byte
func (d *Direct) Marshal() ([]byte, error) {
	if len(d.Nodes) == 0 {
		return nil, errors.New("no bootstrap nodes")
	}
	return toml.Marshal(d)
}

// Unmarshal is used to unmarshal []byte to Direct
func (d *Direct) Unmarshal(config []byte) error {
	memory := security.NewMemory()
	defer memory.Flush()
	err := toml.Unmarshal(config, d)
	if err != nil {
		return err
	}
	rand := random.New()
	memory.Padding()
	key := rand.Bytes(aes.Key256Bit)
	iv := rand.Bytes(aes.IVSize)
	d.cbc, _ = aes.NewCBC(key, iv)
	security.CoverBytes(key)
	security.CoverBytes(iv)
	memory.Padding()
	b, _ := msgpack.Marshal(d.Nodes)
	defer security.CoverBytes(b)
	memory.Padding()
	d.enc, err = d.cbc.Encrypt(b)
	return err
}

// Resolve is used to get bootstrap nodes
func (d *Direct) Resolve() ([]*Node, error) {
	memory := security.NewMemory()
	defer memory.Flush()
	b, err := d.cbc.Decrypt(d.enc)
	defer security.CoverBytes(b)
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
