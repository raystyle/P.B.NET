package dns

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystemResolve(t *testing.T) {
	// ipv4
	ipList, err := systemResolve(domain, IPv4)
	require.NoError(t, err)
	t.Log("system resolve ipv4:", ipList)
	// ipv6
	ipList, err = systemResolve(domain, IPv6)
	require.NoError(t, err)
	t.Log("system resolve ipv6:", ipList)
	// invalid host
	ipList, err = systemResolve("asd.asd", IPv4)
	require.Error(t, err)
	require.Nil(t, ipList)
	// invalid type
	ipList, err = systemResolve(domain, "asd")
	require.Error(t, err)
	require.Nil(t, ipList)
}
