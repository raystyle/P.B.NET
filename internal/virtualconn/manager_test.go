package virtualconn

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/convert"
	"project/internal/guid"
)

func TestNewConnID(t *testing.T) {
	local := guid.GUID{}
	copy(local[:], bytes.Repeat([]byte{1}, guid.Size))
	remote := guid.GUID{}
	copy(remote[:], bytes.Repeat([]byte{2}, guid.Size))

	cid := NewConnID(&local, 3, &remote, 4)

	expected := make([]byte, 0, 2*guid.Size+2*portSize)
	expected = append(expected, bytes.Repeat([]byte{1}, guid.Size)...)
	expected = append(expected, convert.BEUint32ToBytes(3)...)
	expected = append(expected, bytes.Repeat([]byte{2}, guid.Size)...)
	expected = append(expected, convert.BEUint32ToBytes(4)...)

	require.Equal(t, expected, cid[:])
}
