package direct

import (
	"testing"
)

func Test_Direct(t *testing.T) {
	d := Direct{}
	d.HTTP(nil)
	d.Info()
}
