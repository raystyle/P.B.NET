package messages

import (
	"errors"
	"time"

	"project/internal/crypto/aes"
	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/module/info"
	"project/internal/protocol"
)

// MaxRegisterWaitTime is the role max wait time, include Controller action timeout,
// Node wait role register response, Beacon read result timeout.
const MaxRegisterWaitTime = 30 * time.Second

// about role register result
const (
	RegisterResultAccept uint8 = iota + 1
	RegisterResultRefused
	RegisterResultTimeout
)

// Bootstrap contains tag, mode and configuration.
type Bootstrap struct {
	Tag    string
	Mode   string
	Config []byte
}

// about register error
var (
	ErrRegisterRefused       = errors.New("register refused")
	ErrRegisterTimeout       = errors.New("register timeout")
	ErrRegisterUnknownResult = errors.New("unknown register result")
)

// NodeRegisterRequest is used to Node register, controller trust node also use it.
type NodeRegisterRequest struct {
	GUID         guid.GUID // Node GUID
	PublicKey    []byte
	KexPublicKey []byte // key exchange
	ConnAddress  string // usually like "1.2.3.4:5678"
	SystemInfo   *info.System
	RequestTime  time.Time
}

// Validate is used to validate request fields
func (r *NodeRegisterRequest) Validate() error {
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(r.KexPublicKey) != curve25519.ScalarSize {
		return errors.New("invalid key exchange public key size")
	}
	if r.SystemInfo == nil {
		return errors.New("empty system information")
	}
	return nil
}

// NodeRegisterResponse is used to return Node register response.
type NodeRegisterResponse struct {
	ID   guid.GUID // EncryptedRegisterRequest.ID
	GUID guid.GUID // Node GUID

	// all Nodes will save it to storage
	PublicKey    []byte
	KexPublicKey []byte

	RequestTime time.Time
	ReplyTime   time.Time

	Result      uint8
	Certificate []byte

	// type = map[guid.GUID][]*bootstrap.Listener
	// It encrypted by session key.
	// It contains a part of all Node listeners,
	// Node will connect these listeners first.
	NodeListeners []byte
}

// Validate is used to validate response fields.
func (r *NodeRegisterResponse) Validate() error {
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(r.KexPublicKey) != curve25519.ScalarSize {
		return errors.New("invalid key exchange public key size")
	}
	if r.Result < RegisterResultAccept || r.Result > RegisterResultTimeout {
		return errors.New("unknown register result")
	}
	if len(r.Certificate) != protocol.CertificateSize {
		return errors.New("invalid certificate size")
	}
	return nil
}

// BeaconRegisterRequest is used to Beacon register.
type BeaconRegisterRequest struct {
	GUID         guid.GUID // Beacon GUID
	PublicKey    []byte
	KexPublicKey []byte // key exchange
	ConnAddress  string // usually like "1.2.3.4:5678"
	SystemInfo   *info.System
	SleepFixed   uint // second, query sleep
	SleepRandom  uint // second, query sleep
	RequestTime  time.Time
}

// Validate is used to validate request fields.
func (r *BeaconRegisterRequest) Validate() error {
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(r.KexPublicKey) != curve25519.ScalarSize {
		return errors.New("invalid key exchange public key size")
	}
	if r.SystemInfo == nil {
		return errors.New("empty system info")
	}
	return nil
}

// BeaconRegisterResponse is used to return Beacon register response.
type BeaconRegisterResponse struct {
	ID   guid.GUID // EncryptedRegisterRequest.ID
	GUID guid.GUID // Beacon GUID

	// all Nodes will save it to storage
	PublicKey    []byte
	KexPublicKey []byte

	RequestTime time.Time
	ReplyTime   time.Time

	Result uint8

	// type = map[guid.GUID][]*bootstrap.Listener
	// It encrypted by session key.
	// It contains a part of all Node listeners,
	// Node will connect these listeners first.
	NodeListeners []byte
}

// Validate is used to validate response fields.
func (r *BeaconRegisterResponse) Validate() error {
	if len(r.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(r.KexPublicKey) != curve25519.ScalarSize {
		return errors.New("invalid key exchange public key size")
	}
	if r.Result < RegisterResultAccept || r.Result > RegisterResultTimeout {
		return errors.New("unknown register result")
	}
	return nil
}

// EncryptedRegisterRequest contains encrypted role register request.
// Node will send to Controller.
type EncryptedRegisterRequest struct {
	ID           guid.GUID // it will be set to Role register response
	KexPublicKey []byte    // role register request.KexPublicKey
	EncRequest   []byte    // encrypted Role.register.PackRequest()
}

// SetID is used to set message id.
func (r *EncryptedRegisterRequest) SetID(id *guid.GUID) {
	r.ID = *id
}

// Validate is used to validate request filed.
func (r *EncryptedRegisterRequest) Validate() error {
	if len(r.KexPublicKey) != curve25519.ScalarSize {
		return errors.New("invalid key exchange public key size")
	}
	if len(r.EncRequest) < aes.BlockSize {
		return errors.New("invalid encrypted request data size")
	}
	return nil
}
