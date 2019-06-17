package dnsclient

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/dns"
)

func Test_system_resolve(t *testing.T) {
	// ipv4
	ip_list, err := system_resolve(domain, dns.IPV4)
	require.Nil(t, err, err)
	t.Log("system resolve ipv4:", ip_list)
	// ipv6
	ip_list, err = system_resolve(domain, dns.IPV6)
	require.Nil(t, err, err)
	t.Log("system resolve ipv6:", ip_list)
	// invalid host
	_, err = system_resolve("asdasdas.asd", dns.IPV4)
	require.NotNil(t, err)
	// invalid type
	ip_list, err = system_resolve(domain, "asd")
	require.NotNil(t, err)
	require.Nil(t, ip_list)
}
