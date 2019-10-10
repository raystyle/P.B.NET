package timesync

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/options"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/xpanic"
)

type Mode = string

const (
	HTTP Mode = "http" // get response header: Date
	NTP  Mode = "ntp"
)

const (
	defaultSyncInterval = 15 * time.Minute
	addLoopInterval     = 500 * time.Millisecond
)

var (
	ErrNoConfigs = errors.New("no time sync config")
	ErrAllFailed = errors.New("time sync all failed")
	ErrInterval  = errors.New("interval < 60s or > 1h")
	ErrQueryHTTP = errors.New("query http server failed")
	ErrQueryNTP  = errors.New("query ntp server failed")
)

type Config struct {
	Mode     Mode          `toml:"mode"`
	Address  string        `toml:"address"` // if Mode == HTTP cover H_Request.URL
	Timeout  time.Duration `toml:"timeout"`
	ProxyTag string        `toml:"proxy_tag"`
	DNSOpts  dns.Options   `toml:"dns_options"`
	// for queryHttp
	HTTPOpts struct {
		Request   options.HTTPRequest   `toml:"request"`
		Transport options.HTTPTransport `toml:"transport"`
	} `toml:"http_options"`
	// for queryNTP
	NTPOpts struct {
		Version int    `toml:"version"`
		Network string `toml:"network"`
	} `toml:"ntp_options"`
}

type TimeSyncer struct {
	proxyPool  *proxy.Pool        // ctx
	dnsClient  *dns.Client        // ctx
	logger     logger.Logger      // ctx
	configs    map[string]*Config // key = tag
	configsRWM sync.RWMutex
	interval   time.Duration // sync interval
	now        time.Time
	nowRWM     sync.RWMutex // now
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

func NewTimeSyncer(
	pool *proxy.Pool,
	client *dns.Client,
	logger logger.Logger,
	configs map[string]*Config,
	interval time.Duration,
) (*TimeSyncer, error) {
	ts := &TimeSyncer{
		proxyPool:  pool,
		dnsClient:  client,
		logger:     logger,
		now:        time.Now(),
		configs:    make(map[string]*Config),
		stopSignal: make(chan struct{}),
	}
	// add configs
	for tag, config := range configs {
		err := ts.Add(tag, config)
		if err != nil {
			return nil, fmt.Errorf("add time sync config %s failed: %s", tag, err)
		}
	}
	// set time sync interval
	if interval < 1 {
		interval = defaultSyncInterval
	}
	err := ts.SetSyncInterval(interval)
	if err != nil {
		return nil, err
	}
	return ts, nil
}

// after config need test
func (ts *TimeSyncer) Test() error {
	if len(ts.Configs()) == 0 {
		return ErrNoConfigs
	}
	return ts.sync(false, true)
}

func (ts *TimeSyncer) Start() error {
	if len(ts.Configs()) == 0 {
		return ErrNoConfigs
	}
	// first time sync must success
	for {
		err := ts.sync(false, false)
		switch err {
		case nil:
			ts.wg.Add(2)
			go ts.addLoop()
			go ts.syncLoop()
			return nil
		case ErrAllFailed:
			ts.dnsClient.FlushCache()
			ts.log(logger.Warning, ErrAllFailed)
			random.Sleep(10, 20)
		default:
			return err
		}
	}
}

// stop once
func (ts *TimeSyncer) Stop() {
	close(ts.stopSignal)
	ts.wg.Wait()
}

func (ts *TimeSyncer) Now() time.Time {
	ts.nowRWM.RLock()
	t := ts.now
	ts.nowRWM.RUnlock()
	return t
}

func (ts *TimeSyncer) SetSyncInterval(interval time.Duration) error {
	if interval < time.Minute || interval > time.Hour*1 {
		return ErrInterval
	}
	ts.nowRWM.Lock()
	ts.interval = interval
	ts.nowRWM.Unlock()
	return nil
}

func (ts *TimeSyncer) GetSyncInterval() time.Duration {
	ts.nowRWM.RLock()
	i := ts.interval
	ts.nowRWM.RUnlock()
	return i
}

func (ts *TimeSyncer) Configs() map[string]*Config {
	configs := make(map[string]*Config)
	ts.configsRWM.RLock()
	for tag, config := range ts.configs {
		configs[tag] = config
	}
	ts.configsRWM.RUnlock()
	return configs
}

func (ts *TimeSyncer) Add(tag string, c *Config) error {
	switch c.Mode {
	case HTTP:
		c.HTTPOpts.Request.URL = c.Address
	case NTP:
	default:
		return fmt.Errorf("unknown mode: %s", c.Mode)
	}
	ts.configsRWM.Lock()
	defer ts.configsRWM.Unlock()
	if _, ok := ts.configs[tag]; !ok {
		ts.configs[tag] = c
		return nil
	} else {
		return errors.New("time sync config: " + tag + " already exists")
	}
}

func (ts *TimeSyncer) Delete(tag string) error {
	ts.configsRWM.Lock()
	defer ts.configsRWM.Unlock()
	if _, exist := ts.configs[tag]; exist {
		delete(ts.configs, tag)
		return nil
	} else {
		return errors.New("time sync config: " + tag + " doesn't exist")
	}
}

func (ts *TimeSyncer) log(l logger.Level, log ...interface{}) {
	ts.logger.Println(l, "timesyncer", log...)
}

// self walk
func (ts *TimeSyncer) addLoop() {
	defer ts.wg.Done()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ticker := time.NewTicker(addLoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ts.stopSignal:
			return
		case <-ticker.C:
			ts.nowRWM.Lock()
			ts.now = ts.now.Add(addLoopInterval)
			ts.nowRWM.Unlock()
		}
	}
}

