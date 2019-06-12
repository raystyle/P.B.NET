package httptime

import (
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"project/internal/options"
)

// for time sync
func Query(req *http.Request, opt *http.Client) (time.Time, error) {
	if opt == nil {
		opt = new(http.Client)
	}
	if opt.Timeout <= 0 {
		opt.Timeout = options.DEFAULT_DIAL_TIMEOUT
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
