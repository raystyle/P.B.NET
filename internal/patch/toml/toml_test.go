package toml

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testStructRoot struct {
	Foo  int
	Leaf *TestStructLeaf
	Asd  []*TestStructLeaf
}

type TestStructLeaf struct {
	Bar int
}

func TestMarshal(t *testing.T) {
	test := testStructRoot{}
	test.Foo = 1
	b, err := Marshal(test)
	require.NoError(t, err)
	t.Logf("\n%s", b)
}

func TestUnmarshal(t *testing.T) {
	test := testStructRoot{}
	data := []byte(`
      Foo = 1
      [Leaf]
        Bar = 2
`)
	err := Unmarshal(data, &test)
	require.NoError(t, err)

	require.Equal(t, 1, test.Foo)
	require.Equal(t, 2, test.Leaf.Bar)
}
