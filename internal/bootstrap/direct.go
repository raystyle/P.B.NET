package bootstrap

import (
	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/patch/msgpack"
	"project/internal/patch/toml"
	"project/internal/random"
	"project/internal/security"
)

// Direct is used to resolve bootstrap node listeners from local config.
type Direct struct {
	Listeners []*Listener `toml:"listeners"`

	// self encrypt all options
	cbc *aes.CBC
	enc []byte
}

// NewDirect is used to create a direct mode bootstrap.
func NewDirect() *Direct {
	return new(Direct)
}

// Validate is a padding function.
func (d *Direct) Validate() error {
	return nil
}

// Marshal is used to marshal Direct to []byte.
func (d *Direct) Marshal() ([]byte, error) {
	if len(d.Listeners) == 0 {
		return nil, errors.New("no bootstrap node listeners")
	}
	return toml.Marshal(d)
}

// Unmarshal is used to unmarshal []byte to Direct.
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
	listenerData, _ := msgpack.Marshal(d.Listeners)
	defer security.CoverBytes(listenerData)
	memory.Padding()
	d.enc, err = d.cbc.Encrypt(listenerData)
	return err
}

// Resolve is used to get bootstrap node listeners.
func (d *Direct) Resolve() ([]*Listener, error) {
	memory := security.NewMemory()
	defer memory.Flush()
	dec, err := d.cbc.Decrypt(d.enc)
	defer security.CoverBytes(dec)
	if err != nil {
		panic(err)
	}
	memory.Padding()
	var listeners []*Listener
	err = msgpack.Unmarshal(dec, &listeners)
	if err != nil {
		panic(err)
	}
	return EncryptListeners(listeners), nil
}
