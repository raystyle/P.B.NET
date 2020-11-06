package msfrpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"project/internal/patch/monkey"
	"project/internal/patch/toml"
	"project/internal/security"
	"project/internal/testsuite"
)

var (
	testMSFRPC     *MSFRPC
	testMSFRPCURL  string
	testHTTPClient *http.Client
	testWSClient   websocket.Dialer
	testInitOnce   sync.Once
)

func testMainCheckMSFRPCLeaks() bool {
	if testMSFRPC == nil {
		return false
	}
	testMSFRPC.Exit()
	// must copy, because it is a global variable
	testMSFRPCCp := testMSFRPC
	testMSFRPC = nil
	if !testsuite.Destroyed(testMSFRPCCp) {
		fmt.Println("[warning] msfrpc is not destroyed")
		return true
	}
	// close http client
	testHTTPClient.CloseIdleConnections()
	return false
}

func testInitializeMSFRPC(t testing.TB) {
	testInitOnce.Do(func() {
		cfg := testGenerateConfig()
		testMSFRPC = testGenerateMSFRPC(t, cfg)

		// let http.Client.Transport contain persistConn
		time.Sleep(minWatchInterval * 5)

		testMSFRPCURL = fmt.Sprintf("http://%s/", testMSFRPC.Addresses()[0])

		// create http client
		jar, _ := cookiejar.New(nil)
		testHTTPClient = &http.Client{
			Transport: new(http.Transport),
			Jar:       jar,
			Timeout:   30 * time.Second,
		}

		// create websocket client
		testWSClient.EnableCompression = true
		testWSClient.Jar = jar
	})
}

func testHTTPClientGET(t *testing.T, path string, resp interface{}) {
	response, err := testHTTPClient.Get(testMSFRPCURL + path)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)

	err = json.NewDecoder(response.Body).Decode(resp)
	require.NoError(t, err)
}

func testHTTPClientPOST(t *testing.T, path string, data, resp interface{}) {
	buf := bytes.NewBuffer(make([]byte, 0, 128))
	err := json.NewEncoder(buf).Encode(data)
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, testMSFRPCURL+path, buf)
	require.NoError(t, err)

	response, err := testHTTPClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)

	err = json.NewDecoder(response.Body).Decode(resp)
	require.NoError(t, err)
}

func TestWeb_Login(t *testing.T) {
	testInitializeMSFRPC(t)

	gm := testsuite.MarkGoroutines(t)
	defer gm.Compare()

	t.Run("success", func(t *testing.T) {
		req := &struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}{}
		resp := &struct {
			Username    string `json:"username"`
			UserGroup   string `json:"user_group"`
			DisplayName string `json:"display_name"`
		}{}
		for _, item := range [...]*struct {
			username    string
			password    string
			userGroup   string
			displayName string
		}{
			{"admin", "admin", UserGroupAdmins, "Admin"},
			{"manager", "test", UserGroupManagers, "Manager"},
			{"user", "test", UserGroupUsers, "User"},
			{"guest", "test", UserGroupGuests, "Guest"},
		} {
			req.Username = item.username
			req.Password = item.password
			testHTTPClientPOST(t, "api/login", req, resp)

			require.Equal(t, item.username, resp.Username)
			require.Equal(t, item.userGroup, resp.UserGroup)
			require.Equal(t, item.displayName, resp.DisplayName)
		}
	})
}

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

func TestWebAPI_loadUserInfo(t *testing.T) {

}

func TestWebOptions(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/web_opts.toml")
	require.NoError(t, err)

	// check unnecessary field
	opts := WebOptions{}
	err = toml.Unmarshal(data, &opts)
	require.NoError(t, err)

	// check zero value
	testsuite.ContainZeroValue(t, opts)

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
