package protocol

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net"

	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/rand"
	"project/internal/guid"
)

// size about certificate and challenge.
const (
	CertificateSize = guid.Size + ed25519.PublicKeySize + 2*ed25519.SignatureSize
	ChallengeSize   = 32
)

// ErrDifferentNodeGUID is an error about certificate.
var ErrDifferentNodeGUID = fmt.Errorf("different guid in certificate")

// Certificate is used to verify Node.
type Certificate struct {
	GUID      guid.GUID // Node GUID
	PublicKey ed25519.PublicKey

	// with Node GUID: common connect node,
	// client known Node GUID.
	// with Controller GUID: role register, Controller trust node...
	// that client don't known Node GUID.
	Signatures [2][]byte
}

// Encode is used to encode certificate to bytes.
func (cert *Certificate) Encode() []byte {
	b := make([]byte, CertificateSize)
	offset := 0
	copy(b, cert.GUID[:])
	offset += guid.Size
	copy(b[offset:], cert.PublicKey)
	offset += ed25519.PublicKeySize
	copy(b[offset:], cert.Signatures[0])
	offset += ed25519.SignatureSize
	copy(b[offset:], cert.Signatures[1])
	return b
}

// Decode is used to decode bytes to certificate.
func (cert *Certificate) Decode(b []byte) error {
	if len(b) != CertificateSize {
		return errors.New("invalid certificate size")
	}
	// initialize slice
	cert.PublicKey = make([]byte, ed25519.PublicKeySize)
	cert.Signatures[0] = make([]byte, ed25519.SignatureSize)
	cert.Signatures[1] = make([]byte, ed25519.SignatureSize)
	// copy
	begin := 0
	end := guid.Size
	copy(cert.GUID[:], b[begin:end])
	begin = end
	end = begin + ed25519.PublicKeySize
	copy(cert.PublicKey, b[begin:end])
	begin = end
	end = begin + ed25519.SignatureSize
	copy(cert.Signatures[0], b[begin:end])
	copy(cert.Signatures[1], b[end:])
	return nil
}

// VerifySignatureWithNodeGUID is used to verify Node's
// certificate signature with Node GUID.
func (cert *Certificate) VerifySignatureWithNodeGUID(pub ed25519.PublicKey) bool {
	msg := make([]byte, guid.Size+ed25519.PublicKeySize)
	copy(msg, cert.GUID[:])
	copy(msg[guid.Size:], cert.PublicKey)
	return ed25519.Verify(pub, msg, cert.Signatures[0])
}

// VerifySignatureWithCtrlGUID is used to verify Node's
// certificate signature with Controller GUID.
func (cert *Certificate) VerifySignatureWithCtrlGUID(pub ed25519.PublicKey) bool {
	msg := make([]byte, guid.Size+ed25519.PublicKeySize)
	copy(msg, CtrlGUID[:])
	copy(msg[guid.Size:], cert.PublicKey)
	return ed25519.Verify(pub, msg, cert.Signatures[1])
}

// IssueCertificate is used to issue Node certificate,
// use Controller's private key to sign generated certificate.
func IssueCertificate(cert *Certificate, pri ed25519.PrivateKey) error {
	if len(cert.PublicKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}
	msg := make([]byte, guid.Size+ed25519.PublicKeySize)
	// sign with Node GUID
	copy(msg, cert.GUID[:])
	copy(msg[guid.Size:], cert.PublicKey)
	cert.Signatures[0] = ed25519.Sign(pri, msg)
	// sign with Controller GUID
	copy(msg, CtrlGUID[:])
	cert.Signatures[1] = ed25519.Sign(pri, msg)
	return nil
}

// VerifyCertificate is used to verify Node certificate.
// if errors != nil, role must log with level Exploit.
func VerifyCertificate(
	conn net.Conn,
	pub ed25519.PublicKey,
	guid *guid.GUID,
) (*Certificate, bool, error) {
	// receive node's certificate
	buf := make([]byte, CertificateSize)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return nil, false, nil
	}
	cert := Certificate{}
	_ = cert.Decode(buf) // no error
	// if guid == nil, skip verify
	if guid != nil {
		// verify certificate signature
		var ok bool
		if *CtrlGUID == *guid {
			ok = cert.VerifySignatureWithCtrlGUID(pub)
		} else {
			// compare Node GUID
			if cert.GUID != *guid {
				return &cert, false, errors.WithStack(ErrDifferentNodeGUID)
			}
			ok = cert.VerifySignatureWithNodeGUID(pub)
		}
		if !ok {
			return &cert, false, errors.New("invalid certificate signature")
		}
	}
	// send challenge to verify public key
	challenge := buf[ed25519.SignatureSize : ed25519.SignatureSize+ChallengeSize]
	_, err = io.ReadFull(rand.Reader, challenge)
	if err != nil {
		return &cert, false, nil
	}
	_, err = conn.Write(challenge)
	if err != nil {
		return &cert, false, nil
	}
	// receive challenge signature
	signature := buf[:ed25519.SignatureSize]
	_, err = io.ReadFull(conn, signature)
	if err != nil {
		return &cert, false, nil
	}
	if !ed25519.Verify(cert.PublicKey, challenge, signature) {
		return &cert, false, errors.New("invalid challenge signature")
	}
	return &cert, true, nil
}

