package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/proxy/http"
	"project/internal/proxy/socks"
	"project/internal/testutil"
)

type groups map[string]*group

func (g groups) Close() error {
	for _, group := range g {
		err := group.server.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

type group struct {
	server server
	client *Client
}

func (g *group) Close(t *testing.T) {
	require.NoError(t, g.server.Close())
}

func testGenerateProxyGroup(t *testing.T) groups {
	groups := make(map[string]*group)

	// add socks5
	socksOpts := &socks.Options{
		Username: "admin",
		Password: "123456",
	}
	socks5Server, err := socks.NewServer("test", logger.Test, socksOpts)
	require.NoError(t, err)
	require.NoError(t, socks5Server.ListenAndServe("tcp", "localhost:0"))
	address := socks5Server.Address()
	socks5Client, err := socks.NewClient("tcp", address, socksOpts)
	require.NoError(t, err)
	groups["socks5"] = &group{
		server: socks5Server,
		client: &Client{
			Mode:    ModeSocks,
			Network: "tcp",
			Address: address,
			client:  socks5Client,
		},
	}

	// add socks4a
	socksOpts = &socks.Options{
		Socks4: true,
		UserID: "admin",
	}
	socks4aServer, err := socks.NewServer("test", logger.Test, socksOpts)
	require.NoError(t, err)
	require.NoError(t, socks4aServer.ListenAndServe("tcp", "localhost:0"))
	address = socks4aServer.Address()
	socks4aClient, err := socks.NewClient("tcp", address, socksOpts)
	require.NoError(t, err)
	groups["socks4a"] = &group{
		server: socks4aServer,
		client: &Client{
			Mode:    ModeSocks,
			Network: "tcp",
			Address: address,
			client:  socks4aClient,
		},
	}

	// add http
	httpOpts := &http.Options{
		Username: "admin",
		Password: "123456",
	}
	httpServer, err := http.NewServer("test", logger.Test, httpOpts)
	require.NoError(t, err)
	require.NoError(t, httpServer.ListenAndServe("tcp", "localhost:0"))
	address = httpServer.Address()
	httpClient, err := http.NewClient("tcp", address, httpOpts)
	require.NoError(t, err)
	groups["http"] = &group{
		server: httpServer,
		client: &Client{
			Mode:    ModeHTTP,
			Network: "tcp",
			Address: address,
			client:  httpClient,
		},
	}

	// add https
	serverCfg, clientCfg := testutil.TLSConfigOptionPair(t)
	httpOpts = &http.Options{
		HTTPS:    true,
		Username: "admin",
		Password: "123456",
	}
	httpOpts.Server.TLSConfig = *serverCfg
	httpsServer, err := http.NewServer("test", logger.Test, httpOpts)
	require.NoError(t, err)
	require.NoError(t, httpsServer.ListenAndServe("tcp", "localhost:0"))
	address = httpsServer.Address()
	httpOpts.TLSConfig = *clientCfg
	httpsClient, err := http.NewClient("tcp", address, httpOpts)
	require.NoError(t, err)
	groups["https"] = &group{
		server: httpsServer,
		client: &Client{
			Mode:    ModeHTTP,
			Network: "tcp",
			Address: address,
			client:  httpsClient,
		},
	}
	return groups
}
