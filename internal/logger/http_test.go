package logger

import (
	"net/http"
	"testing"
)

func Test_HTTP_Request(t *testing.T) {
	r := &http.Request{
		Method:     http.MethodGet,
		RequestURI: "/",
		RemoteAddr: "127.0.0.1:1234",
	}
	t.Log(HTTP_Request(r))
}
