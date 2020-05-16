package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSendReplyError(t *testing.T) {
	err := GetReplyError(nil)
	require.EqualError(t, err, "empty reply")

	err = GetReplyError(ReplyExpired)
	require.Equal(t, ErrReplyExpired, err)

	err = GetReplyError(ReplyHandled)
	require.Equal(t, ErrReplyHandled, err)

	err = GetReplyError([]byte("foo"))
	require.EqualError(t, err, "custom error: foo")
}
