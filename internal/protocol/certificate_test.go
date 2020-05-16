package protocol

import (
	"bytes"
	"crypto/sha256"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/aes"
	"project/internal/crypto/ed25519"
	"project/internal/crypto/rand"
	"project/internal/guid"
	"project/internal/patch/monkey"
)

func TestIssueCertificate(t *testing.T) {
	// generate Controller private key
	privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	nodeGUID := guid.GUID{}
	err = nodeGUID.Write(bytes.Repeat([]byte{2}, guid.Size))
	require.NoError(t, err)

	// issue a Node certificate
	cert := Certificate{
		GUID:      nodeGUID,
		PublicKey: bytes.Repeat([]byte{3}, ed25519.PublicKeySize),
	}
	err = IssueCertificate(&cert, privateKey)
	require.NoError(t, err)

	// print certificate signature
	require.Len(t, cert.Signatures[0], ed25519.SignatureSize)
	require.Len(t, cert.Signatures[1], ed25519.SignatureSize)
	t.Log("signature:", cert.Signatures)

	t.Run("invalid public key size", func(t *testing.T) {
		cert := Certificate{GUID: nodeGUID}
		err = IssueCertificate(&cert, nil)
		require.EqualError(t, err, "invalid public key size")
	})
}

func TestCertificate_VerifySignature(t *testing.T) {
	// generate Controller private key
	privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	nodeGUID := guid.GUID{}
	err = nodeGUID.Write(bytes.Repeat([]byte{2}, guid.Size))
	require.NoError(t, err)

	// issue a Node certificate
	cert := Certificate{
		GUID:      nodeGUID,
		PublicKey: bytes.Repeat([]byte{3}, ed25519.PublicKeySize),
	}
	err = IssueCertificate(&cert, privateKey)
	require.NoError(t, err)

	// encode and decode
	certBytes := cert.Encode()
	require.NoError(t, err)
	recvCert := Certificate{}
	err = recvCert.Decode(nil)
	require.EqualError(t, err, "invalid certificate size")
	err = recvCert.Decode(certBytes)
	require.NoError(t, err)
	require.Equal(t, cert, recvCert)

	t.Run("with node guid", func(t *testing.T) {
		ok := recvCert.VerifySignatureWithNodeGUID(privateKey.PublicKey())
		require.True(t, ok)
	})

	t.Run("with controller guid", func(t *testing.T) {
		ok := recvCert.VerifySignatureWithCtrlGUID(privateKey.PublicKey())
		require.True(t, ok)
	})
}

