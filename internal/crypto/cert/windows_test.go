// +build windows

package cert

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
	t.Run("UTF16PtrFromString", func(t *testing.T) {
		patch := func(_ string) (*uint16, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(syscall.UTF16PtrFromString, patch)
		defer pg.Unpatch()
		_, err := LoadSystemCertWithName("ROOT")
		monkey.IsMonkeyError(t, err)
	})

	t.Run("CertOpenSystemStore", func(t *testing.T) {
		patch := func(_ syscall.Handle, _ *uint16) (syscall.Handle, error) {
			return 0, monkey.Error
		}
		pg := monkey.Patch(syscall.CertOpenSystemStore, patch)
		defer pg.Unpatch()
		_, err := LoadSystemCertWithName("ROOT")
		monkey.IsMonkeyError(t, err)
	})

	t.Run("CertEnumCertificatesInStore error", func(t *testing.T) {
		patch := func(_ syscall.Handle, _ *syscall.CertContext) (*syscall.CertContext, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(syscall.CertEnumCertificatesInStore, patch)
		defer pg.Unpatch()
		_, err := LoadSystemCertWithName("ROOT")
		monkey.IsMonkeyError(t, err)
	})

	t.Run("CertEnumCertificatesInStore nil", func(t *testing.T) {
		patch := func(_ syscall.Handle, _ *syscall.CertContext) (*syscall.CertContext, error) {
			return nil, nil
		}
		pg := monkey.Patch(syscall.CertEnumCertificatesInStore, patch)
		defer pg.Unpatch()
		certs, err := LoadSystemCertWithName("ROOT")
		require.NoError(t, err)
		require.Len(t, certs, 0)
	})
}

func TestLoadSystemCert(t *testing.T) {
	patch := func(_ string) ([][]byte, error) {
		return nil, monkey.Error
	}
	pg := monkey.Patch(LoadSystemCertWithName, patch)
	defer pg.Unpatch()
	_, err := loadSystemCert()
	monkey.IsMonkeyError(t, err)
}

func TestSystemCertPool_Windows(t *testing.T) {
	// must set errSystemCert, other it will return a copy of cache
	func() {
		systemCertMu.Lock()
		defer systemCertMu.Unlock()
		errSystemCert = errors.New("temp")
	}()

	patch := func() ([]*x509.Certificate, error) {
		return nil, monkey.Error
	}
	pg := monkey.Patch(loadSystemCert, patch)
	defer pg.Unpatch()
	_, err := SystemCertPool()
	monkey.IsMonkeyError(t, err)
}
