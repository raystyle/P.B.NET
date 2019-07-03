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

func test_restful_api(method, path string, body io.Reader) ([]byte, error) {
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

func test_json_encode(t *testing.T, m interface{}) io.Reader {
	buffer := &bytes.Buffer{}
	err := json.NewEncoder(buffer).Encode(m)
	require.Nil(t, err, err)
	return buffer
}

func test_shutdown() {
	_, _ = test_restful_api(http.MethodGet, "api/debug/shutdown", nil)
}

func Test_h_trust_node(t *testing.T) {
	init_ctrl(t)
	m := &m_trust_node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:9950",
	}
	reader := test_json_encode(t, m)
	resp, err := test_restful_api(http.MethodPost, "api/node/trust", reader)
	require.Nil(t, err, err)
	fmt.Println(string(resp))
	time.Sleep(1 * time.Second)
	test_shutdown()
	time.Sleep(1 * time.Second)
}