// size about update node request and response.
const (
	UpdateNodeRequestSize  = 2*guid.Size + sha256.Size + ed25519.PublicKeySize + aes.BlockSize
	UpdateNodeResponseSize = sha256.Size + aes.BlockSize
)

// role update node response
const (
	UpdateNodeResponseOK byte = 1 + iota
	UpdateNodeResponseNotExist
	UpdateNodeResponseIncorrectPublicKey
)

// UpdateNodeRequest Beacon will use it to query from Controller that
// this Node is updated(like restart a Node, Listener is same, but Node
// GUID is changed, Beacon will update).
type UpdateNodeRequest struct {
	GUID    guid.GUID // Beacon or Node GUID
	Hash    []byte    // HMAC-SHA256 GUID + raw data
	EncData []byte    // use AES to encrypt it, NodeGUID + PublicKey
}

// NewUpdateNodeRequest is used to create UpdateNodeRequest, Unpack() need it.
func NewUpdateNodeRequest() *UpdateNodeRequest {
	return &UpdateNodeRequest{
		Hash:    make([]byte, sha256.Size),
		EncData: make([]byte, guid.Size+ed25519.PublicKeySize+aes.BlockSize),
	}
}

// Pack is used to pack UpdateNodeRequest to *bytes.Buffer.
func (unr *UpdateNodeRequest) Pack(buf *bytes.Buffer) {
	buf.Write(unr.GUID[:])
	buf.Write(unr.Hash)
	buf.Write(unr.EncData)
}

// Unpack is used to unpack []byte to UpdateNodeRequest.
func (unr *UpdateNodeRequest) Unpack(data []byte) error {
	if len(data) != UpdateNodeRequestSize {
		return errors.New("invalid UpdateNodeRequest packet size")
	}
	copy(unr.GUID[:], data[:guid.Size])
	copy(unr.Hash, data[guid.Size:guid.Size+sha256.Size])
	copy(unr.EncData, data[guid.Size+sha256.Size:])
	return nil
}

// Validate is used to check size about fields.
func (unr *UpdateNodeRequest) Validate() error {
	if len(unr.Hash) != sha256.Size {
		return errors.New("invalid hmac hash size")
	}
	if len(unr.EncData) != guid.Size+ed25519.PublicKeySize+aes.BlockSize {
		return errors.New("invalid encrypted data size")
	}
	return nil
}

// UpdateNodeResponse Controller will send the response to the Node that
// send UpdateNodeRequest, then Node will send the response to Beacon.
type UpdateNodeResponse struct {
	Hash    []byte // HMAC-SHA256
	EncData []byte // use AES to encrypt it, 1 useful byte + 7 useless bytes
}

// NewUpdateNodeResponse is used to create UpdateNodeResponse, Unpack() need it.
func NewUpdateNodeResponse() *UpdateNodeResponse {
	return &UpdateNodeResponse{
		Hash:    make([]byte, sha256.Size),
		EncData: make([]byte, aes.BlockSize),
	}
}

// Pack is used to pack UpdateNodeResponse to *bytes.Buffer.
func (unr *UpdateNodeResponse) Pack(buf *bytes.Buffer) {
	buf.Write(unr.Hash)
	buf.Write(unr.EncData)
}

// Unpack is used to unpack []byte to UpdateNodeResponse.
func (unr *UpdateNodeResponse) Unpack(data []byte) error {
	if len(data) != UpdateNodeResponseSize {
		return errors.New("invalid UpdateNodeResponse packet size")
	}
	copy(unr.Hash, data[:sha256.Size])
	copy(unr.EncData, data[sha256.Size:])
	return nil
}

// Validate is used to check size about fields.
func (unr *UpdateNodeResponse) Validate() error {
	if len(unr.Hash) != sha256.Size {
		return errors.New("invalid hmac hash size")
	}
	if len(unr.EncData) != aes.BlockSize {
		return errors.New("invalid encrypted data size")
	}
	return nil
}
