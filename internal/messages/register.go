package messages

import (
	"errors"
	"time"

	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/modules/info"
	"project/internal/xnet"
)

type RegisterResult = uint8

const (
	RegisterAccept RegisterResult = iota
	RegisterRefused
	RegisterTimeout
)

var (
	RegisterSucceed = []byte("ok")
)

type Listener struct {
	Tag    string
	Mode   xnet.Mode
	Config []byte // xnet.Config
}

type NodeRegisterRequest struct {
	GUID         []byte
	PublicKey    []byte
	KexPublicKey []byte // key exchange
	SystemInfo   *info.System
	RequestTime  time.Time
}

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

type NodeRegisterResponse struct {
	GUID         []byte
	PublicKey    []byte // verify message
	KexPublicKey []byte // aes key exchange
	Result       RegisterResult
	Certificates []byte
	Listeners    []*Listener
	RequestTime  time.Time
	ReplyTime    time.Time
}

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
	if r.Result > RegisterRefused {
		return errors.New("invalid result")
	}
	// see controller/certificate.go CTRL.issueCertificate()
	if len(r.Certificates) != 2*(2+ed25519.SignatureSize) {
		return errors.New("invalid certificate size")
	}
	return nil
}

type BeaconRegisterRequest struct {
	GUID         []byte
	PublicKey    []byte
	KexPublicKey []byte // key exchange
	SystemInfo   *info.System
	RequestTime  time.Time
}

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

type BeaconRegisterResponse struct {
	GUID         []byte
	PublicKey    []byte // verify message
	KexPublicKey []byte // aes key exchange
	Result       RegisterResult
	RequestTime  time.Time
	ReplyTime    time.Time
}

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
	if r.Result > RegisterRefused {
		return errors.New("invalid result")
	}
	return nil
}
