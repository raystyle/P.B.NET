package messages

import (
	"bytes"
	"errors"
	"time"

	"project/internal/crypto/curve25519"
	"project/internal/crypto/ed25519"
	"project/internal/guid"
	"project/internal/option"
)

// MaxQueryWaitTime is the time that Node will wait Controller.
const MaxQueryWaitTime = 15 * time.Second

var (
	// ZeroGUID means Controller can't query the target Node or Beacon key.
	ZeroGUID = guid.GUID{}

	// ZeroPublicKey is used to padding.
	ZeroPublicKey = bytes.Repeat([]byte{0}, ed25519.PublicKeySize)

	// ZeroKexPublicKey is used to padding.
	ZeroKexPublicKey = bytes.Repeat([]byte{0}, curve25519.ScalarSize)
)

// QueryNodeKey is used to query Node key from Controller.
type QueryNodeKey struct {
	ID   guid.GUID
	GUID guid.GUID // Node GUID
	Time time.Time
}

// SetID is used to set message id.
func (qnk *QueryNodeKey) SetID(id *guid.GUID) {
	qnk.ID = *id
}

// AnswerNodeKey is used to answer to Node about queried Node key.
type AnswerNodeKey struct {
	ID           guid.GUID // QueryNodeKey.ID
	GUID         guid.GUID // Node GUID, if not exists guid will be ZeroGUID.
	PublicKey    []byte
	KexPublicKey []byte
	ReplyTime    time.Time // Controller reply time
}

// Validate is used to validate answer fields.
func (ank *AnswerNodeKey) Validate() error {
	if len(ank.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(ank.KexPublicKey) != curve25519.ScalarSize {
		return errors.New("invalid key exchange public key size")
	}
	return nil
}

// QueryBeaconKey is used to query Beacon key from Controller.
type QueryBeaconKey struct {
	ID   guid.GUID
	GUID guid.GUID // Beacon GUID
	Time time.Time
}

// SetID is used to set message id.
func (qbk *QueryBeaconKey) SetID(id *guid.GUID) {
	qbk.ID = *id
}

// AnswerBeaconKey is used to answer to Node about queried Beacon key.
type AnswerBeaconKey struct {
	ID           guid.GUID // QueryBeaconKey.ID
	GUID         guid.GUID // Beacon GUID, if not exists guid will be ZeroGUID.
	PublicKey    []byte
	KexPublicKey []byte
	ReplyTime    time.Time // Controller reply time
}

// Validate is used to validate answer fields.
func (abk *AnswerBeaconKey) Validate() error {
	if len(abk.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	if len(abk.KexPublicKey) != curve25519.ScalarSize {
		return errors.New("invalid key exchange public key size")
	}
	return nil
}

// Listener is used to listen a listener to a Node.
type Listener struct {
	Tag       string
	Mode      string
	Network   string
	Address   string
	Timeout   time.Duration
	TLSConfig option.TLSConfig
}
