package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Load(t *testing.T) {
	// load

	// invalid mode
	_, err := Load_Client(15, nil)
	require.Equal(t, err, ERR_UNKNOWN_MODE)
	// invalid socks5 config
	_, err = Load_Client(SOCKS5, nil)
	require.Equal(t, err, ERR_INVALID_SOCKS5_CONFIG)
	// invalid http proxy config
	_, err = Load_Client(HTTP, nil)
	require.Equal(t, err, ERR_INVALID_HTTP_PROXY_CONFIG)
}
