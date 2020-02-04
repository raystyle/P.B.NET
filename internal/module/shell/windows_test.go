package shell

import (
	"fmt"
	"testing"

	"github.com/axgle/mahonia"
)

func TestShell(t *testing.T) {
	output, err := Shell("whoami")
	fmt.Println("1", err)

	str := mahonia.NewDecoder("GBK").ConvertString(string(output))
	fmt.Println("2", str)
}
