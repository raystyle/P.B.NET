package monkey

import (
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsMonkeyError(t *testing.T) {
	pg := Patch(net.Dial, func(string, string) (net.Conn, error) {
		return nil, Error
	})
	defer pg.Unpatch()

	_, err := net.Dial("", "")
	IsMonkeyError(t, err)
}

func ExamplePatch() {
	patch := func(a ...interface{}) (n int, err error) {
		s := make([]interface{}, len(a))
		for i, v := range a {
			s[i] = strings.ReplaceAll(fmt.Sprint(v), "hell", "*bleep*")
		}
		return fmt.Fprintln(os.Stdout, s...)
	}
	pg := Patch(fmt.Println, patch)
	defer pg.Unpatch()

	fmt.Println("what the hell?")

	// output:
	// what the *bleep*?
}

// private structure in other package, and it appeared in a interface
type private struct {
	str string
}

func (p *private) Get(s string) string {
	return p.str + s + "foo"
}

func ExamplePatchInstanceMethod() {
	pri := &private{str: "pri"}
	patch := func(interface{}, string) string {
		return "monkey"
	}
	pg := PatchInstanceMethod(pri, "Get", patch)
	defer pg.Unpatch()

	fmt.Println(pri.Get("foo"))

	// output:
	// monkey
}

// copy from internal/testsuite/testsuite.go
func testDeferForPanic(t testing.TB) {
	r := recover()
	require.NotNil(t, r)
	t.Logf("\npanic in %s:\n%s\n", t.Name(), r)
}

func TestPatchInstanceMethodType(t *testing.T) {
	t.Run("unknown method", func(t *testing.T) {
		defer testDeferForPanic(t)

		pri := &private{str: "pri"}
		PatchInstanceMethod(pri, "foo", nil)
	})

	t.Run("invalid parameter", func(t *testing.T) {
		defer testDeferForPanic(t)

		pri := &private{str: "pri"}
		patch := func(interface{}, string, string) string {
			return "monkey"
		}
		PatchInstanceMethod(pri, "Get", patch)
	})
}
