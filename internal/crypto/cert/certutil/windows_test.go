// +build windows

package certutil

import (
	"crypto/x509"
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
)

func TestLoadSystemCertWithName(t *testing.T) {
	root, err := LoadSystemCertWithName("ROOT")
	require.NoError(t, err)
	ca, err := LoadSystemCertWithName("CA")
	require.NoError(t, err)

	certs := append(root, ca...)
	t.Log("raw number:", len(certs))

	count := 0
	for i := 0; i < len(certs); i++ {
		cert, err := x509.ParseCertificate(certs[i])
		if err != nil {
			t.Log(err)
			continue
		}
		count++

		// print CA info
		const format = "V%d %s\n"
		switch {
		case cert.Subject.CommonName != "":
			t.Logf(format, cert.Version, cert.Subject.CommonName)
		case len(cert.Subject.Organization) != 0:
			t.Logf(format, cert.Version, cert.Subject.Organization[0])
		default:
			t.Logf(format, cert.Version, cert.Subject)
		}
	}
	t.Log("actual number:", count)
}

func TestLoadSystemCertWithNameFailed(t *testing.T) {
	t.Run("syscall.UTF16PtrFromString", func(t *testing.T) {
		patchFunc := func(_ string) (*uint16, error) {
			return nil, monkey.ErrMonkey
		}
		pg := monkey.Patch(syscall.UTF16PtrFromString, patchFunc)
		defer pg.Unpatch()
		_, err := LoadSystemCertWithName("ROOT")
		monkey.IsMonkeyError(t, err)
	})

	t.Run("syscall.CertOpenSystemStore", func(t *testing.T) {
		patchFunc := func(_ syscall.Handle, _ *uint16) (syscall.Handle, error) {
			return 0, monkey.ErrMonkey
		}
		pg := monkey.Patch(syscall.CertOpenSystemStore, patchFunc)
		defer pg.Unpatch()
		_, err := LoadSystemCertWithName("ROOT")
		monkey.IsMonkeyError(t, err)
	})

	t.Run("syscall.CertEnumCertificatesInStore error", func(t *testing.T) {
		patchFunc := func(_ syscall.Handle, _ *syscall.CertContext) (*syscall.CertContext, error) {
			return nil, monkey.ErrMonkey
		}
		pg := monkey.Patch(syscall.CertEnumCertificatesInStore, patchFunc)
		defer pg.Unpatch()
		_, err := LoadSystemCertWithName("ROOT")
		monkey.IsMonkeyError(t, err)
	})

	t.Run("syscall.CertEnumCertificatesInStore nil", func(t *testing.T) {
		patchFunc := func(_ syscall.Handle, _ *syscall.CertContext) (*syscall.CertContext, error) {
			return nil, nil
		}
		pg := monkey.Patch(syscall.CertEnumCertificatesInStore, patchFunc)
		defer pg.Unpatch()
		certs, err := LoadSystemCertWithName("ROOT")
		require.NoError(t, err)
		require.Equal(t, 0, len(certs))
	})
}

func TestLoadSystemCert(t *testing.T) {
	patchFunc := func(_ string) ([][]byte, error) {
		return nil, monkey.ErrMonkey
	}
	pg := monkey.Patch(LoadSystemCertWithName, patchFunc)
	defer pg.Unpatch()
	_, err := loadSystemCert()
	monkey.IsMonkeyError(t, err)
}

func TestSystemCertPool_Windows(t *testing.T) {
	// must set errSystemCert, other it will return a copy of cache
	func() {
		systemCertMutex.Lock()
		defer systemCertMutex.Unlock()
		errSystemCert = errors.New("temp")
	}()

	patchFunc := func() ([]*x509.Certificate, error) {
		return nil, monkey.ErrMonkey
	}
	pg := monkey.Patch(loadSystemCert, patchFunc)
	defer pg.Unpatch()
	_, err := SystemCertPool()
	monkey.IsMonkeyError(t, err)
}
