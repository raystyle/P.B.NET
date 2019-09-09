package controller

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/xnet"
)

func testRestfulAPI(method, path string, body io.Reader) ([]byte, error) {
	r, _ := http.NewRequest(method, "https://localhost:9931/"+path, body)
	t := &http.Transport{}
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	c := http.Client{Transport: t}
	resp, err := c.Do(r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	return ioutil.ReadAll(resp.Body)
}

func testEncodeJSON(t *testing.T, m interface{}) io.Reader {
	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(m)
	require.NoError(t, err)
	return buffer
}

func testShutdown() {
	_, _ = testRestfulAPI(http.MethodGet, "api/debug/shutdown", nil)
}

func TestHandleTrustNode(t *testing.T) {
	initCtrl(t)
	m := &mTrustNode{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:9950",
	}
	reader := testEncodeJSON(t, m)
	resp, err := testRestfulAPI(http.MethodPost, "api/node/trust", reader)
	require.NoError(t, err)
	fmt.Println("trust node result:", string(resp))
	testShutdown()
	time.Sleep(2 * time.Second)
}
