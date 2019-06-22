package logger

import (
	"fmt"
	"net/http"
)

// TODO print more info

//     address: 127.0.0.1:2275
//     GET /index.html
//     aaa:asdasdasd
func HTTP_Request(r *http.Request) string {
	return fmt.Sprintf("address: %s\n%s %s",
		r.RemoteAddr, r.Method, r.RequestURI)
}
