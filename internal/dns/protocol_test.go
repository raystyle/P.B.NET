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
		queryID = 0x1234
	)

	t.Run("invalid message data", func(t *testing.T) {
		_, err := unpackMessage([]byte{1, 2, 3, 4}, domain, queryID)
		require.Error(t, err)
	})

	t.Run("not response", func(t *testing.T) {
		msg := packMessage(dnsmessage.TypeA, domain, queryID)

		_, err := unpackMessage(msg, domain, queryID)
		require.EqualError(t, err, "dns message is not a response")
	})

	t.Run("different query id", func(t *testing.T) {
		msg := dnsmessage.Message{}
		msg.Response = true
		data, err := msg.Pack()
		require.NoError(t, err)

		_, err = unpackMessage(data, domain, queryID)
		errStr := `query id "0x0000" in dns message is different with original "0x1234"`
		require.EqualError(t, err, errStr)
	})

	t.Run("unexpected question", func(t *testing.T) {
		msg := dnsmessage.Message{}
		msg.Response = true
		msg.ID = queryID
		data, err := msg.Pack()
		require.NoError(t, err)

		_, err = unpackMessage(data, domain, queryID)
		require.EqualError(t, err, "dns message with unexpected question")
	})

	t.Run("different domain name", func(t *testing.T) {
		msg := dnsmessage.Message{}
		msg.Response = true
		msg.ID = queryID
		name, err := dnsmessage.NewName("123.")
		require.NoError(t, err)
		msg.Questions = append(msg.Questions, dnsmessage.Question{Name: name})
		data, err := msg.Pack()
		require.NoError(t, err)

		_, err = unpackMessage(data, domain, queryID)
		errStr := `domain name "123" in dns message is different with original "test.com"`
		require.EqualError(t, err, errStr)
	})
}
