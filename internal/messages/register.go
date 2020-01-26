package messages

import (
	"errors"
	"time"

	"project/internal/bootstrap"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/module/info"
	"project/internal/protocol"
)

// Bootstrap contains tag, mode and configuration
type Bootstrap struct {
	Tag    string
	Mode   string
	Config []byte
}

// about register result
const (
	RegisterResultAccept uint8 = iota + 1
	RegisterResultRefused
	RegisterResultTimeout
)

// about register error
var (
	ErrRegisterRefused       = errors.New("register refused")
	ErrRegisterTimeout       = errors.New("register timeout")
	ErrRegisterUnknownResult = errors.New("unknown register result")
)

// NodeRegisterRequest is used to Node register,
// controller trust node also use it
type NodeRegisterRequest struct {
	GUID         []byte // Node GUID
	PublicKey    []byte
	KexPublicKey []byte // key exchange
	SystemInfo   *info.System
	RequestTime  time.Time
}

// Validate is used to validate request fields
func (r *NodeRegisterRequest) Validate() error {
	if len(r.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(r.KexPublicKey) != 32 {
		return errors.New("invalid key exchange public key size")
	}
	if r.SystemInfo == nil {
		return errors.New("empty system info")
	}
	return nil
}

// NodeRegisterResponse is used to return Node register response
type NodeRegisterResponse struct {
	GUID []byte // Node GUID

	// all node save it
	PublicKey    []byte
	KexPublicKey []byte

	RequestTime time.Time
	ReplyTime   time.Time

	Result      uint8
	Certificate []byte

	// key = hex(node guid)
	// a part of all node listeners
	Listeners map[string][]*bootstrap.Listener
}

// Validate is used to validate response fields
func (r *NodeRegisterResponse) Validate() error {
	if len(r.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(r.KexPublicKey) != 32 {
		return errors.New("invalid key exchange public key size")
	}
	if r.Result < RegisterResultAccept || r.Result > RegisterResultTimeout {
		return errors.New("unknown node register result")
	}
	if len(r.Certificate) != protocol.CertificateSize {
		return errors.New("invalid certificate size")
	}
	return nil
}

// BeaconRegisterRequest is used to Beacon register
type BeaconRegisterRequest struct {
	GUID         []byte // Beacon GUID
	PublicKey    []byte
	KexPublicKey []byte // key exchange
	SystemInfo   *info.System
	RequestTime  time.Time
}

// Validate is used to validate request fields
func (r *BeaconRegisterRequest) Validate() error {
	if len(r.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(r.KexPublicKey) != 32 {
		return errors.New("invalid key exchange public key size")
	}
	if r.SystemInfo == nil {
		return errors.New("empty system info")
	}
	return nil
}

// BeaconRegisterResponse is used to return Beacon register response
type BeaconRegisterResponse struct {
	GUID []byte // Beacon GUID

	// all node save it
	PublicKey    []byte
	KexPublicKey []byte

	RequestTime time.Time
	ReplyTime   time.Time

	Result uint8

	// key = hex(node guid)
	// a part of all node listeners
	Listeners map[string][]*bootstrap.Listener // key = hex(node guid)
}

// Validate is used to validate response fields
func (r *BeaconRegisterResponse) Validate() error {
	if len(r.GUID) != guid.Size {
		return errors.New("invalid guid size")
	}
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(r.KexPublicKey) != 32 {
		return errors.New("invalid key exchange public key size")
	}
	if r.Result < RegisterResultAccept || r.Result > RegisterResultTimeout {
		return errors.New("unknown beacon register result")
	}
	return nil
}
