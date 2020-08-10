package msfrpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/option"
	"project/internal/patch/msgpack"
	"project/internal/xsync"
)

// MSFRPC is used to connect metasploit RPC service.
type MSFRPC struct {
	username string
	password string
	logger   logger.Logger

	url    string
	client *http.Client

	encoderPool  sync.Pool
	decoderPool  sync.Pool
	rDecoderPool sync.Pool

	token    string
	tokenRWM sync.RWMutex

	// key = console id
	consoles map[string]*Console
	// key = shell session id
	shells map[uint64]*Shell
	// key = meterpreter session id
	meterpreters map[uint64]*Meterpreter
	// io resource counter
	counter xsync.Counter

	inShutdown int32
	rwm        sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
}

// Options contains options about NewMSFRPC().
type Options struct {
	DisableTLS bool                 `toml:"disable_tls"`
	TLSVerify  bool                 `toml:"tls_verify"`
	Handler    string               `toml:"handler"` // custom URI
	Transport  option.HTTPTransport `toml:"transport" check:"-"`
	Timeout    time.Duration        `toml:"timeout"`
	Token      string               `toml:"token"` // permanent token
}

type bufEncoder struct {
	buf     *bytes.Buffer
	encoder *msgpack.Encoder
}

type bufDecoder struct {
	buf     *bytes.Buffer
	decoder *msgpack.Decoder
}

type readerDecoder struct {
	reader  *bytes.Reader
	decoder *msgpack.Decoder
}

