package options

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_TLS_Default(t *testing.T) {
	config, err := new(TLS_Config).Apply()
	require.Nil(t, err, err)
	require.NotNil(t, config)
}
