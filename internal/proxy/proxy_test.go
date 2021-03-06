package proxy

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/logger"
	"project/internal/patch/toml"
	"project/internal/proxy/http"
	"project/internal/proxy/socks"
	"project/internal/random"
	"project/internal/testsuite"
	"project/internal/testsuite/testcert"
	"project/internal/testsuite/testtls"
)

type groups map[string]*group

func (g groups) Clients() []*Client {
	clients := make([]*Client, len(g))
	for ri := 0; ri < 3+random.Int(10); ri++ {
		i := 0
		for _, group := range g {
			if i < 5 {
				clients[i] = group.client
			} else {
				break
			}
			i++
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
	err := g.server.Close()
	require.NoError(t, err)
}

func testGenerateProxyGroup(t *testing.T) groups {
	groups := make(map[string]*group)

	const (
		tag     = "test"
		network = "tcp"
	)

	// add socks5 server
	socks5Opts := &socks.Options{
		Username: "admin1",
		Password: "1234561",
	}
	socks5Server, err := socks.NewSocks5Server(tag, logger.Test, socks5Opts)
	require.NoError(t, err)
	go func() {
		err := socks5Server.ListenAndServe(network, "127.0.1.1:0")
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, socks5Server, 1)

	// add socks4a server
	socks4aOpts := &socks.Options{
		UserID: "admin2",
	}
	socks4aServer, err := socks.NewSocks4aServer(tag, logger.Test, socks4aOpts)
	require.NoError(t, err)
	go func() {
		err := socks4aServer.ListenAndServe(network, "127.0.1.2:0")
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, socks4aServer, 1)

	// add socks4 server
	socks4Opts := &socks.Options{
		UserID: "admin3",
	}
	socks4Server, err := socks.NewSocks4Server(tag, logger.Test, socks4Opts)
	require.NoError(t, err)
	go func() {
		err := socks4Server.ListenAndServe(network, "127.0.1.3:0")
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, socks4Server, 1)

	// add http proxy server
	httpOpts := &http.Options{
		Username: "admin4",
		Password: "1234564",
	}
	httpServer, err := http.NewHTTPServer(tag, logger.Test, httpOpts)
	require.NoError(t, err)
	go func() {
		err := httpServer.ListenAndServe(network, "127.0.1.4:0")
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, httpServer, 1)

	// add https proxy server
	certPool := testcert.CertPool(t)
	serverCfg, clientCfg := testtls.OptionPair(t, "127.0.1.5")
	httpsOpts := &http.Options{
		Username: "admin5",
		Password: "1234565",
	}
	httpsOpts.Server.TLSConfig = serverCfg
	httpsOpts.Server.TLSConfig.CertPool = certPool
	httpsOpts.Transport.TLSClientConfig.CertPool = certPool
	httpsServer, err := http.NewHTTPSServer(tag, logger.Test, httpsOpts)
	require.NoError(t, err)
	go func() {
		err := httpsServer.ListenAndServe(network, "127.0.1.5:0")
		require.NoError(t, err)
	}()
	testsuite.WaitProxyServerServe(t, httpsServer, 1)

	// add socks5 client
	address := socks5Server.Addresses()[0].String()
	socks5Client, err := socks.NewSocks5Client(network, address, socks5Opts)
	require.NoError(t, err)
	groups["socks5"] = &group{
		server: socks5Server,
		client: &Client{
			Tag:     "socks5-c",
			Mode:    ModeSocks5,
			Network: network,
			Address: address,
			client:  socks5Client,
		},
	}

	// add socks4a client
	address = socks4aServer.Addresses()[0].String()
	socks4aClient, err := socks.NewSocks4aClient(network, address, socks4aOpts)
	require.NoError(t, err)
	groups["socks4a"] = &group{
		server: socks4aServer,
		client: &Client{
			Tag:     "socks4a-c",
			Mode:    ModeSocks4a,
			Network: network,
			Address: address,
			client:  socks4aClient,
		},
	}

	// add socks4 client
	address = socks4Server.Addresses()[0].String()
	socks4Client, err := socks.NewSocks4Client(network, address, socks4Opts)
	require.NoError(t, err)
	groups["socks4"] = &group{
		server: socks4Server,
		client: &Client{
			Tag:     "socks4-c",
			Mode:    ModeSocks4,
			Network: network,
			Address: address,
			client:  socks4Client,
		},
	}

	// add http proxy client
	address = httpServer.Addresses()[0].String()
	httpClient, err := http.NewHTTPClient(network, address, httpOpts)
	require.NoError(t, err)
	groups["http"] = &group{
		server: httpServer,
		client: &Client{
			Tag:     "http-c",
			Mode:    ModeHTTP,
			Network: network,
			Address: address,
			client:  httpClient,
		},
	}

	// add https proxy client
	address = httpsServer.Addresses()[0].String()
	httpsOpts.TLSConfig = clientCfg
	httpsClient, err := http.NewHTTPSClient(network, address, httpsOpts)
	require.NoError(t, err)
	groups["https"] = &group{
		server: httpsServer,
		client: &Client{
			Tag:     "https-c",
			Mode:    ModeHTTPS,
			Network: network,
			Address: address,
			client:  httpsClient,
		},
	}
	return groups
}

func testGenerateBalanceInBalance(t *testing.T) (groups, *Balance) {
	groups := testGenerateProxyGroup(t)
	clients := make([]*Client, 3)

	b1, err := NewBalance("balance-1", groups.Clients()...)
	require.NoError(t, err)
	clients[0] = &Client{Tag: b1.tag, Mode: ModeBalance, client: b1}

	b2, err := NewBalance("balance-2", groups.Clients()...)
	require.NoError(t, err)
	clients[1] = &Client{Tag: b2.tag, Mode: ModeBalance, client: b2}

	b3, err := NewBalance("balance-3", groups.Clients()...)
	require.NoError(t, err)
	clients[2] = &Client{Tag: b3.tag, Mode: ModeBalance, client: b3}

	fb, err := NewBalance("final-balance", clients...)
	require.NoError(t, err)
	return groups, fb
}

func TestPrintClientsInfo(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	groups, fb := testGenerateBalanceInBalance(t)
	defer func() {
		err := groups.Close()
		require.NoError(t, err)
	}()
	fmt.Println(fb.Info())

	// create a chain
	c1 := groups.Clients()[0]
	c2 := &Client{Tag: fb.tag, Mode: ModeBalance, client: fb}
	c3 := groups.Clients()[1]
	chain, err := NewChain("chain-mix", c1, c2, c3)
	require.NoError(t, err)
	fmt.Println(chain.Info())

	// create a balance with chain
	cc := &Client{Tag: chain.tag, Mode: ModeChain, client: chain}
	balance, err := NewBalance("balance-mix", c1, cc, c3)
	require.NoError(t, err)
	fmt.Println(balance.Info())
}

func TestClientOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/client.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := Client{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	// check zero value
	testsuite.ContainZeroValue(t, opts)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "test", actual: opts.Tag},
		{expected: "socks5", actual: opts.Mode},
		{expected: "tcp", actual: opts.Network},
		{expected: "127.0.0.1:1080", actual: opts.Address},
		{expected: "username = \"admin\"", actual: opts.Options},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}

func TestServerOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/server.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := &Server{}
	err = toml.Unmarshal(data, opts)
	require.NoError(t, err)

	// check zero value
	testsuite.ContainZeroValue(t, opts)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "test", actual: opts.Tag},
		{expected: "socks5", actual: opts.Mode},
		{expected: "username = \"admin\"", actual: opts.Options},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
