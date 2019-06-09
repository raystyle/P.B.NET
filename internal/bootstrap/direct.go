package bootstrap

import (
	"fmt"

	"github.com/pelletier/go-toml"
	"github.com/vmihailenco/msgpack"

	"project/internal/crypto/aes"
	"project/internal/random"
	"project/internal/security"
)

type Direct struct {
	nodes     []*Node
	nodes_enc []byte
	cryptor   *aes.CBC_Cryptor
}

func New_Direct(n []*Node) *Direct {
	d := &Direct{nodes: make([]*Node, len(n))}
	copy(d.nodes, n)
	return d
}

func (this *Direct) Generate(_ []*Node) (string, error) {
	return "", nil
}

func (this *Direct) Marshal() ([]byte, error) {
	nodes := &struct {
		Nodes []*Node
	}{}
	nodes.Nodes = make([]*Node, len(this.nodes))
	copy(nodes.Nodes, this.nodes)
	return toml.Marshal(nodes)
}

func (this *Direct) Unmarshal(data []byte) error {
	nodes := &struct {
		Nodes []*Node
	}{}
	err := toml.Unmarshal(data, nodes)
	if err != nil {
		return err
	}
	memory := security.New_Memory()
	defer memory.Flush()
	rand := random.New()
	memory.Padding()
	key := rand.Bytes(32)
	iv := rand.Bytes(aes.IV_SIZE)
	this.cryptor, err = aes.New_CBC_Cryptor(key, iv)
	if err != nil {
		panic(fmt.Errorf("internal error: %s", err))
	}
	security.Flush_Bytes(key)
	security.Flush_Bytes(iv)
	b, err := msgpack.Marshal(&nodes.Nodes)
	if err != nil {
		panic(fmt.Errorf("internal error: %s", err))
	}
	memory.Padding()
	this.nodes_enc, err = this.cryptor.Encrypt(b)
	if err != nil {
		panic(fmt.Errorf("internal error: %s", err))
	}
	security.Flush_Bytes(b)
	return nil
}

func (this *Direct) Resolve() ([]*Node, error) {
	memory := security.New_Memory()
	defer memory.Flush()
	b, err := this.cryptor.Decrypt(this.nodes_enc)
	if err != nil {
		return nil, err
	}
	memory.Padding()
	var nodes []*Node
	err = msgpack.Unmarshal(b, &nodes)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}
