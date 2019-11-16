package curve25519

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScalarBaseMult(t *testing.T) {
	x := make([]byte, 32)
	x[0] = 1

	for i := 0; i < 200; i++ {
		var err error
		x, err = ScalarBaseMult(x)
		if err != nil {
			t.Fatal(err)
		}
	}

	result := hex.EncodeToString(x)
	const expectedHex = "89161fde887b2b53de549af483940106ecc114d6982daa98256de23bdf77661a"
	require.Equal(t, expectedHex, result)
}

func TestKeyExchange(t *testing.T) {
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
	require.Error(t, err)
	require.Nil(t, cPub)
}
