package timesync

import (
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"project/internal/options"
)

func queryHTTPServer(req *http.Request, client *http.Client) (time.Time, error) {
	if client.Timeout < 1 {
		client.Timeout = options.DefaultDialTimeout
	}
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer func() {
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		_ = resp.Body.Close()
		client.CloseIdleConnections()
	}()
	return http.ParseTime(resp.Header.Get("Date"))
}
