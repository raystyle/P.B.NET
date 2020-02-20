package toml

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"
)

type testStructRoot struct {
	Foo  int
	Leaf *TestStructLeaf
	Asd  []*TestStructLeaf
}

type TestStructLeaf struct {
	Bar int
}

func TestUnmarshal(t *testing.T) {
	test := testStructRoot{}
	data := []byte(`
      Foo = 1
      #[Leaf]
      #  Bar = 2
`)
	err := Unmarshal(data, &test)
	require.NoError(t, err)

	require.Equal(t, 1, test.Foo)
	require.Equal(t, 2, test.Leaf.Bar)

}

func TestMsgpack(t *testing.T) {
	a := testStructRoot{
		Foo:  1,
		Leaf: nil,
		Asd:  nil,
	}
	b, err := msgpack.Marshal(&a)
	require.NoError(t, err)

	bb := new(TestStructLeaf)
	decoder := msgpack.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()

	err = decoder.Decode(bb)
	require.NoError(t, err)
}
