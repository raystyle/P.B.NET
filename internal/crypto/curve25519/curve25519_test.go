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
	// key exchange
	c_pri := make([]byte, 32)
	c_pri[0] = 199
	c_pub, err := Scalar_Base_Mult(c_pri)
	require.Nil(t, err, err)
	s_pri := make([]byte, 32)
	s_pri[0] = 2
	s_pub, err := Scalar_Base_Mult(s_pri)
	require.Nil(t, err, err)
	key_c, err := Scalar_Mult(c_pri, s_pub)
	require.Nil(t, err, err)
	key_s, err := Scalar_Mult(s_pri, c_pub)
	require.Nil(t, err, err)
	require.Equal(t, key_c, key_s)
	t.Log(key_c)
	// invalid in size
	c_pub, err = Scalar_Base_Mult(nil)
	require.NotNil(t, err)
	require.Nil(t, c_pub)
}
