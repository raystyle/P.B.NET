package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetVersion(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		major, minor, err := GetVersion()
		require.NoError(t, err)

		fmt.Println("major:", major, "minor", minor)
	})
}
