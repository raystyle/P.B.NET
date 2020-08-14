// +build windows

package netstat

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
)

func TestRefresher(t *testing.T) {
	ref, err := newRefresher()
	require.NoError(t, err)

	conns, err := ref.Refresh()
	require.NoError(t, err)

	for i := 0; i < len(conns); i++ {
		spew.Dump(conns[i])

	}

}
