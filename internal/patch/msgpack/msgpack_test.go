package msgpack

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"
)

type testStructRoot struct {
	Foo   int
	Leaf  *TestStructLeaf
	Slice []*TestStructLeaf
}

type TestStructLeaf struct {
	Bar int
}

func TestMsgpack(t *testing.T) {
	a := testStructRoot{
		Foo: 1,
	}
	b, err := msgpack.Marshal(&a)
	require.NoError(t, err)

	bb := new(TestStructLeaf)
	decoder := msgpack.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()

	err = decoder.Decode(bb)
	require.NoError(t, err)
}
