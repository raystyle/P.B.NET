package msgpack

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testStructRoot struct {
	Foo   int
	Leaf  *testStructLeaf
	Slice []*testStructLeaf
}

type testStructLeaf struct {
	Bar int
}

func TestMsgpack(t *testing.T) {
	a := &testStructRoot{
		Foo: 1,
	}
	a.Leaf = new(testStructLeaf)
	a.Leaf.Bar = 2
	data, err := Marshal(a)
	require.NoError(t, err)

	b := new(testStructRoot)
	err = Unmarshal(data, b)
	require.NoError(t, err)
	require.Equal(t, a, b)

	_, err = Marshal(func() {})
	require.Error(t, err)
}

func TestMsgpackWithUnknownField(t *testing.T) {
	a := testStructRoot{
		Foo: 1,
	}
	a.Leaf = new(testStructLeaf)
	a.Leaf.Bar = 2
	data, err := Marshal(&a)
	require.NoError(t, err)

	b := new(testStructLeaf)
	err = Unmarshal(data, b)
	errStr := "msgpack: unknown field \"Foo\" in *msgpack.testStructLeaf"
	require.EqualError(t, err, errStr)
}
