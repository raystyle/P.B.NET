package options

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultTLS(t *testing.T) {
	config, err := new(TLSConfig).Apply()
	require.NoError(t, err)
	require.NotNil(t, config)
}
