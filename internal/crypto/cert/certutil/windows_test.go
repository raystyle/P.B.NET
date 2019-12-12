// +build windows

package certutil

import (
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"
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
