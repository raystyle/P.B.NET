package dns

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDomainName(t *testing.T) {
	require.True(t, IsDomainName("asd.com"))
	require.True(t, IsDomainName("asd-asd.com"))
	require.True(t, IsDomainName("asd-asd6.com"))
	// invalid domain
	require.False(t, IsDomainName(""))
	require.False(t, IsDomainName(string([]byte{255, 254, 12, 35})))
	require.False(t, IsDomainName("asd-"))
	require.False(t, IsDomainName("asd.-"))
	require.False(t, IsDomainName("asd.."))
	require.False(t, IsDomainName(strings.Repeat("a", 64)+".com"))
}

func TestUnpackMessage(t *testing.T) {
	_, err := unpackMessage([]byte{1, 2, 3, 4})
	require.Error(t, err)
}
