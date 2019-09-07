package bootstrap

import (
	"github.com/pelletier/go-toml"
	"github.com/vmihailenco/msgpack/v4"

	"project/internal/crypto/aes"
	"project/internal/random"
	"project/internal/security"
)

type Direct struct {
	nodes []*Node
	// self store all encrypted nodes by msgpack
	nodesEnc []byte
	cbc      *aes.CBC
}

func NewDirect(n []*Node) *Direct {
	d := &Direct{nodes: make([]*Node, len(n))}
	copy(d.nodes, n)
	return d
}

func (d *Direct) Validate() error { return nil }

func (d *Direct) Marshal() ([]byte, error) {
	nodes := &struct {
		Nodes []*Node `toml:"nodes"`
	}{}
	nodes.Nodes = make([]*Node, len(d.nodes))
	copy(nodes.Nodes, d.nodes)
	return toml.Marshal(nodes)
}

func (d *Direct) Unmarshal(data []byte) error {
	nodes := &struct {
		Nodes []*Node `toml:"nodes"`
	}{}
	err := toml.Unmarshal(data, nodes)
	if err != nil {
		return err
	}
	memory := security.NewMemory()
	defer memory.Flush()
	rand := random.New(0)
	memory.Padding()
	key := rand.Bytes(aes.Bit256)
	iv := rand.Bytes(aes.IVSize)
	d.cbc, err = aes.NewCBC(key, iv)
	if err != nil {
		panic(&fPanic{Mode: ModeDirect, Err: err})
	}
	security.FlushBytes(key)
	security.FlushBytes(iv)
	b, err := msgpack.Marshal(&nodes.Nodes)
	if err != nil {
		panic(&fPanic{Mode: ModeDirect, Err: err})
	}
	memory.Padding()
	d.nodesEnc, err = d.cbc.Encrypt(b)
	if err != nil {
		panic(&fPanic{Err: err})
	}
	security.FlushBytes(b)
	return nil
}

func (d *Direct) Resolve() ([]*Node, error) {
	memory := security.NewMemory()
	defer memory.Flush()
	b, err := d.cbc.Decrypt(d.nodesEnc)
	if err != nil {
		panic(&fPanic{Mode: ModeDirect, Err: err})
	}
	memory.Padding()
	var nodes []*Node
	err = msgpack.Unmarshal(b, &nodes)
	if err != nil {
		panic(&fPanic{Mode: ModeDirect, Err: err})
	}
	security.FlushBytes(b)
	return nodes, nil
}
