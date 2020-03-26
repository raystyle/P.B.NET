package httptool

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

const (
	// post data length in one line
	bodyLineLength = 64

	// <security> prevent too big resp.Body
	maxBodyLength = 1024
)

// PrintRequest is used to print http.Request.
//
// client: 127.0.0.1:1234
// POST /index HTTP/1.1
// Host: github.com
// Accept: text/html
// Connection: keep-alive
// User-Agent: Mozilla
//
// post data...
// post data...
func PrintRequest(r *http.Request) *bytes.Buffer {
	buf := new(bytes.Buffer)
	_, _ = fmt.Fprintf(buf, "client: %s\n", r.RemoteAddr)
	// request
	_, _ = fmt.Fprintf(buf, "%s %s %s", r.Method, r.RequestURI, r.Proto)
	// host
	_, _ = fmt.Fprintf(buf, "\nHost: %s", r.Host)
	// header
	for k, v := range r.Header {
		_, _ = fmt.Fprintf(buf, "\n%s: %s", k, v[0])
	}
	if r.Body != nil {
		rawBody := new(bytes.Buffer)
		defer func() { r.Body = ioutil.NopCloser(io.MultiReader(rawBody, r.Body)) }()
		// start print
		buffer := make([]byte, bodyLineLength)
		// check body
		n, err := io.ReadFull(r.Body, buffer)
		if err != nil {
			if n == 0 { // no body
				return buf
			}
			// 0 < data size < bodyLineLength
			_, _ = fmt.Fprintf(buf, "\n\n%s", buffer[:n])
			rawBody.Write(buffer[:n])
			return buf
		}
		// new line and write data
		_, _ = fmt.Fprintf(buf, "\n\n%s", buffer)
		rawBody.Write(buffer)
		for {
			if rawBody.Len() > maxBodyLength {
				break
			}
			n, err := io.ReadFull(r.Body, buffer)
			if err != nil {
				// write last line
				if n != 0 {
					_, _ = fmt.Fprintf(buf, "\n%s", buffer[:n])
					rawBody.Write(buffer[:n])
				}
				break
			}
			_, _ = fmt.Fprintf(buf, "\n%s", buffer)
			rawBody.Write(buffer)
		}
	}
	return buf
}
