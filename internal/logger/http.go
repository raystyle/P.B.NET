package logger

import (
	"fmt"
	"net/http"
)

// TODO print more info
//     address: 127.0.0.1:2275
//     GET /index.html
//     foo:foo
func HTTPRequest(r *http.Request) string {
	return fmt.Sprintf("address: %s\n%s %s",
		r.RemoteAddr, r.Method, r.RequestURI)
}
