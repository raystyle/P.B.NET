package msfrpc

import (
	"bytes"
	"context"
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
	username string
	password string
	url      string
	client   *http.Client

	bufPool sync.Pool
	token   string
	rwm     sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Options contains options about NewMSFRPC().
type Options struct {
	DisableTLS bool
	Handler    string // URI
	Transport  option.HTTPTransport
	Timeout    time.Duration
}

// NewMSFRPC is used to create a new metasploit RPC connection.
func NewMSFRPC(host string, port uint16, username, password string, opts *Options) (*MSFRPC, error) {
	if opts == nil {
		opts = new(Options)
	}
	// make http client
	tr, err := opts.Transport.Apply()
	if err != nil {
		return nil, err
	}
	tr.TLSClientConfig.InsecureSkipVerify = true
	client := http.Client{
		Transport: tr,
		Timeout:   opts.Timeout,
	}
	if client.Timeout < 1 {
		client.Timeout = 30 * time.Second
	}
	// url
	msfrpc := MSFRPC{
		username: username,
		password: password,
		client:   &client,
	}
	var scheme string
	if opts.DisableTLS {
		scheme = "http"
	} else {
		scheme = "https"
	}
	var handler string
	if opts.Handler == "" {
		handler = "api"
	} else {
		handler = opts.Handler
	}
	msfrpc.url = fmt.Sprintf("%s://%s:%d/%s", scheme, host, port, handler)
	// buffer pool
	msfrpc.bufPool.New = func() interface{} {
		buf := bytes.NewBuffer(make([]byte, 0, 64))
		return buf
	}
	msfrpc.ctx, msfrpc.cancel = context.WithCancel(context.Background())
	return &msfrpc, nil
}

func (msf *MSFRPC) send(ctx context.Context, request, response interface{}) error {
	buf := msf.bufPool.Get().(*bytes.Buffer)
	defer msf.bufPool.Put(buf)
	buf.Reset()

	// pack request
	if _, ok := request.(asArray); ok {
		encoder := msgpack.NewEncoder(buf)
		encoder.StructAsArray(true)
		err := encoder.Encode(request)
		if err != nil {
			return errors.WithStack(err)
		}
	} else {
		err := msgpack.NewEncoder(buf).Encode(request)
		if err != nil {
			return errors.WithStack(err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msf.url, buf)
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
		var msfErr MSFError
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

func (msf *MSFRPC) setToken(token string) {
	msf.rwm.Lock()
	defer msf.rwm.Unlock()
	msf.token = token
}

func (msf *MSFRPC) getToken() string {
	msf.rwm.RLock()
	defer msf.rwm.RUnlock()
	return msf.token
}

// Login is used to login metasploit RPC and get token.
func (msf *MSFRPC) Login() error {
	request := AuthLoginRequest{
		Method:   MethodAuthLogin,
		Username: msf.username,
		Password: msf.password,
	}
	var result AuthLoginResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		return &result.MSFError
	}
	msf.setToken(result.Token)
	return nil
}

// Logout is used to delete token.
func (msf *MSFRPC) Logout(token string) error {
	request := AuthLogoutRequest{
		Method:      MethodAuthLogout,
		Token:       msf.getToken(),
		LogoutToken: token,
	}
	var result AuthLogoutResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		return &result.MSFError
	}
	if result.Result != success {
		return errors.New(result.Result)
	}
	msf.setToken("")
	return nil
}

// Close is used to logout metasploit RPC.
func (msf *MSFRPC) Close() error {
	err := msf.Logout(msf.getToken())
	if err != nil {
		return err
	}
	msf.close()
	return nil
}

// Kill is ued to logout metasploit RPC when can't connect target.
func (msf *MSFRPC) Kill() {
	_ = msf.Logout(msf.getToken())
	msf.close()
}

func (msf *MSFRPC) close() {
	msf.cancel()
	msf.client.CloseIdleConnections()
	msf.wg.Wait()
}
