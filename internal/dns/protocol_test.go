package dns

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
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
	const (
		domain  = "test.com"
		queryId = 1234
	)

	t.Run("invalid message data", func(t *testing.T) {
		_, err := unpackMessage([]byte{1, 2, 3, 4}, domain, queryId)
		require.Error(t, err)
	})

	t.Run("not response", func(t *testing.T) {
		msg := packMessage(dnsmessage.TypeA, domain, queryId)

		_, err := unpackMessage(msg, domain, queryId)
		require.Error(t, err)
	})
}
