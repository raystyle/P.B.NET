package curve25519

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

const expectedHex = "89161fde887b2b53de549af483940106ecc114d6982daa98256de23bdf77661a"

func Test_Scalar_Base_Mult(t *testing.T) {
	var (
		out []byte
		err error
	)
	in := make([]byte, 32)
	in[0] = 1
	for i := 0; i < 200; i++ {
		out, err = Scalar_Base_Mult(in)
		require.Nil(t, err, err)
		in, out = out, in
	}
	result := hex.EncodeToString(in)
	require.Equal(t, expectedHex, result)
}
