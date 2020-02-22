package testcert

import (
	"testing"
)

func TestCertPool(t *testing.T) {
	pool := CertPool(t)
	t.Log(pool.GetPrivateClientPairs())
}
