package dns

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDomainName(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		testdata := []string{
			"test.com",
			"Test-sub.com",
			"test-sub2.com",
		}
		for i := 0; i < len(testdata); i++ {
			require.True(t, IsDomainName(testdata[i]))
		}
	})

	t.Run("invalid", func(t *testing.T) {
		testdata := []string{
			"",
			string([]byte{255, 254, 12, 35}),
			"test-",
			"Test.-",
			"test..",
			strings.Repeat("a", 64) + ".com",
		}
		for i := 0; i < len(testdata); i++ {
			require.False(t, IsDomainName(testdata[i]))
		}
	})
}

func TestUnpackMessage(t *testing.T) {
	_, err := unpackMessage([]byte{1, 2, 3, 4})
	require.Error(t, err)
}
