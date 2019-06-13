package bootstrap

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"

	"project/internal/netx"
)

const test_domain string = "www.baidu.com"

func Test_DNS(t *testing.T) {
	DNS := New_DNS(nil)
	DNS.Domain = test_domain
	DNS.L_Mode = netx.TLS
	DNS.L_Network = "tcp"
	DNS.L_Port = "443"
	_, _ = DNS.Generate(nil)
	b, err := DNS.Marshal()
	require.Nil(t, err, err)
	DNS = New_DNS(&mock_resolver{})
	err = DNS.Unmarshal(b)
	require.Nil(t, err, err)
	nodes, err := DNS.Resolve()
	require.Nil(t, err, err)
	spew.Dump(nodes)
}