// NewMSFRPC is used to create a new metasploit RPC connection.
func NewMSFRPC(address, username, password string, lg logger.Logger, opts *Options) (*MSFRPC, error) {
	if opts == nil {
		opts = new(Options)
	}
	// make http client
	tr, err := opts.Transport.Apply()
	if err != nil {
		return nil, err
	}
	// cover options about max connection
	if opts.Transport.MaxIdleConns < 1 {
		tr.MaxIdleConns = 32
	}
	if opts.Transport.MaxIdleConnsPerHost < 1 {
		tr.MaxIdleConnsPerHost = 32
	}
	if opts.Transport.MaxConnsPerHost < 1 {
		tr.MaxConnsPerHost = 32
	}
	// tls
	if !opts.TLSVerify {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}
	// make http client
	jar, _ := cookiejar.New(nil)
	timeout := opts.Timeout
	if timeout < 1 {
		timeout = 30 * time.Second
	}
	client := http.Client{
		Transport: tr,
		Jar:       jar,
		Timeout:   timeout,
	}
	// url
	msfrpc := MSFRPC{
		username:     username,
		password:     password,
		logger:       lg,
		client:       &client,
		token:        opts.Token,
		consoles:     make(map[string]*Console),
		shells:       make(map[uint64]*Shell),
		meterpreters: make(map[uint64]*Meterpreter),
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
	// sync pool
	msfrpc.encoderPool.New = func() interface{} {
		buf := bytes.NewBuffer(make([]byte, 0, 64))
		encoder := msgpack.NewEncoder(buf)
		encoder.UseArrayEncodedStructs(true)
		return &bufEncoder{
			buf:     buf,
			encoder: encoder,
		}
	}
	msfrpc.decoderPool.New = func() interface{} {
		buf := bytes.NewBuffer(make([]byte, 0, 64))
		decoder := msgpack.NewDecoder(buf)
		return &bufDecoder{
			buf:     buf,
			decoder: decoder,
		}
	}
	msfrpc.rDecoderPool.New = func() interface{} {
		reader := bytes.NewReader(nil)
		decoder := msgpack.NewDecoder(reader)
		return &readerDecoder{
			reader:  reader,
			decoder: decoder,
		}
	}
	msfrpc.ctx, msfrpc.cancel = context.WithCancel(context.Background())
	return &msfrpc, nil
}

// HijackLogWriter is used to hijack all packages that use log.Print().
func (msf *MSFRPC) HijackLogWriter() {
	logger.HijackLogWriter(logger.Error, "pkg", msf.logger)
}

// SetToken is used to set token to current client.
func (msf *MSFRPC) SetToken(token string) {
	msf.tokenRWM.Lock()
	defer msf.tokenRWM.Unlock()
	msf.token = token
}

// GetToken is used to get token from current client.
func (msf *MSFRPC) GetToken() string {
	msf.tokenRWM.RLock()
	defer msf.tokenRWM.RUnlock()
	return msf.token
}

func (msf *MSFRPC) send(ctx context.Context, request, response interface{}) error {
	return msf.sendWithReplace(ctx, request, response, nil)
}

// sendWithReplace is used to replace response to another response like CoreThreadList
// and MSFError if decode failed(return a MSFError).
func (msf *MSFRPC) sendWithReplace(ctx context.Context, request, response, replace interface{}) error {
	// pack request
	be := msf.encoderPool.Get().(*bufEncoder)
	defer msf.encoderPool.Put(be)
	be.buf.Reset()
	err := be.encoder.Encode(request)
	if err != nil {
		return errors.WithStack(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msf.url, be.buf)
	if err != nil {
		return errors.WithStack(err)
	}
	header := req.Header
	header.Set("Content-Type", "binary/message-pack")
	header.Set("Accept", "binary/message-pack")
	header.Set("Accept-Charset", "utf-8")
	header.Set("Connection", "keep-alive")
	// do
	resp, err := msf.client.Do(req)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	// read response body
	switch resp.StatusCode {
	case http.StatusOK:
		bd := msf.decoderPool.Get().(*bufDecoder)
		defer msf.decoderPool.Put(bd)
		bd.buf.Reset()
		_, err = bd.buf.ReadFrom(resp.Body)
		if err != nil {
			return errors.WithStack(err)
		}
		if replace == nil {
			return bd.decoder.Decode(response)
		}
		// first try to decode to response
		rd := msf.rDecoderPool.Get().(*readerDecoder)
		defer msf.rDecoderPool.Put(rd)
		rd.reader.Reset(bd.buf.Bytes())
		err = rd.decoder.Decode(response)
		if err == nil {
			return nil
		}
		// then try to decode to replace
		return bd.decoder.Decode(replace)
	case http.StatusInternalServerError:
		bd := msf.decoderPool.Get().(*bufDecoder)
		defer msf.decoderPool.Put(bd)
		bd.buf.Reset()
		_, err = bd.buf.ReadFrom(resp.Body)
		if err != nil {
			return errors.WithStack(err)
		}
		msfErr := new(MSFError)
		err = bd.decoder.Decode(msfErr)
		if err != nil {
			return errors.WithStack(err)
		}
		return errors.WithStack(msfErr)
	case http.StatusUnauthorized:
		err = errors.New("invalid token")
	case http.StatusForbidden:
		err = errors.New("this token is not granted access to the resource")
	case http.StatusNotFound:
		err = errors.New("the request was sent to an invalid URL")
	default:
		err = errors.Errorf("unexpected http status code: %d", resp.StatusCode)
	}
	return err
}

func (msf *MSFRPC) logf(lv logger.Level, format string, log ...interface{}) {
	msf.logger.Printf(lv, "msfrpc", format, log...)
}

func (msf *MSFRPC) log(lv logger.Level, log ...interface{}) {
	msf.logger.Println(lv, "msfrpc", log...)
}

func (msf *MSFRPC) shuttingDown() bool {
	return atomic.LoadInt32(&msf.inShutdown) != 0
}

func (msf *MSFRPC) addIOResourceCount(delta int) {
	msf.counter.Add(delta)
}

func (msf *MSFRPC) trackConsole(console *Console, add bool) bool {
	msf.rwm.Lock()
	defer msf.rwm.Unlock()
	if add {
		if msf.shuttingDown() {
			return false
		}
		msf.consoles[console.id] = console
	} else {
		delete(msf.consoles, console.id)
	}
	return true
}

func (msf *MSFRPC) trackShell(shell *Shell, add bool) bool {
	msf.rwm.Lock()
	defer msf.rwm.Unlock()
	if add {
		if msf.shuttingDown() {
			return false
		}
		msf.shells[shell.id] = shell
	} else {
		delete(msf.shells, shell.id)
	}
	return true
}

func (msf *MSFRPC) trackMeterpreter(mp *Meterpreter, add bool) bool {
	msf.rwm.Lock()
	defer msf.rwm.Unlock()
	if add {
		if msf.shuttingDown() {
			return false
		}
		msf.meterpreters[mp.id] = mp
	} else {
		delete(msf.meterpreters, mp.id)
	}
	return true
}

// GetConsole is used to get console by id.
func (msf *MSFRPC) GetConsole(id string) (*Console, error) {
	msf.rwm.RLock()
	defer msf.rwm.RUnlock()
	if console, ok := msf.consoles[id]; ok {
		return console, nil
	}
	return nil, errors.Errorf("console \"%s\" doesn't exist", id)
}

// GetShell is used to get shell by id.
func (msf *MSFRPC) GetShell(id uint64) (*Shell, error) {
	msf.rwm.RLock()
	defer msf.rwm.RUnlock()
	if shell, ok := msf.shells[id]; ok {
		return shell, nil
	}
	return nil, errors.Errorf("shell \"%d\" doesn't exist", id)
}

// GetMeterpreter is used to get meterpreter by id.
func (msf *MSFRPC) GetMeterpreter(id uint64) (*Meterpreter, error) {
	msf.rwm.RLock()
	defer msf.rwm.RUnlock()
	if meterpreter, ok := msf.meterpreters[id]; ok {
		return meterpreter, nil
	}
	return nil, errors.Errorf("meterpreter \"%d\" doesn't exist", id)
}

// Close is used to logout metasploit RPC and destroy all objects.
func (msf *MSFRPC) Close() error {
	err := msf.AuthLogout(msf.GetToken())
	if err != nil {
		return err
	}
	msf.close()
	msf.counter.Wait()
	return nil
}

// Kill is used to logout metasploit RPC when can't connect target.
func (msf *MSFRPC) Kill() {
	err := msf.AuthLogout(msf.GetToken())
	if err != nil {
		msf.log(logger.Warning, "appear error when kill msfrpc:", err)
	}
	msf.close()
	msf.counter.Wait()
}

func (msf *MSFRPC) close() {
	msf.rwm.Lock()
	defer msf.rwm.Unlock()
	atomic.StoreInt32(&msf.inShutdown, 1)
	// close all consoles
	for _, console := range msf.consoles {
		_ = console.Close()
	}
	// close all shells
	for _, shell := range msf.shells {
		_ = shell.Close()
	}
	// close all meterpreters
	for _, meterpreter := range msf.meterpreters {
		_ = meterpreter.Close()
	}
	msf.cancel()
	msf.client.CloseIdleConnections()
}
