package curve25519

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

const expectedHex = "89161fde887b2b53de549af483940106ecc114d6982daa98256de23bdf77661a"

func TestScalarBaseMult(t *testing.T) {
	var (
		out []byte
		err error
	)
	in := make([]byte, 32)
	in[0] = 1
	for i := 0; i < 200; i++ {
		out, err = ScalarBaseMult(in)
		require.NoError(t, err)
		in, out = out, in
	}
	result := hex.EncodeToString(in)
	require.Equal(t, expectedHex, result)
	// key exchange
	cPri := make([]byte, 32)
	cPri[0] = 199
	cPub, err := ScalarBaseMult(cPri)
	require.NoError(t, err)
	sPri := make([]byte, 32)
	sPri[0] = 2
	sPub, err := ScalarBaseMult(sPri)
	require.NoError(t, err)
	cKey, err := ScalarMult(cPri, sPub)
	require.NoError(t, err)
	sKey, err := ScalarMult(sPri, cPub)
	require.NoError(t, err)
	require.Equal(t, cKey, sKey)
	t.Log(cKey)
	// invalid in size
	cPub, err = ScalarBaseMult(nil)
	require.NoError(t, err)
	require.Nil(t, cPub)
}
