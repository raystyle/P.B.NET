package test

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/proxy"
	"project/internal/testsuite"
	"project/tool/proxy/client"
	"project/tool/proxy/server"
)

func TestProxyClientWithBalanceAndChain(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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
				Mode:    proxy.ModeSocks5,
				Network: "tcp",
				Address: "localhost:0",
			},
		})
		go func(srv *server.Server) {
			err := srv.Main()
			require.NoError(t, err)
		}(proxyServers[i])
	}
	// wait proxy server serve
	time.Sleep(250 * time.Millisecond)
	defer func() {
		for i := 0; i < 9; i++ {
			require.NoError(t, proxyServers[i].Exit())
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
		cfg.Clients = append(cfg.Clients, &proxy.Client{
			Tag:     "socks5-0" + strconv.Itoa(i+1),
			Mode:    proxy.ModeSocks5,
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
		cfg.Clients = append(cfg.Clients, &proxy.Client{
			Tag:     "balance-0" + strconv.Itoa(i/3+1),
			Mode:    proxy.ModeBalance,
			Options: tags,
		})
	}

	// add final chain
	cfg.Clients = append(cfg.Clients, &proxy.Client{
		Tag:     "final-chain",
		Mode:    proxy.ModeChain,
		Options: `tags = ["balance-01","balance-02","balance-03"]`,
	})

	proxyClient := client.New("", &cfg)
	go func() {
		err := proxyClient.Main()
		require.NoError(t, err)
	}()
	// wait proxy server in client serve
	time.Sleep(250 * time.Millisecond)
	defer func() {
		require.NoError(t, proxyClient.Exit())
		testsuite.IsDestroyed(t, proxyClient)
	}()

	// make client
	u, err := url.Parse("socks5://" + proxyClient.Address())
	require.NoError(t, err)
	transport := http.Transport{Proxy: http.ProxyURL(u)}

	testsuite.ProxyServer(t, testsuite.NewNopCloser(), &transport)
}
