package controller

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/json-iterator/go"
	"github.com/stretchr/testify/require"

	"project/internal/xnet"
)

const host = "https://localhost:9931/"

func test_restful_api(method, path string, body io.Reader) ([]byte, error) {
	r, _ := http.NewRequest(method, host+path, body)
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
	err := jsoniter.NewEncoder(buffer).Encode(m)
	require.Nil(t, err, err)
	return buffer
}

func Test_h_trust_node(t *testing.T) {
	m := &m_trust_node{
		Mode:    xnet.TLS,
		Network: "tcp",
		Address: "localhost:1733",
	}
	reader := test_json_encode(t, m)
	resp, err := test_restful_api(http.MethodPost, "api/trust_node", reader)
	require.Nil(t, err, err)
	fmt.Println(string(resp))
}
