package msfrpc

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/option"
	"project/internal/patch/msgpack"
)

// MSFRPC is used to connect metasploit RPC service.
type MSFRPC struct {
	url    string
	client *http.Client

	bufPool sync.Pool
	token   string
}

// Options contains options about NewMSFRPC().
type Options struct {
	TLS       bool
	Transport option.HTTPTransport
	Timeout   time.Duration
}

// NewMSFRPC is used to create a new metasploit RPC connection.
func NewMSFRPC(host string, port uint16, opts *Options) (*MSFRPC, error) {
	// make http client
	tr, err := opts.Transport.Apply()
	if err != nil {
		return nil, err
	}
	client := http.Client{
		Transport: tr,
		Timeout:   opts.Timeout,
	}
	if client.Timeout < 1 {
		client.Timeout = 30 * time.Second
	}
	// url
	msfrpc := MSFRPC{
		client: &client,
	}
	var scheme string
	if opts.TLS {
		scheme = "https"
	} else {
		scheme = "http"
	}
	msfrpc.url = fmt.Sprintf("%s://%s:%d/api", scheme, host, port)
	// buffer pool
	msfrpc.bufPool.New = func() interface{} {
		buf := bytes.NewBuffer(make([]byte, 0, 64))
		return buf
	}
	return &msfrpc, nil
}

func (msf *MSFRPC) send(request, response interface{}) error {
	buf := msf.bufPool.Get().(*bytes.Buffer)
	defer msf.bufPool.Put(buf)
	buf.Reset()

	// pack request
	err := msgpack.NewEncoder(buf).Encode(request)
	if err != nil {
		return errors.WithStack(err)
	}
	req, err := http.NewRequest(http.MethodPost, msf.url, buf)
	if err != nil {
		return errors.WithStack(err)
	}
	req.Header.Set("Content-Type", "binary/message-pack")
	req.Header.Set("Accept", "binary/message-pack")
	req.Header.Set("Accept-Charset", "UTF-8")

	// do
	resp, err := msf.client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = resp.Body.Close() }()

	// read response body
	switch resp.StatusCode {
	case http.StatusOK:
		return msgpack.NewDecoder(resp.Body).Decode(response)
	case http.StatusInternalServerError:
		var msfErr msfError
		err = msgpack.NewDecoder(resp.Body).Decode(&msfErr)
		if err != nil {
			return errors.WithStack(err)
		}
		return errors.WithStack(&msfErr)
	case http.StatusUnauthorized:
		err = errors.New("token is invalid")
	case http.StatusForbidden:
		err = errors.New("token is not granted access to the resource")
	case http.StatusNotFound:
		err = errors.New("the request was sent to an invalid URL")
	default:
		err = errors.New(resp.Status)
	}
	_, _ = io.Copy(ioutil.Discard, resp.Body)
	return err
}

// Login is used to login metasploit RPC and get token.
func (msf *MSFRPC) Login() error {
	return nil
}

// Logout is used to logout metasploit RPC and delete token.
func (msf *MSFRPC) Logout() error {
	return nil
}

// LogoutForce is used to logout metasploit RPC when can't connect target.
func (msf *MSFRPC) LogoutForce() {

}
