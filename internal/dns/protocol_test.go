package dns

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDomainName(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		for _, domain := range []string{
			"test.com",
			"Test-sub.com",
			"test-sub2.com",
		} {
			require.True(t, IsDomainName(domain))
		}
	})

	t.Run("invalid", func(t *testing.T) {
		for _, domain := range []string{
			"",
			string([]byte{255, 254, 12, 35}),
			"test-",
			"Test.-",
			"test..",
			strings.Repeat("a", 64) + ".com",
		} {
			require.False(t, IsDomainName(domain))
		}
	})
}

func TestUnpackMessage(t *testing.T) {
	_, err := unpackMessage([]byte{1, 2, 3, 4})
	require.Error(t, err)
}
