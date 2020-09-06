// +build windows

package privilege

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnableDebugPrivilege(t *testing.T) {
	err := EnableDebug()
	require.NoError(t, err)
}
