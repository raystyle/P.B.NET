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

	"project/internal/logger"
	"project/internal/option"
	"project/internal/patch/msgpack"
)

// MSFRPC is used to connect metasploit RPC service.
type MSFRPC struct {
	username string
	password string
	logger   logger.Logger

	url    string
	client *http.Client

	bufferPool  sync.Pool
	encoderPool sync.Pool
	decoderPool sync.Pool

	token string
	rwm   sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Options contains options about NewMSFRPC().
type Options struct {
	DisableTLS bool                 `toml:"disable_tls"`
	TLSVerify  bool                 `toml:"tls_verify"`
	Handler    string               `toml:"handler"` // custom URI
	Transport  option.HTTPTransport `toml:"transport"`
	Timeout    time.Duration        `toml:"timeout"`
	Token      string               `toml:"token"` // permanent token
}

// NewMSFRPC is used to create a new metasploit RPC connection.
func NewMSFRPC(
	address string,
	username string,
	password string,
	logger logger.Logger,
	opts *Options,
) (*MSFRPC, error) {
	if opts == nil {
		opts = new(Options)
	}
	// make http client
	tr, err := opts.Transport.Apply()
	if err != nil {
		return nil, err
	}
	if !opts.TLSVerify {
		tr.TLSClientConfig.InsecureSkipVerify = true
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
		username: username,
		password: password,
		logger:   logger,
		client:   &client,
		token:    opts.Token,
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
	msfrpc.url = fmt.Sprintf("%s://%s/%s", scheme, address, handler)
	// pool
	msfrpc.bufferPool.New = func() interface{} {
		buf := bytes.NewBuffer(make([]byte, 0, 64))
		return buf
	}
	msfrpc.encoderPool.New = func() interface{} {
		encoder := msgpack.NewEncoder(nil)
		encoder.StructAsArray(true)
		return encoder
	}
	msfrpc.decoderPool.New = func() interface{} {
		return msgpack.NewDecoder(nil)
	}
	msfrpc.ctx, msfrpc.cancel = context.WithCancel(context.Background())
	return &msfrpc, nil
}

func (msf *MSFRPC) send(ctx context.Context, request, response interface{}) error {
	return msf.sendWithReplace(ctx, request, response, nil)
}

// sendWithReplace is used to replace response to another response like CoreThreadList
// and MSFError if decode failed(return a MSFError).
func (msf *MSFRPC) sendWithReplace(ctx context.Context, request, response, replace interface{}) error {
	buf := msf.bufferPool.Get().(*bytes.Buffer)
	defer msf.bufferPool.Put(buf)
	buf.Reset()
	// pack request
	err := msf.encodeRequest(request, buf)
	if err != nil {
		return errors.WithStack(err)
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
		_, err = buf.ReadFrom(resp.Body)
		if err != nil {
			return errors.WithStack(err)
		}
		if replace == nil {
			return msf.decodeResponse(response, buf)
		}
		// try decode to response
		reader := bytes.NewReader(buf.Bytes())
		err = msf.decodeResponse(response, reader)
		if err == nil {
			return nil
		}
		// try decode to another
		reader.Reset(buf.Bytes())
		return msf.decodeResponse(replace, reader)
	case http.StatusInternalServerError:
		var msfErr MSFError
		err = msf.decodeResponse(&msfErr, resp.Body)
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

func (msf *MSFRPC) encodeRequest(request interface{}, buf *bytes.Buffer) error {
	encoder := msf.encoderPool.Get().(*msgpack.Encoder)
	defer msf.encoderPool.Put(encoder)
	encoder.Reset(buf)
	return encoder.Encode(request)
}

func (msf *MSFRPC) decodeResponse(response interface{}, reader io.Reader) error {
	decoder := msf.decoderPool.Get().(*msgpack.Decoder)
	defer msf.decoderPool.Put(decoder)
	decoder.Reset(reader)
	return decoder.Decode(response)
}

// SetToken is used to set token to current client.
func (msf *MSFRPC) SetToken(token string) {
	msf.rwm.Lock()
	defer msf.rwm.Unlock()
	msf.token = token
}

// GetToken is used to get token from current client.
func (msf *MSFRPC) GetToken() string {
	msf.rwm.RLock()
	defer msf.rwm.RUnlock()
	return msf.token
}

// AuthLogin is used to login metasploit RPC and get a temporary token. if use
// permanent token, dont need to call AuthLogin() but need AuthLogout().
func (msf *MSFRPC) AuthLogin() error {
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
		return errors.WithStack(&result.MSFError)
	}
	msf.SetToken(result.Token)
	return nil
}

// AuthLogout is used to remove the specified token from the authentication token list.
// Note that this method can be used to disable any temporary token, not just the one
// used by the current user. The permanent token will not be removed.
func (msf *MSFRPC) AuthLogout(token string) error {
	request := AuthLogoutRequest{
		Method:      MethodAuthLogout,
		Token:       msf.GetToken(),
		LogoutToken: token,
	}
	var result AuthLogoutResult
	err := msf.send(msf.ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// AuthTokenList is used to get token list.
func (msf *MSFRPC) AuthTokenList(ctx context.Context) ([]string, error) {
	request := AuthTokenListRequest{
		Method: MethodAuthTokenList,
		Token:  msf.GetToken(),
	}
	var result AuthTokenListResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return nil, err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return nil, errors.WithStack(&result.MSFError)
	}
	return result.Tokens, nil
}

// AuthTokenGenerate is used to create a random 32-byte authentication token,
// add this token to the authenticated list, and return this token.
func (msf *MSFRPC) AuthTokenGenerate(ctx context.Context) (string, error) {
	request := AuthTokenGenerateRequest{
		Method: MethodAuthTokenGenerate,
		Token:  msf.GetToken(),
	}
	var result AuthTokenGenerateResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return "", err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return "", errors.WithStack(&result.MSFError)
	}
	return result.Token, nil
}

// AuthTokenAdd is used to add an arbitrary string as a valid permanent authentication
// token. This token can be used for all future authentication purposes.
func (msf *MSFRPC) AuthTokenAdd(ctx context.Context, token string) error {
	request := AuthTokenAddRequest{
		Method:   MethodAuthTokenAdd,
		Token:    msf.GetToken(),
		NewToken: token,
	}
	var result AuthTokenAddResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// AuthTokenRemove is used to delete a specified token. This will work for both
// temporary and permanent tokens, including those stored in the database backend.
func (msf *MSFRPC) AuthTokenRemove(ctx context.Context, token string) error {
	request := AuthTokenRemoveRequest{
		Method:           MethodAuthTokenRemove,
		Token:            msf.GetToken(),
		TokenToBeRemoved: token,
	}
	var result AuthTokenRemoveResult
	err := msf.send(ctx, &request, &result)
	if err != nil {
		return err
	}
	if result.Err {
		if result.ErrorMessage == ErrInvalidToken {
			result.ErrorMessage = ErrInvalidTokenFriendly
		}
		return errors.WithStack(&result.MSFError)
	}
	return nil
}

// Close is used to logout metasploit RPC and destroy all objects.
func (msf *MSFRPC) Close() error {
	err := msf.clean()
	if err != nil {
		return err
	}
	err = msf.AuthLogout(msf.GetToken())
	if err != nil {
		return err
	}
	msf.close()
	return nil
}

// Kill is ued to logout metasploit RPC when can't connect target.
func (msf *MSFRPC) Kill() {
	_ = msf.clean()
	_ = msf.AuthLogout(msf.GetToken())
	msf.close()
}

func (msf *MSFRPC) clean() error {
	// close all console
	return nil
}

func (msf *MSFRPC) close() {
	msf.cancel()
	msf.client.CloseIdleConnections()
	msf.wg.Wait()
}

func (msf *MSFRPC) logf(lv logger.Level, format string, log ...interface{}) {
	msf.logger.Printf(lv, "msfrpc", format, log...)
}

func (msf *MSFRPC) log(lv logger.Level, log ...interface{}) {
	msf.logger.Println(lv, "msfrpc", log...)
}
