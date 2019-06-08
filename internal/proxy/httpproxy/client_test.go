package httpproxy

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Client(t *testing.T) {
	server := test_generate_server(t)
	err := server.Listen_And_Serve("localhost:0", 0)
	require.Nil(t, err, err)
	defer func() {
		err = server.Stop()
		require.Nil(t, err, err)
	}()
	http_proxy, err := New_Client("http://admin:123456@" + server.Addr())
	require.Nil(t, err, err)
	transport := &http.Transport{}
	http_proxy.HTTP(transport)
	client := http.Client{
		Transport: transport,
	}
	get := func(url string) {
		resp, err := client.Get(url)
		require.Nil(t, err, err)
		defer func() {
			_ = resp.Body.Close()
		}()
		_, err = ioutil.ReadAll(resp.Body)
		require.Nil(t, err, err)
	}
	get("http://20019.ip138.com/ic.asp")
	get("https://www.baidu.com/")
	// test other
	_, err = http_proxy.Dial("", "")
	require.Equal(t, err, ERR_NOT_SUPPORT_DIAL)
	_, err = http_proxy.Dial_Context(nil, "", "")
	require.Equal(t, err, ERR_NOT_SUPPORT_DIAL)
	_, err = http_proxy.Dial_Timeout("", "", 0)
	require.Equal(t, err, ERR_NOT_SUPPORT_DIAL)
	t.Log(http_proxy.Info())
}
