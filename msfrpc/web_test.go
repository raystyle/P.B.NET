package msfrpc

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/toml"
	"project/internal/testsuite"
)

func TestWebUI(t *testing.T) {
	mux := http.NewServeMux()

	hfs := http.Dir("testdata/web")
	webUI, err := newWebUI(hfs, mux)
	require.NoError(t, err)

	server := http.Server{Handler: mux}

	listener, err := net.Listen("tcp", "127.0.0.1:8181")
	require.NoError(t, err)
	server.Serve(listener)

	fmt.Println(webUI)
}

func TestWebOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/web_opts.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := WebOptions{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	// check zero value
	testsuite.CheckOptions(t, opts)

	for _, testdata := range [...]*struct {
		expected interface{}
		actual   interface{}
	}{
		{expected: "admin", actual: opts.AdminUsername},
		{expected: "bcrypt", actual: opts.AdminPassword},
		{expected: 1000, actual: opts.MaxConns},
		{expected: int64(1024), actual: opts.MaxBodySize},
		{expected: int64(10240), actual: opts.MaxLargeBodySize},
		{expected: true, actual: opts.APIOnly},
		{expected: 30 * time.Second, actual: opts.Server.ReadTimeout},
		{expected: "bcrypt", actual: opts.Users["user"]},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
