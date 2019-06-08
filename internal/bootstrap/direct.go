package bootstrap

import (
	"sync"

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
	rwmutex   sync.RWMutex
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
	defer this.rwmutex.RUnlock()
	this.rwmutex.RLock()
	nodes := &struct {
		Nodes []*Node
	}{}
	copy(nodes.Nodes, this.nodes)
	return toml.Marshal(nodes)
}

func (this *Direct) Unmarshal(data []byte) error {
	defer this.rwmutex.Unlock()
	this.rwmutex.Lock()
	nodes := &struct {
		Nodes []*Node
	}{}
	err := toml.Unmarshal(data, nodes)
	if err != nil {
		return err
	}
	b, err := msgpack.Marshal(nodes.Nodes)
	if err != nil {
		return err
	}
	memory := security.New_Memory()
	defer memory.Flush()
	rand := random.New()
	this.cryptor, err = aes.New_CBC_Cryptor(rand.Bytes(32), rand.Bytes(aes.IV_SIZE))
	if err != nil {
		return err
	}
	memory.Padding()
	this.nodes_enc, err = this.cryptor.Encrypt(b)
	security.Flush_Bytes(b)
	return err
}

func (this *Direct) Resolve() ([]*Node, error) {
	defer this.rwmutex.RUnlock()
	this.rwmutex.RLock()
	b, err := this.cryptor.Decrypt(this.nodes_enc)
	if err != nil {
		return nil, err
	}
	var nodes []*Node
	err = msgpack.Unmarshal(b, &nodes)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}
