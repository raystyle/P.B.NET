package certutil

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadCertificates(t *testing.T) {
	root, err := loadSystemCertWithName("ROOT")
	if err != nil {
		return
	}
	ca, err := loadSystemCertWithName("CA")
	if err != nil {
		return
	}
	certs := append(root, ca...)

	fmt.Println("number:", len(certs))

	for i := 0; i < len(certs); i++ {

		if i == 54 {
			_, err := x509.ParseCertificate(certs[i])
			if err != nil {
				fmt.Println(err)
			}

		}

		block := pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certs[i],
		}
		bs := pem.EncodeToMemory(&block)
		path := "e:/certs/" + strconv.Itoa(i) + ".pem"
		require.NoError(t, ioutil.WriteFile(path, bs, 644))
		_, err := x509.ParseCertificate(certs[i])
		if err == nil {
			// fmt.Println(cert.Issuer.CommonName)

		}

	}
}
