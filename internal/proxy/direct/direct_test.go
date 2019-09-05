package direct

import (
	"testing"
)

func TestDirect(t *testing.T) {
	d := Direct{}
	d.HTTP(nil)
	d.Info()
}
