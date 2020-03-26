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
	except := make([]byte, 0, 2*guid.Size+2*portSize)
	except = append(except, bytes.Repeat([]byte{1}, guid.Size)...)
	except = append(except, convert.Uint32ToBytes(3)...)
	except = append(except, bytes.Repeat([]byte{2}, guid.Size)...)
	except = append(except, convert.Uint32ToBytes(4)...)
	require.Equal(t, except, cid[:])
}
