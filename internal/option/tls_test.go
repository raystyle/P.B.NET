package option

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/toml"
	"project/internal/testsuite/testcert"
)

func TestTLSDefault(t *testing.T) {
	t.Run("client side", func(t *testing.T) {
		tlsConfig := TLSConfig{}

		t.Run("without cert pool", func(t *testing.T) {
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Len(t, config.Certificates, 0)
			require.Len(t, config.RootCAs.Certs(), 0)
			require.Len(t, config.ClientCAs.Certs(), 0)
		})

		t.Run("with cert pool", func(t *testing.T) {
			tlsConfig.CertPool = testcert.CertPool(t)
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Len(t, config.Certificates, testcert.PublicClientCertNum)
			require.Len(t, config.RootCAs.Certs(), testcert.PublicRootCANum)
			require.Len(t, config.ClientCAs.Certs(), 0)
		})
	})

	t.Run("server side", func(t *testing.T) {
		tlsConfig := TLSConfig{ServerSide: true}

		t.Run("without cert pool", func(t *testing.T) {
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Len(t, config.Certificates, 0)
			require.Len(t, config.RootCAs.Certs(), 0)
			require.Len(t, config.ClientCAs.Certs(), 0)
		})

		t.Run("with cert pool", func(t *testing.T) {
			tlsConfig.CertPool = testcert.CertPool(t)
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Len(t, config.Certificates, 0)
			require.Len(t, config.RootCAs.Certs(), 0)
			require.Len(t, config.ClientCAs.Certs(), testcert.PublicClientCANum)
		})
	})

	t.Run("common", func(t *testing.T) {
		config, err := new(TLSConfig).Apply()
		require.NoError(t, err)

		require.Equal(t, tls.ClientAuthType(0), config.ClientAuth)
		require.Zero(t, config.ServerName)
		require.Nil(t, config.NextProtos)
		require.Equal(t, uint16(tls.VersionTLS12), config.MinVersion)
		require.Equal(t, uint16(0), config.MaxVersion)
		require.Nil(t, config.CipherSuites)
		require.Equal(t, false, config.InsecureSkipVerify)
	})
}

// copy from internal/testsuite/testsuite.go
func testCheckOptions(father string, v interface{}) string {
	ok, result := testCheckSpecialType(father, v)
	if ok {
		return result
	}
	typ := reflect.TypeOf(v)
	var value reflect.Value
	if typ.Kind() == reflect.Ptr {
		// check is nil point
		value = reflect.ValueOf(v)
		typ = value.Type()
		if value.IsNil() {
			return father + typ.Name() + " is nil point"
		}
		value = value.Elem()
		typ = value.Type()
	} else {
		value = reflect.ValueOf(v)
	}
	for i := 0; i < value.NumField(); i++ {
		fieldType := typ.Field(i)
		fieldValue := value.Field(i)
		// skip unexported field
		if fieldType.PkgPath != "" && !fieldType.Anonymous {
			continue
		}
		// skip filed with check tag
		if fieldType.Tag.Get("check") == "-" {
			continue
		}
		switch fieldType.Type.Kind() {
		case reflect.Struct, reflect.Ptr, reflect.Interface:
			var f string
			if father == "" {
				f = typ.Name() + "." + fieldType.Name
			} else {
				f = father + "." + fieldType.Name
			}
			str := testCheckOptions(f, fieldValue.Interface())
			if str != "" {
				return str
			}
		case reflect.Chan, reflect.Func, reflect.Complex64,
			reflect.Complex128, reflect.UnsafePointer:
			continue
		default:
			if !fieldValue.IsZero() {
				continue
			}
			const format = "%s.%s is zero value"
			if father == "" {
				return fmt.Sprintf(format, typ.Name(), fieldType.Name)
			}
			return fmt.Sprintf(format, father, fieldType.Name)
		}
	}
	return ""
}

