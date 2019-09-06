package timesync

import (
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"project/internal/options"
)

func queryHTTPServer(req *http.Request, opt *http.Client) (time.Time, error) {
	if opt.Timeout < 1 {
		opt.Timeout = options.DefaultDialTimeout
	}
	resp, err := opt.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer func() {
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		_ = resp.Body.Close()
		opt.CloseIdleConnections()
	}()
	return http.ParseTime(resp.Header.Get("Date"))
}
