// +build windows

package shell

import (
	"context"
	"fmt"
	"testing"

	"github.com/axgle/mahonia"
	"github.com/stretchr/testify/require"
)

func TestShell(t *testing.T) {
	output, err := Shell(context.Background(), "whoami")
	require.NoError(t, err)

	decoder := mahonia.NewDecoder("GBK")

	fmt.Println(decoder.ConvertString(string(output)))

	output, err = Shell(context.Background(), "ddd")
	fmt.Println(decoder.ConvertString(string(output)), err)
}