func TestVerifyCertificate(t *testing.T) {
	// generate Controller private key
	ctrlPrivateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)

	// generate Node private key
	nodePrivateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	nodeGUID := new(guid.GUID)
	err = nodeGUID.Write(bytes.Repeat([]byte{2}, guid.Size))
	require.NoError(t, err)

	// issue a Node certificate
	cert := Certificate{
		GUID:      *nodeGUID,
		PublicKey: nodePrivateKey.PublicKey(),
	}
	err = IssueCertificate(&cert, ctrlPrivateKey)
	require.NoError(t, err)
	certBytes := cert.Encode()
	require.NoError(t, err)

	wg := sync.WaitGroup{}

	serverAck := func(conn net.Conn) {
		defer wg.Done()

		// send certificate
		_, err := conn.Write(certBytes)
		require.NoError(t, err)
		// receive role challenge
		challenge := make([]byte, ChallengeSize)
		_, err = io.ReadFull(conn, challenge)
		require.NoError(t, err)
		// signature challenge (ha ha, remember check challenge size)
		signature := ed25519.Sign(nodePrivateKey, challenge)
		_, err = conn.Write(signature)
		require.NoError(t, err)
	}

	t.Run("with node guid", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			err := server.Close()
			require.NoError(t, err)
			err = client.Close()
			require.NoError(t, err)
		}()

		wg.Add(1)
		go serverAck(server)

		publicKey := ctrlPrivateKey.PublicKey()
		cert, ok, err := VerifyCertificate(client, publicKey, nodeGUID)
		require.NoError(t, err)
		require.True(t, ok)
		require.NotNil(t, cert)
	})

	t.Run("with controller guid", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			err := server.Close()
			require.NoError(t, err)
			err = client.Close()
			require.NoError(t, err)
		}()

		wg.Add(1)
		go serverAck(server)

		publicKey := ctrlPrivateKey.PublicKey()
		cert, ok, err := VerifyCertificate(client, publicKey, CtrlGUID)
		require.NoError(t, err)
		require.True(t, ok)
		require.NotNil(t, cert)
	})

	t.Run("skip verify", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			err := server.Close()
			require.NoError(t, err)
			err = client.Close()
			require.NoError(t, err)
		}()

		wg.Add(1)
		go serverAck(server)

		cert, ok, err := VerifyCertificate(client, nil, nil)
		require.NoError(t, err)
		require.True(t, ok)
		require.NotNil(t, cert)
	})

	t.Run("failed to receive certificate", func(t *testing.T) {
		server, client := net.Pipe()
		err := server.Close()
		require.NoError(t, err)
		defer func() {
			err := client.Close()
			require.NoError(t, err)
		}()

		cert, ok, err := VerifyCertificate(client, nil, nil)
		require.NoError(t, err)
		require.False(t, ok)
		require.Nil(t, cert)
	})

	t.Run("different node guid", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			err := server.Close()
			require.NoError(t, err)
			err = client.Close()
			require.NoError(t, err)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := server.Write(bytes.Repeat([]byte{0}, CertificateSize))
			require.NoError(t, err)
		}()

		cert, ok, err := VerifyCertificate(client, nil, nodeGUID)
		require.EqualError(t, err, "different guid in certificate")
		require.False(t, ok)
		require.NotNil(t, cert)
	})

	t.Run("invalid certificate signature", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			err := server.Close()
			require.NoError(t, err)
			err = client.Close()
			require.NoError(t, err)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			cert := make([]byte, CertificateSize)
			copy(cert, nodeGUID[:])
			_, err := server.Write(cert)
			require.NoError(t, err)
		}()

		publicKey := ctrlPrivateKey.PublicKey()
		cert, ok, err := VerifyCertificate(client, publicKey, nodeGUID)
		require.EqualError(t, err, "invalid certificate signature")
		require.False(t, ok)
		require.NotNil(t, cert)
	})

	t.Run("failed to generate challenge", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			err := server.Close()
			require.NoError(t, err)
			err = client.Close()
			require.NoError(t, err)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := server.Write(certBytes)
			require.NoError(t, err)
		}()

		// patch
		patch := func(interface{}, []byte) (int, error) {
			return 0, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(rand.Reader, "Read", patch)
		defer pg.Unpatch()

		publicKey := ctrlPrivateKey.PublicKey()
		cert, ok, err := VerifyCertificate(client, publicKey, nodeGUID)
		require.NoError(t, err)
		require.False(t, ok)
		require.NotNil(t, cert)
	})

	t.Run("failed to send challenge", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			err := client.Close()
			require.NoError(t, err)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := server.Write(certBytes)
			require.NoError(t, err)
			err = server.Close()
			require.NoError(t, err)
		}()

		publicKey := ctrlPrivateKey.PublicKey()
		cert, ok, err := VerifyCertificate(client, publicKey, nodeGUID)
		require.NoError(t, err)
		require.False(t, ok)
		require.NotNil(t, cert)
	})

	t.Run("failed to receive challenge signature", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			err := client.Close()
			require.NoError(t, err)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := server.Write(certBytes)
			require.NoError(t, err)
			// read challenge
			challenge := make([]byte, ChallengeSize)
			_, err = io.ReadFull(server, challenge)
			require.NoError(t, err)
			err = server.Close()
			require.NoError(t, err)
		}()

		publicKey := ctrlPrivateKey.PublicKey()
		cert, ok, err := VerifyCertificate(client, publicKey, nodeGUID)
		require.NoError(t, err)
		require.False(t, ok)
		require.NotNil(t, cert)
	})

	t.Run("invalid challenge signature", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			err := client.Close()
			require.NoError(t, err)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := server.Write(certBytes)
			require.NoError(t, err)
			// read challenge
			challenge := make([]byte, ChallengeSize)
			_, err = io.ReadFull(server, challenge)
			require.NoError(t, err)
			_, err = server.Write(bytes.Repeat([]byte{0}, ed25519.SignatureSize))
			require.NoError(t, err)
		}()

		publicKey := ctrlPrivateKey.PublicKey()
		cert, ok, err := VerifyCertificate(client, publicKey, nodeGUID)
		require.EqualError(t, err, "invalid challenge signature")
		require.False(t, ok)
		require.NotNil(t, cert)
	})

	wg.Wait()
}

func TestUpdateNodeRequest(t *testing.T) {
	nodeGUID := guid.GUID{}
	err := nodeGUID.Write(bytes.Repeat([]byte{1}, guid.Size))
	require.NoError(t, err)

	rawR := new(UpdateNodeRequest)
	rawR.GUID = nodeGUID
	rawR.Hash = bytes.Repeat([]byte{2}, sha256.Size)
	rawR.EncData = bytes.Repeat([]byte{3}, guid.Size+ed25519.PublicKeySize+aes.BlockSize)
	err = rawR.Validate()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	rawR.Pack(buf)

	newR := NewUpdateNodeRequest()
	err = newR.Unpack(buf.Bytes())
	require.NoError(t, err)

	require.Equal(t, rawR, newR)

	t.Run("Unpack", func(t *testing.T) {
		err = newR.Unpack(nil)
		require.Error(t, err)
	})

	t.Run("Validate", func(t *testing.T) {
		rawR.Hash = nil
		err := rawR.Validate()
		require.Error(t, err)

		rawR.Hash = bytes.Repeat([]byte{1}, sha256.Size)
		rawR.EncData = nil
		err = rawR.Validate()
		require.Error(t, err)
	})
}

func TestUpdateNodeResponse(t *testing.T) {
	rawR := new(UpdateNodeResponse)
	rawR.Hash = bytes.Repeat([]byte{1}, sha256.Size)
	rawR.EncData = bytes.Repeat([]byte{2}, aes.BlockSize)
	err := rawR.Validate()
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	rawR.Pack(buf)

	newR := NewUpdateNodeResponse()
	err = newR.Unpack(buf.Bytes())
	require.NoError(t, err)

	require.Equal(t, rawR, newR)

	t.Run("Unpack", func(t *testing.T) {
		err = newR.Unpack(nil)
		require.Error(t, err)
	})

	t.Run("Validate", func(t *testing.T) {
		rawR.Hash = nil
		err := rawR.Validate()
		require.Error(t, err)

		rawR.Hash = bytes.Repeat([]byte{1}, sha256.Size)
		rawR.EncData = nil
		err = rawR.Validate()
		require.Error(t, err)
	})
}
