package msfrpc

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/patch/toml"
	"project/internal/security"
	"project/internal/testsuite"
)

func TestWebUI(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	mux := http.NewServeMux()

	hfs := http.Dir("testdata/web")
	webUI, err := newWebUI(hfs, mux)
	require.NoError(t, err)
	require.NotNil(t, webUI)

	server := http.Server{
		Addr:    "localhost:0",
		Handler: mux,
	}
	defer func() { _ = server.Close() }()
	port := testsuite.RunHTTPServer(t, "tcp", &server)

	client := http.Client{Transport: new(http.Transport)}
	defer client.CloseIdleConnections()

	url := fmt.Sprintf("http://localhost:%s/", port)
	for _, item := range [...]*struct {
		path string
		data string
	}{
		{"favicon.ico", "test favicon"},

		{"", "test index"},
		{"index.html", "test index"},
		{"index.htm", "test index"},
		{"index", "test index"},

		{"css/test.css", "test css"},
		{"js/test.js", `let test = "javascript"`},
		{"img/test.jpg", "test image"},
		{"fonts/test.ttf", "test fonts"},
	} {
		resp, err := client.Get(url + item.path)
		require.NoError(t, err)
		b, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, []byte(item.data), b)
	}

	err = webUI.Reload()
	require.NoError(t, err)
}

func TestWebUI_Reload(t *testing.T) {
	hfs := http.Dir("testdata/web")
	t.Run("failed to open", func(t *testing.T) {
		patch := func(interface{}, string) (http.File, error) {
			return nil, monkey.Error
		}
		pg := monkey.PatchInstanceMethod(hfs, "Open", patch)
		defer pg.Unpatch()

		webUI, err := newWebUI(hfs, nil)
		require.Error(t, err)
		require.Nil(t, webUI)
	})

	t.Run("failed to read", func(t *testing.T) {
		patch := func(io.Reader, int64) ([]byte, error) {
			return nil, monkey.Error
		}
		pg := monkey.Patch(security.ReadAll, patch)
		defer pg.Unpatch()

		webUI, err := newWebUI(hfs, nil)
		require.Error(t, err)
		require.Nil(t, webUI)
	})
}

func TestWebAPI_Login(t *testing.T) {
	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

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
		{expected: "Admin", actual: opts.AdminDisplayName},
		{expected: true, actual: opts.DisableTLS},
		{expected: 1000, actual: opts.MaxConns},
		{expected: time.Minute, actual: opts.Timeout},
		{expected: int64(1024), actual: opts.MaxBodySize},
		{expected: int64(10240), actual: opts.MaxLargeBodySize},
		{expected: true, actual: opts.APIOnly},
		{expected: 30 * time.Second, actual: opts.Server.ReadTimeout},
	} {
		require.Equal(t, testdata.expected, testdata.actual)
	}
}
