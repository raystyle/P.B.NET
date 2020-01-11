package testsuite

import (
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestIsMonkeyError(t *testing.T) {
	pg := Patch(net.Dial, func(string, string) (net.Conn, error) {
		return nil, ErrMonkey
	})
	defer pg.Unpatch()

	_, err := net.Dial("", "")
	IsMonkeyError(t, err)
}

func ExamplePatch() {
	patchFunc := func(a ...interface{}) (n int, err error) {
		s := make([]interface{}, len(a))
		for i, v := range a {
			s[i] = strings.ReplaceAll(fmt.Sprint(v), "hell", "*bleep*")
		}
		return fmt.Fprintln(os.Stdout, s...)
	}
	pg := Patch(fmt.Println, patchFunc)
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
	typ := reflect.TypeOf(pri)
	patchFunc := func(interface{}, string) string {
		return "monkey"
	}
	pg := PatchInstanceMethod(typ, "Get", patchFunc)
	defer pg.Unpatch()

	fmt.Println(pri.Get("foo"))

	// output:
	// monkey
}

func TestPatchInstanceMethod(t *testing.T) {
	t.Run("unknown method", func(t *testing.T) {
		defer func() { recover() }()
		pri := &private{str: "pri"}
		typ := reflect.TypeOf(pri)
		PatchInstanceMethod(typ, "foo", nil)
	})

	t.Run("invalid parameter", func(t *testing.T) {
		defer func() { recover() }()
		pri := &private{str: "pri"}
		typ := reflect.TypeOf(pri)
		patchFunc := func(interface{}, string, string) string {
			return "monkey"
		}
		PatchInstanceMethod(typ, "Get", patchFunc)
	})
}
