package json

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

func TestMarshal(t *testing.T) {
	a := &testStructRoot{
		Foo: 1,
	}
	a.Leaf = new(testStructLeaf)
	a.Leaf.Bar = 2
	data, err := Marshal(a)
	require.NoError(t, err)
	t.Log(string(data))

	_, err = Marshal(func() {})
	require.Error(t, err)
}

func TestUnmarshal(t *testing.T) {
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

	// invalid data
	err = Unmarshal(nil, b)
	require.Error(t, err)
}

func TestUnmarshalWithUnknownField(t *testing.T) {
	a := testStructRoot{
		Foo: 1,
	}
	a.Leaf = new(testStructLeaf)
	a.Leaf.Bar = 2
	data, err := Marshal(&a)
	require.NoError(t, err)

	b := new(testStructLeaf)
	err = Unmarshal(data, b)
	errStr := `json: unknown field "Foo" in *json.testStructLeaf`
	require.EqualError(t, err, errStr)
}
