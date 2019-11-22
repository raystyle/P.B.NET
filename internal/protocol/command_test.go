package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSendReplyError(t *testing.T) {
	require.EqualError(t, GetReplyError(nil), "empty reply")
	require.Equal(t, ErrReplyExpired, GetReplyError(ReplyExpired))
	require.Equal(t, ErrReplyHandled, GetReplyError(ReplyHandled))
	require.EqualError(t, GetReplyError([]byte("foo")), "custom error: foo")
}
