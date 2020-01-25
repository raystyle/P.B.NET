package protocol

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/crypto/ed25519"
	"project/internal/crypto/rand"
	"project/internal/guid"
	"project/internal/testsuite"
)

func TestIssueCertificate(t *testing.T) {
	// generate Controller private key
	privateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	nodeGUID := bytes.Repeat([]byte{2}, guid.Size)

	// issue a Node certificate
	cert := Certificate{
		GUID:      nodeGUID,
		PublicKey: bytes.Repeat([]byte{3}, ed25519.PublicKeySize),
	}
	err = IssueCertificate(&cert, privateKey)
	require.NoError(t, err)

	// print certificate signature
	require.Equal(t, ed25519.SignatureSize, len(cert.Signatures[0]))
	require.Equal(t, ed25519.SignatureSize, len(cert.Signatures[1]))
	t.Log("signature:", cert.Signatures)

	t.Run("invalid guid size", func(t *testing.T) {
		err = IssueCertificate(new(Certificate), nil)
		require.EqualError(t, err, "invalid guid size")
	})

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
	nodeGUID := bytes.Repeat([]byte{2}, guid.Size)

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
		require.True(t, recvCert.VerifySignatureWithNodeGUID(privateKey.PublicKey()))
	})

	t.Run("with controller guid", func(t *testing.T) {
		require.True(t, recvCert.VerifySignatureWithCTRLGUID(privateKey.PublicKey()))
	})
}

func TestVerifyCertificate(t *testing.T) {
	// generate Controller private key
	ctrlPrivateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	// generate Node private key
	nodePrivateKey, err := ed25519.GenerateKey()
	require.NoError(t, err)
	nodeGUID := bytes.Repeat([]byte{2}, guid.Size)

	// issue a Node certificate
	cert := Certificate{
		GUID:      nodeGUID,
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
			require.NoError(t, server.Close())
			require.NoError(t, client.Close())
		}()
		wg.Add(1)
		go serverAck(server)
		ok, err := VerifyCertificate(client, ctrlPrivateKey.PublicKey(), nodeGUID)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("with controller guid", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			require.NoError(t, server.Close())
			require.NoError(t, client.Close())
		}()
		wg.Add(1)
		go serverAck(server)
		ok, err := VerifyCertificate(client, ctrlPrivateKey.PublicKey(), CtrlGUID)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("skip verify", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			require.NoError(t, server.Close())
			require.NoError(t, client.Close())
		}()
		wg.Add(1)
		go serverAck(server)
		ok, err := VerifyCertificate(client, nil, nil)
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("failed to receive certificate", func(t *testing.T) {
		server, client := net.Pipe()
		require.NoError(t, server.Close())
		defer func() {
			err := client.Close()
			require.NoError(t, err)
		}()
		ok, err := VerifyCertificate(client, nil, nil)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("different node guid", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			require.NoError(t, server.Close())
			require.NoError(t, client.Close())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := server.Write(bytes.Repeat([]byte{0}, CertificateSize))
			require.NoError(t, err)
		}()
		ok, err := VerifyCertificate(client, nil, nodeGUID)
		require.EqualError(t, err, "guid in certificate is different")
		require.False(t, ok)
	})

	t.Run("invalid certificate signature", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			require.NoError(t, server.Close())
			require.NoError(t, client.Close())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			cert := make([]byte, CertificateSize)
			copy(cert, nodeGUID)
			_, err := server.Write(cert)
			require.NoError(t, err)
		}()
		ok, err := VerifyCertificate(client, ctrlPrivateKey.PublicKey(), nodeGUID)
		require.EqualError(t, err, "invalid certificate signature")
		require.False(t, ok)
	})

	t.Run("failed to generate challenge", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			require.NoError(t, server.Close())
			require.NoError(t, client.Close())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := server.Write(certBytes)
			require.NoError(t, err)
		}()

		// patch
		patchFunc := func(_ interface{}, _ []byte) (int, error) {
			return 0, testsuite.ErrMonkey
		}
		pg := testsuite.PatchInstanceMethod(rand.Reader, "Read", patchFunc)
		defer pg.Unpatch()

		ok, err := VerifyCertificate(client, ctrlPrivateKey.PublicKey(), nodeGUID)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("failed to send challenge", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			require.NoError(t, client.Close())
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := server.Write(certBytes)
			require.NoError(t, err)
			err = server.Close()
			require.NoError(t, err)
		}()
		ok, err := VerifyCertificate(client, ctrlPrivateKey.PublicKey(), nodeGUID)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("failed to receive challenge signature", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			require.NoError(t, client.Close())
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
		ok, err := VerifyCertificate(client, ctrlPrivateKey.PublicKey(), nodeGUID)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("invalid challenge signature", func(t *testing.T) {
		server, client := net.Pipe()
		defer func() {
			require.NoError(t, client.Close())
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
		ok, err := VerifyCertificate(client, ctrlPrivateKey.PublicKey(), nodeGUID)
		require.EqualError(t, err, "invalid challenge signature")
		require.False(t, ok)
	})

	wg.Wait()
}