func testCheckSpecialType(father string, v interface{}) (bool, string) {
	var typ string
	switch val := v.(type) {
	case *time.Time:
		if val != nil && !val.IsZero() {
			return true, ""
		}
		typ = "time.Time"
	case time.Time:
		if !val.IsZero() {
			return true, ""
		}
		typ = "time.Time"
	default:
		return false, ""
	}
	if father == "" {
		return true, typ + " is zero value"
	}
	return true, father + " is zero value"
}

// the number of the certificate in testdata/tls.toml
const (
	testRootCANum      = 3
	testClientCANum    = 2
	testCertificateNum = 1
)

func TestTLSUnmarshal(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/tls.toml")
	require.NoError(t, err)

	tlsConfig := TLSConfig{}
	err = toml.Unmarshal(data, &tlsConfig)
	require.NoError(t, err)

	// check zero value
	str := testCheckOptions("", tlsConfig)
	require.True(t, str == "", str)

	t.Run("client side", func(t *testing.T) {
		t.Run("without cert pool", func(t *testing.T) {
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Len(t, config.Certificates, testCertificateNum)
			require.Len(t, config.RootCAs.Certs(), testRootCANum)
			require.Len(t, config.ClientCAs.Certs(), 0)
		})

		t.Run("with cert pool", func(t *testing.T) {
			tlsConfig.CertPool = testcert.CertPool(t)
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			clientCertNum := testCertificateNum + testcert.PrivateClientCertNum
			require.Len(t, config.Certificates, clientCertNum)
			rootCANum := testRootCANum + testcert.PrivateRootCANum
			require.Len(t, config.RootCAs.Certs(), rootCANum)
			require.Len(t, config.ClientCAs.Certs(), 0)
		})
	})

	t.Run("server side", func(t *testing.T) {
		tlsConfig.CertPool = nil
		tlsConfig.ServerSide = true

		t.Run("without cert pool", func(t *testing.T) {
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Len(t, config.Certificates, testCertificateNum)
			require.Len(t, config.RootCAs.Certs(), 0)
			require.Len(t, config.ClientCAs.Certs(), testClientCANum)
		})

		t.Run("with cert pool", func(t *testing.T) {
			tlsConfig.CertPool = testcert.CertPool(t)
			config, err := tlsConfig.Apply()
			require.NoError(t, err)

			require.Len(t, config.Certificates, testCertificateNum)
			require.Len(t, config.RootCAs.Certs(), 0)
			clientCANum := testClientCANum + testcert.PrivateClientCANum
			require.Len(t, config.ClientCAs.Certs(), clientCANum)
		})
	})

	t.Run("common", func(t *testing.T) {
		tlsConfig.CertPool = nil
		config, err := tlsConfig.Apply()
		require.NoError(t, err)

		testdata := [...]*struct {
			expected interface{}
			actual   interface{}
		}{
			{expected: tls.ClientAuthType(4), actual: config.ClientAuth},
			{expected: "test.com", actual: config.ServerName},
			{expected: []string{"h2", "h2c"}, actual: config.NextProtos},
			{expected: uint16(tls.VersionTLS10), actual: config.MinVersion},
			{expected: uint16(tls.VersionTLS11), actual: config.MaxVersion},
			{expected: []uint16{tls.TLS_RSA_WITH_AES_128_GCM_SHA256}, actual: config.CipherSuites},
			{expected: false, actual: config.InsecureSkipVerify},
		}
		for _, td := range testdata {
			require.Equal(t, td.expected, td.actual)
		}
	})
}

func TestTLSConfig_Apply(t *testing.T) {
	config := TLSConfig{}

	t.Run("invalid certificates", func(t *testing.T) {
		config.Certificates = append(config.Certificates, X509KeyPair{
			Cert: "foo data",
			Key:  "foo data",
		})
		_, err := config.Apply()
		require.Error(t, err)
	})

	t.Run("invalid Root CAs", func(t *testing.T) {
		config.Certificates = nil
		config.RootCAs = []string{"foo data"}
		_, err := config.Apply()
		require.Error(t, err)
	})

	t.Run("invalid Client CAs", func(t *testing.T) {
		config.ServerSide = true
		config.ClientCAs = []string{"foo data"}
		_, err := config.Apply()
		require.Error(t, err)
	})
}
