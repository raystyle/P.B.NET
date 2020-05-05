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

// FprintRequest is used to print *http.Request to a io.Writer.
//
// Remote: 127.0.0.1:1234
// POST /index HTTP/1.1
// Host: github.com
// Accept: text/html
// Connection: keep-alive
// User-Agent: Mozilla
//
// post data...
// post data...
func FprintRequest(w io.Writer, r *http.Request) (int, error) {
	n, err := fmt.Fprintf(w, "Remote: %s\n", r.RemoteAddr)
	if err != nil {
		return n, err
	}
	var nn int
	// request
	nn, err = fmt.Fprintf(w, "%s %s %s", r.Method, r.RequestURI, r.Proto)
	if err != nil {
		return n + nn, err
	}
	n += nn
	// host
	nn, err = fmt.Fprintf(w, "\nHost: %s", r.Host)
	if err != nil {
		return n + nn, err
	}
	n += nn
	// header
	for k, v := range r.Header {
		nn, err = fmt.Fprintf(w, "\n%s: %s", k, v[0])
		if err != nil {
			return n + nn, err
		}
		n += nn
	}
	if r.Body == nil {
		return n, nil
	}
	nn, err = printBody(w, r)
	return n + nn, err
}

func printBody(w io.Writer, r *http.Request) (int, error) {
	rawBody := new(bytes.Buffer)
	defer func() { r.Body = ioutil.NopCloser(io.MultiReader(rawBody, r.Body)) }()
	var (
		total int
		err   error
	)
	// check body
	buffer := make([]byte, bodyLineLength)
	n, err := io.ReadFull(r.Body, buffer)
	if err != nil {
		if n == 0 { // no body
			return 0, nil
		}
		// 0 < data size < bodyLineLength
		nn, err := fmt.Fprintf(w, "\n\n%s", buffer[:n])
		if err != nil {
			return nn, err
		}
		rawBody.Write(buffer[:n])
		return n, nil
	}
	// new line and write data
	n, err = fmt.Fprintf(w, "\n\n%s", buffer)
	if err != nil {
		return n, err
	}
	total += n
	rawBody.Write(buffer)
	for {
		if rawBody.Len() > maxBodyLength {
			break
		}
		n, err = io.ReadFull(r.Body, buffer)
		if err != nil {
			// write last line
			if n != 0 {
				nn, err := fmt.Fprintf(w, "\n%s", buffer[:n])
				if err != nil {
					return total + nn, err
				}
				rawBody.Write(buffer[:n])
			}
			break
		}
		n, err = fmt.Fprintf(w, "\n%s", buffer)
		if err != nil {
			return total + n, err
		}
		total += n
		rawBody.Write(buffer)
	}
	return total, nil
}

// PrintRequest is used to print *http.Request to a buffer.
func PrintRequest(r *http.Request) *bytes.Buffer {
	buf := new(bytes.Buffer)
	_, _ = FprintRequest(buf, r)
	return buf
}

// subHTTPFileSystem is used to open sub directory for http file server.
type subHTTPFileSystem struct {
	hfs  http.FileSystem
	path string
}

// NewSubHTTPFileSystem is used to create a new sub http file system.
func NewSubHTTPFileSystem(hfs http.FileSystem, path string) http.FileSystem {
	return &subHTTPFileSystem{hfs: hfs, path: path + "/"}
}

func (s *subHTTPFileSystem) Open(name string) (http.File, error) {
	return s.hfs.Open(s.path + name)
}
