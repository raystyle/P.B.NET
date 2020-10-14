package proxy

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/proxy"
	"project/internal/testsuite"
)

func TestBalanceAndChain(t *testing.T) {
	testsuite.InitHTTPServers(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	// generate 9 proxy servers
	proxyServers := make([]*Server, 9)
	var err error
	for i := 0; i < 9; i++ {
		options := fmt.Sprintf("username = \"admin\"\r\npassword = \"12345%d\"", i)
		proxyServers[i], err = NewServer(&ServerConfig{
			tag: strconv.Itoa(i + 1),
			Proxy: struct {
				Mode    string `toml:"mode"`
				Network string `toml:"network"`
				Address string `toml:"address"`
				Options string `toml:"options"`
			}{
				Mode:    proxy.ModeSocks5,
				Network: "tcp",
				Address: fmt.Sprintf("127.0.2.%d:0", i),
				Options: options,
			}})
		require.NoError(t, err)
		go func(server *Server) {
			err := server.Main()
			require.NoError(t, err)
		}(proxyServers[i])
	}
	// wait proxy servers serve
	time.Sleep(250 * time.Millisecond)

	// make proxy client
	config := ClientConfig{
		Server: struct {
			Mode    string `toml:"mode"`
			Network string `toml:"network"`
			Address string `toml:"address"`
			Options string `toml:"options"`
		}{
			Mode:    proxy.ModeSocks5,
			Network: "tcp",
			Address: "localhost:0",
		},
	}

	// add basic socks5 proxy clients
	for i := 0; i < 9; i++ {
		options := fmt.Sprintf("username = \"admin\"\r\npassword = \"12345%d\"", i)
		config.Clients = append(config.Clients, &proxy.Client{
			Tag:     "socks5-" + strconv.Itoa(i+1),
			Mode:    proxy.ModeSocks5,
			Network: "tcp",
			Address: proxyServers[i].testAddress(),
			Options: options,
		})
	}

	// add three balance
	for i := 0; i < 9; i = i + 3 {
		tag1 := "socks5-" + strconv.Itoa(i+1)
		tag2 := "socks5-" + strconv.Itoa(i+2)
		tag3 := "socks5-" + strconv.Itoa(i+3)
		tags := fmt.Sprintf(`tags = ["%s", "%s", "%s"]`, tag1, tag2, tag3)
		config.Clients = append(config.Clients, &proxy.Client{
			Tag:     "balance-" + strconv.Itoa(i/3+1),
			Mode:    proxy.ModeBalance,
			Options: tags,
		})
	}

	// add final chain
	config.Clients = append(config.Clients, &proxy.Client{
		Tag:     "final-chain",
		Mode:    proxy.ModeChain,
		Options: `tags = ["balance-1","balance-2","balance-3"]`,
	})

	proxyClient, err := NewClient(&config)
	require.NoError(t, err)
	go func() {
		err := proxyClient.Main()
		require.NoError(t, err)
	}()
	// wait proxy server in client serve
	time.Sleep(250 * time.Millisecond)

	// make client
	URL, err := url.Parse("socks5://" + proxyClient.testAddress())
	require.NoError(t, err)
	transport := &http.Transport{Proxy: http.ProxyURL(URL)}

	// test client
	httpClient := http.Client{Transport: transport}
	defer httpClient.CloseIdleConnections()
	resp, err := httpClient.Get("https://cloudflare-dns.com/")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_, err = io.Copy(ioutil.Discard, resp.Body)
	require.NoError(t, err)

	testsuite.ProxyServer(t, testsuite.NewNopCloser(), transport)

	// clean
	for i := 0; i < 9; i++ {
		err := proxyServers[i].Exit()
		require.NoError(t, err)
	}
	testsuite.IsDestroyed(t, &proxyServers)

	err = proxyClient.Exit()
	require.NoError(t, err)

	testsuite.IsDestroyed(t, proxyClient)
}