func (ts *TimeSyncer) syncLoop() {
	defer ts.wg.Done()
	var interval time.Duration
	for {
		ts.nowRWM.RLock()
		interval = ts.interval
		ts.nowRWM.RUnlock()
		select {
		case <-ts.stopSignal:
			return
		case <-time.After(interval):
			err := ts.sync(true, false)
			if err != nil {
				ts.log(logger.Warning, "sync time failed:", err)
			}
		}
	}
}

// if accept_failed == true when sync time all failed
// set this.now = time.Now()
// sync_all is for test all clients
func (ts *TimeSyncer) sync(acceptFailed, syncAll bool) (err error) {
	var isOptsErr bool
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error("sync() panic:", r)
			ts.log(logger.Fatal, err)
		}
	}()
	// query
	for tag, config := range ts.Configs() {
		switch config.Mode {
		case HTTP:
			isOptsErr, err = ts.syncHTTP(config)
		case NTP:
			isOptsErr, err = ts.syncNTP(config)
		default:
			return fmt.Errorf("config %s use unknown mode", tag)
		}
		if isOptsErr {
			return fmt.Errorf("config %s has wrong options: %s", tag, err)
		}
		if err != nil {
			err = fmt.Errorf("config %s sync time failed: %s", tag, err)
			if syncAll {
				return err
			}
			ts.log(logger.Warning, err)
		} else {
			if syncAll {
				continue
			}
			return nil
		}
	}
	if syncAll {
		return nil
	}
	if acceptFailed {
		ts.nowRWM.Lock()
		ts.now = time.Now()
		ts.nowRWM.Unlock()
	}
	return ErrAllFailed
}

func (ts *TimeSyncer) syncHTTP(c *Config) (isOptsErr bool, err error) {
	// http request
	req, err := c.HTTPOpts.Request.Apply()
	if err != nil {
		isOptsErr = true
		return
	}
	hostname := req.URL.Hostname()
	// http transport
	tr, err := c.HTTPOpts.Transport.Apply()
	if err != nil {
		isOptsErr = true
		return
	}
	tr.TLSClientConfig.ServerName = hostname
	// set proxy
	p, err := ts.proxyPool.Get(c.ProxyTag)
	if err != nil {
		isOptsErr = true
		return
	}
	if p != nil {
		p.HTTP(tr)
	}
	// dns
	ipList, err := ts.dnsClient.Resolve(hostname, &c.DNSOpts)
	if err != nil {
		return
	}
	if req.URL.Scheme == "https" {
		if req.Host == "" {
			req.Host = req.URL.Host
		}
	}
	port := req.URL.Port()
	if port != "" {
		port = ":" + port
	}
	query := func() bool {
		now, err := queryHTTPServer(req, &http.Client{
			Transport: tr,
			Timeout:   c.Timeout,
		})
		if err != nil {
			return false
		}
		ts.nowRWM.Lock()
		ts.now = now
		ts.nowRWM.Unlock()
		return true
	}
	switch c.DNSOpts.Type {
	case "", dns.IPv4:
		for i := 0; i < len(ipList); i++ {
			req.URL.Host = ipList[i] + port
			if query() {
				return
			}
		}
	case dns.IPv6:
		for i := 0; i < len(ipList); i++ {
			req.URL.Host = "[" + ipList[i] + "]" + port
			if query() {
				return
			}
		}
	default:
		err = fmt.Errorf("timesyncer internal error: %s",
			dns.UnknownTypeError(c.DNSOpts.Type))
		panic(err)
	}
	return false, ErrQueryHTTP
}

// return opt_err
func (ts *TimeSyncer) syncNTP(c *Config) (isOptsErr bool, err error) {
	host, port, err := net.SplitHostPort(c.Address)
	if err != nil {
		isOptsErr = true
		return
	}
	ntpOpts := ntpOptions{
		Version: c.NTPOpts.Version,
		Network: c.NTPOpts.Network,
		Timeout: c.Timeout,
	}
	// set proxy
	p, err := ts.proxyPool.Get(c.ProxyTag)
	if err != nil {
		isOptsErr = true
		return
	}
	if p != nil {
		ntpOpts.Dial = p.Dial
	}
	// dns
	ipList, err := ts.dnsClient.Resolve(host, &c.DNSOpts)
	if err != nil {
		isOptsErr = true
		err = fmt.Errorf("resolve domain name failed: %s", err)
		return
	}
	switch c.DNSOpts.Type {
	case "", dns.IPv4:
		for i := 0; i < len(ipList); i++ {
			resp, err := queryNTPServer(ipList[i]+":"+port, &ntpOpts)
			if err != nil {
				continue
			}
			ts.nowRWM.Lock()
			ts.now = resp.Time
			ts.nowRWM.Unlock()
			return false, nil
		}
	case dns.IPv6:
		for i := 0; i < len(ipList); i++ {
			resp, err := queryNTPServer("["+ipList[i]+"]:"+port, &ntpOpts)
			if err != nil {
				continue
			}
			ts.nowRWM.Lock()
			ts.now = resp.Time
			ts.nowRWM.Unlock()
			return false, nil
		}
	default:
		err = fmt.Errorf("timesyncer internal error: %s",
			dns.UnknownTypeError(c.DNSOpts.Type))
		panic(err)
	}
	return false, ErrQueryNTP
}
