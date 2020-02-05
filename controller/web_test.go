package controller

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"project/internal/xnet"
)

func TestRunWebServer(t *testing.T) {
	testInitializeController(t)
	time.Sleep(5 * time.Minute)
}

func testRestfulAPI(method, path string, model interface{}) ([]byte, error) {
	// json
	buf := bytes.Buffer{}
	if model != nil {
		err := json.NewEncoder(&buf).Encode(model)
		if err != nil {
			return nil, err
		}
	}
	r, _ := http.NewRequest(method, "https://localhost:9931/"+path, &buf)
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

func TestHandleTrustNode(t *testing.T) {
	Node := testGenerateInitialNode(t)
	defer Node.Exit(nil)
	m := &mTrustNode{
		Mode:    xnet.ModeTLS,
		Network: "tcp",
		Address: "localhost:9950",
	}
	resp, err := testRestfulAPI(http.MethodPost, "api/node/trust", m)
	require.NoError(t, err)
	t.Log("trust node result:", string(resp))
}
