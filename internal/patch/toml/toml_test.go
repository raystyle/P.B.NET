package toml

import (
	"fmt"
	"testing"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
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

	err = Unmarshal([]byte{0x00}, &test)
	require.Error(t, err)

	patchFunc := func(_ []byte) (*toml.Tree, error) {
		return nil, monkey.ErrMonkey
	}
	pg := monkey.Patch(toml.LoadBytes, patchFunc)
	defer pg.Unpatch()
	err = Unmarshal(data, &test)
	errStr := fmt.Sprintf("toml: %s in *toml.testStructRoot", monkey.ErrMonkey)
	require.EqualError(t, err, errStr)
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
	errStr := `toml: undecoded keys: ["Foo" "Leaf.Bar"] in *toml.testStructLeaf`
	require.EqualError(t, err, errStr)
}
