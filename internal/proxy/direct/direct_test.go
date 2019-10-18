package direct

import (
	"testing"
)

func TestDirect(t *testing.T) {
	d := Direct{}
	d.HTTP(nil)
	t.Log(d.Info())
	t.Log(d.Mode())
}
