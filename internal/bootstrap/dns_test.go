package bootstrap

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/xnet"
)

const test_domain string = "localhost"

func Test_DNS(t *testing.T) {
	DNS := New_DNS(nil)
	DNS.Domain = test_domain
	DNS.L_Mode = xnet.TLS
	DNS.L_Network = "tcp"
	DNS.L_Port = "443"
	_, _ = DNS.Generate(nil)
	b, err := DNS.Marshal()
	require.Nil(t, err, err)
	DNS = New_DNS(new(mock_resolver))
	err = DNS.Unmarshal(b)
	require.Nil(t, err, err)
	nodes, err := DNS.Resolve()
	require.Nil(t, err, err)
	spew.Dump(nodes)
}
