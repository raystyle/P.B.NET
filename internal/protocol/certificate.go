package protocol

import (
	"io"
	"net"

	"github.com/pkg/errors"

	"project/internal/crypto/ed25519"
	"project/internal/crypto/rand"
	"project/internal/guid"
)

// certificate and challenge size
const (
	CertificateSize = guid.Size + ed25519.PublicKeySize + 2*ed25519.SignatureSize
	ChallengeSize   = 32
)

// Certificate is used to verify node
type Certificate struct {
	GUID      guid.GUID // node GUID
	PublicKey ed25519.PublicKey

	// with Node GUID: common connect node,
	// client known Node GUID.
	// with Controller GUID: role register, Controller trust node...
	// that client don't known Node GUID.
	Signatures [2][]byte
}

// Encode is used to encode certificate to bytes
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

// Decode is used to decode bytes to certificate
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

// VerifySignatureWithNodeGUID is used to verify node
// certificate signature with Node GUID
func (cert *Certificate) VerifySignatureWithNodeGUID(pub ed25519.PublicKey) bool {
	msg := make([]byte, guid.Size+ed25519.PublicKeySize)
	copy(msg, cert.GUID[:])
	copy(msg[guid.Size:], cert.PublicKey)
	return ed25519.Verify(pub, msg, cert.Signatures[0])
}

// VerifySignatureWithCtrlGUID is used to verify node
// certificate signature with Controller GUID
func (cert *Certificate) VerifySignatureWithCtrlGUID(pub ed25519.PublicKey) bool {
	msg := make([]byte, guid.Size+ed25519.PublicKeySize)
	copy(msg, CtrlGUID[:])
	copy(msg[guid.Size:], cert.PublicKey)
	return ed25519.Verify(pub, msg, cert.Signatures[1])
}

// IssueCertificate is used to issue node certificate,
// use controller private key to sign generated certificate
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

// VerifyCertificate is used to verify node certificate
// if errors != nil, role must log with level Exploit
func VerifyCertificate(conn net.Conn, pub ed25519.PublicKey, guid *guid.GUID) (bool, error) {
	// receive node certificate
	buf := make([]byte, CertificateSize)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return false, nil
	}
	var cert Certificate
	_ = cert.Decode(buf) // no error
	// if guid == nil, skip verify
	if guid != nil {
		// verify certificate signature
		var ok bool
		if *CtrlGUID == *guid {
			ok = cert.VerifySignatureWithCtrlGUID(pub)
		} else {
			// verify Node GUID
			if cert.GUID != *guid {
				return false, errors.New("guid in certificate is different")
			}
			ok = cert.VerifySignatureWithNodeGUID(pub)
		}
		if !ok {
			return false, errors.New("invalid certificate signature")
		}
	}
	// send challenge to verify public key
	challenge := buf[ed25519.SignatureSize : ed25519.SignatureSize+ChallengeSize]
	_, err = io.ReadFull(rand.Reader, challenge)
	if err != nil {
		return false, nil
	}
	_, err = conn.Write(challenge)
	if err != nil {
		return false, nil
	}
	// receive challenge signature
	signature := buf[:ed25519.SignatureSize]
	_, err = io.ReadFull(conn, signature)
	if err != nil {
		return false, nil
	}
	if !ed25519.Verify(cert.PublicKey, challenge, signature) {
		return false, errors.New("invalid challenge signature")
	}
	return true, nil
}
