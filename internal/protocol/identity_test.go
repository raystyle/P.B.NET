package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRole_String(t *testing.T) {
	for _, testdata := range [...]*struct {
		expect string
		actual uint8
	}{
		{"controller", 0},
		{"node", 1},
		{"beacon", 2},
		{"invalid role: 3", 3},
	} {
		require.Equal(t, testdata.expect, Role(testdata.actual).String())
	}
	t.Log(Role(5).Error())
}

func TestRole_Bytes(t *testing.T) {
	for _, testdata := range [...]*struct {
		expect []byte
		actual uint8
	}{
		{[]byte{0}, 0},
		{[]byte{1}, 1},
		{[]byte{2}, 2},
		{[]byte{255}, 3},
	} {
		require.Equal(t, testdata.expect, Role(testdata.actual).Bytes())
	}
}
