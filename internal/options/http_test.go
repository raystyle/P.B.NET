package options

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_HTTP_Default(t *testing.T) {
	// request
	request, err := new(HTTP_Request).Apply()
	require.Nil(t, err, err)
	require.NotNil(t, request)
	// transport
	transport, err := new(HTTP_Transport).Apply()
	require.Nil(t, err, err)
	require.NotNil(t, transport)
	// server
	server, err := new(HTTP_Server).Apply()
	require.Nil(t, err, err)
	require.NotNil(t, server)
}
