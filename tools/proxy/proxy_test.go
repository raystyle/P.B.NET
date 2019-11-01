package test

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"project/internal/proxy"
	"project/internal/testsuite"
	"project/tools/proxy/client"
	"project/tools/proxy/server"
)

func TestProxyClientWithBalanceAndChain(t *testing.T) {
	proxyServers := make([]*server.Server, 9)
	// generate 9 proxy server
	for i := 0; i < 9; i++ {
		proxyServers[i] = server.New(&server.Configs{
			Tag: strconv.Itoa(i + 1),
			Proxy: struct {
				Mode    string `toml:"mode"`
				Network string `toml:"network"`
				Address string `toml:"address"`
				Options string `toml:"options"`
			}{
				Mode:    proxy.ModeSocks,
				Network: "tcp",
				Address: "localhost:0",
			},
		})
		require.NoError(t, proxyServers[i].Start())
	}
	defer func() {
		for i := 0; i < 9; i++ {
			require.NoError(t, proxyServers[i].Stop())
		}
		testsuite.IsDestroyed(t, &proxyServers)
	}()

	// make proxy client
	cfg := client.Configs{
		Listener: struct {
			Network  string `toml:"network"`
			Address  string `toml:"address"`
			Username string `toml:"username"`
			Password string `toml:"password"`
			MaxConns int    `toml:"max_conns"`
		}{
			Network: "tcp",
			Address: "localhost:0",
		},
	}

	// add basic socks5 proxy clients
	for i := 0; i < 9; i++ {
		cfg.Clients = append(cfg.Clients, &struct {
			Tag     string `toml:"tag"`
			Mode    string `toml:"mode"`
			Network string `toml:"network"`
			Address string `toml:"address"`
			Options string `toml:"options"`
		}{
			Tag:     "socks5-0" + strconv.Itoa(i+1),
			Mode:    proxy.ModeSocks,
			Network: "tcp",
			Address: proxyServers[i].Address(),
		})
	}

	// add three balance
	for i := 0; i < 9; i = i + 3 {
		tag1 := "socks5-0" + strconv.Itoa(i+1)
		tag2 := "socks5-0" + strconv.Itoa(i+2)
		tag3 := "socks5-0" + strconv.Itoa(i+3)
		tags := fmt.Sprintf(`tags = ["%s","%s","%s"]`, tag1, tag2, tag3)
		cfg.Clients = append(cfg.Clients, &struct {
			Tag     string `toml:"tag"`
			Mode    string `toml:"mode"`
			Network string `toml:"network"`
			Address string `toml:"address"`
			Options string `toml:"options"`
		}{
			Tag:     "balance-0" + strconv.Itoa(i/3+1),
			Mode:    proxy.ModeBalance,
			Options: tags,
		})
	}

	// add final chain
	cfg.Clients = append(cfg.Clients, &struct {
		Tag     string `toml:"tag"`
		Mode    string `toml:"mode"`
		Network string `toml:"network"`
		Address string `toml:"address"`
		Options string `toml:"options"`
	}{
		Tag:     "final-chain",
		Mode:    proxy.ModeChain,
		Options: `tags = ["balance-01","balance-02","balance-03"]`,
	})

	proxyClient := client.New("", &cfg)
	require.NoError(t, proxyClient.Start())

	defer func() {
		require.NoError(t, proxyClient.Stop())
		testsuite.IsDestroyed(t, proxyClient)
	}()

	// make client
	u, err := url.Parse("socks5://" + proxyClient.Address())
	require.NoError(t, err)
	transport := &http.Transport{Proxy: http.ProxyURL(u)}
	hc := http.Client{Transport: transport}
	defer hc.CloseIdleConnections()

	testsuite.ProxyServer(t, testsuite.NopCloser(), &hc)
}
