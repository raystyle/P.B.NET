package proxy

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/proxy/http"
	"project/internal/proxy/socks"
	"project/internal/random"
	"project/internal/testsuite"
)

type groups map[string]*group

func (g groups) Clients() []*Client {
	clients := make([]*Client, len(g))
	for ri := 0; ri < 3+random.Int(10); ri++ {
		i := 0
		for _, group := range g {
			if i < 4 {
				clients[i] = group.client
			} else {
				break
			}
			i += 1
		}
	}
	return clients
}

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
			Tag:     "socks5-c",
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
			Tag:     "socks4a-c",
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
			Tag:     "http-c",
			Mode:    ModeHTTP,
			Network: "tcp",
			Address: address,
			client:  httpClient,
		},
	}

	// add https
	serverCfg, clientCfg := testsuite.TLSConfigOptionPair(t)
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
			Tag:     "https-c",
			Mode:    ModeHTTP,
			Network: "tcp",
			Address: address,
			client:  httpsClient,
		},
	}
	return groups
}

func TestOptions(t *testing.T) {
	// socks
	b, err := ioutil.ReadFile("testdata/socks.toml")
	require.NoError(t, err)
	socksOpts := socks.Options{}
	require.NoError(t, toml.Unmarshal(b, &socksOpts))
	// compare
	require.Equal(t, true, socksOpts.Socks4)
	require.Equal(t, "admin", socksOpts.Username)
	require.Equal(t, "123456", socksOpts.Password)
	require.Equal(t, time.Minute, socksOpts.Timeout)
	require.Equal(t, "test", socksOpts.UserID)
	require.Equal(t, true, socksOpts.DisableSocks4A)
	require.Equal(t, 100, socksOpts.MaxConns)

	// http
	b, err = ioutil.ReadFile("testdata/http.toml")
	require.NoError(t, err)
	httpOpts := http.Options{}
	require.NoError(t, toml.Unmarshal(b, &httpOpts))
	// compare
	require.Equal(t, true, httpOpts.HTTPS)
	require.Equal(t, "admin", httpOpts.Username)
	require.Equal(t, "123456", httpOpts.Password)
	require.Equal(t, time.Minute, httpOpts.Timeout)
	require.Equal(t, 100, httpOpts.MaxConns)
	require.Equal(t, "keep-alive", httpOpts.Header.Get("Connection"))
	require.Equal(t, 10*time.Second, httpOpts.Server.ReadTimeout)
	require.Equal(t, 2, httpOpts.Transport.MaxIdleConns)
}
