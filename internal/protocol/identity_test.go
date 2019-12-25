package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRole_String(t *testing.T) {
	testdata := [...]*struct {
		expect string
		actual uint8
	}{
		{"controller", 0},
		{"node", 1},
		{"beacon", 2},
		{"invalid role: 3", 3},
	}
	for i := 0; i < len(testdata); i++ {
		require.Equal(t, testdata[i].expect, Role(testdata[i].actual).String())
	}
	t.Log(Role(5).Error())
}

func TestRole_Bytes(t *testing.T) {
	testdata := [...]*struct {
		expect []byte
		actual uint8
	}{
		{[]byte{0}, 0},
		{[]byte{1}, 1},
		{[]byte{2}, 2},
		{[]byte{255}, 3},
	}
	for i := 0; i < len(testdata); i++ {
		require.Equal(t, testdata[i].expect, Role(testdata[i].actual).Bytes())
	}
}

func TestRole_Byte(t *testing.T) {
	testdata := [...]*struct {
		expect byte
		actual uint8
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{255, 3},
	}
	for i := 0; i < len(testdata); i++ {
		require.Equal(t, testdata[i].expect, Role(testdata[i].actual).Byte())
	}
}
