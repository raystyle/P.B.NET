package timesync

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"project/internal/dns"
	"project/internal/global/dnsclient"
	"project/internal/global/proxyclient"
	"project/internal/httptime"
	"project/internal/logger"
	"project/internal/ntp"
	"project/internal/options"
	"project/internal/random"
)

type Mode string

const (
	HTTP Mode = "http" // get response header: Date
	NTP  Mode = "ntp"
)

const (
	default_interval = 15 * time.Minute
	add_interval     = time.Millisecond * 500
	// 0 = add
	// 1 = stop time sync loop
	stop_signal = 2
)

var (
	ERR_NO_CLIENTS   = errors.New("no timesync client")
	ERR_UNKNOWN_MODE = errors.New("unknown client mode")
	ERR_ALL_FAILED   = errors.New("time sync all failed")
	ERR_INTERVAL     = errors.New("interval < 60s or > 1h")
	ERR_QUERY_NTP    = errors.New("query ntp server failed")
)

type Client struct {
	Mode    Mode
	Address string // if Mode == HTTP cover H_Request.URL
	// options
	Timeout time.Duration
	Proxy   string
	// for ntp.Option
	NTP_Opts struct {
		Version  int    // NTP protocol version, defaults to 4
		Network  string // network to use, defaults to udp
		DNS_Opts dnsclient.Options
	} `toml:"ntp_options"`
	// for httptime
	HTTP_Opts struct {
		Request   options.HTTP_Request
		Transport options.HTTP_Transport
	} `toml:"http_options"`
}

type TIMESYNC struct {
	proxy       *proxyclient.PROXY // ctx
	dns         *dnsclient.DNS     // ctx
	logger      logger.Logger      // ctx
	interval    time.Duration
	clients     map[string]*Client // key = tag
	clients_rwm sync.RWMutex
	now         time.Time
	rwm         sync.RWMutex
	stop_signal [stop_signal]chan struct{}
	wg          sync.WaitGroup
}

func New(p *proxyclient.PROXY, d *dnsclient.DNS, l logger.Logger,
	c map[string]*Client, interval time.Duration) (*TIMESYNC, error) {
	t := &TIMESYNC{
		proxy:    p,
		dns:      d,
		logger:   l,
		interval: interval,
		clients:  make(map[string]*Client),
	}
	// add clients
	for tag, client := range c {
		err := t.Add(tag, client)
		if err != nil {
			return nil, fmt.Errorf("add timesync client %s failed: %s", tag, err)
		}
	}
	// set time sync interval
	if interval <= 0 {
		interval = default_interval
	}
	err := t.Set_Interval(interval)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// after config need test
func (this *TIMESYNC) Test() error {
	if len(this.Clients()) == 0 {
		return ERR_NO_CLIENTS
	}
	return this.sync(false, true)
}

func (this *TIMESYNC) Start() error {
	if len(this.Clients()) == 0 {
		return ERR_NO_CLIENTS
	}
	// first time sync must success
	for {
		err := this.sync(false, false)
		switch err {
		case nil:
			goto S
		case ERR_ALL_FAILED:
			this.dns.Flush_Cache()
			this.log(logger.WARNING, ERR_ALL_FAILED)
			random.Sleep(10, 20)
		default:
			return err
		}
	}
S:
	for i := 0; i < stop_signal; i++ {
		this.stop_signal[i] = make(chan struct{}, 1)
	}
	this.wg.Add(2)
	go this.add()
	go this.sync_loop()
	return nil
}

func (this *TIMESYNC) Stop() {
	for i := 0; i < stop_signal; i++ {
		this.stop_signal[i] <- struct{}{}
		close(this.stop_signal[i])
	}
	this.wg.Wait()
}

func (this *TIMESYNC) Now() time.Time {
	this.rwm.RLock()
	t := this.now
	this.rwm.RUnlock()
	return t
}

func (this *TIMESYNC) Set_Interval(interval time.Duration) error {
	if interval < time.Minute || interval > time.Hour*1 {
		return ERR_INTERVAL
	}
	this.rwm.Lock()
	this.interval = interval
	this.rwm.Unlock()
	return nil
}

func (this *TIMESYNC) Clients() map[string]*Client {
	client_pool := make(map[string]*Client)
	defer this.clients_rwm.RUnlock()
	this.clients_rwm.RLock()
	for tag, client := range this.clients {
		client_pool[tag] = client
	}
	return client_pool
}

func (this *TIMESYNC) Add(tag string, c *Client) error {
	switch c.Mode {
	case HTTP:
		c.HTTP_Opts.Request.URL = c.Address
	case NTP:
	default:
		return ERR_UNKNOWN_MODE
	}
	defer this.clients_rwm.Unlock()
	this.clients_rwm.Lock()
	if _, exist := this.clients[tag]; !exist {
		this.clients[tag] = c
		return nil
	} else {
		return errors.New("time sync client: " + tag + " already exists")
	}
}

func (this *TIMESYNC) Delete(tag string) error {
	defer this.clients_rwm.Unlock()
	this.clients_rwm.Lock()
	if _, exist := this.clients[tag]; exist {
		delete(this.clients, tag)
		return nil
	} else {
		return errors.New("time sync client: " + tag + " doesn't exist")
	}
}

func (this *TIMESYNC) log(l logger.Level, log ...interface{}) {
	this.logger.Println(l, "timesync", log...)
}

// self walk
func (this *TIMESYNC) add() {
	ticker := time.NewTicker(add_interval)
	for {
		select {
		case <-this.stop_signal[0]:
			ticker.Stop()
			this.wg.Done()
			return
		case <-ticker.C:
			this.rwm.Lock()
			this.now = this.now.Add(add_interval)
			this.rwm.Unlock()
		}
	}
}

func (this *TIMESYNC) sync_loop() {
	var interval time.Duration
	for {
		this.rwm.RLock()
		interval = this.interval
		this.rwm.RUnlock()
		select {
		case <-this.stop_signal[1]:
			this.wg.Done()
			return
		case <-time.After(interval):
			err := this.sync(true, false)
			if err != nil {
				this.log(logger.WARNING, "sync time failed:", err)
			}
		}
	}
}

// if accept_failed == true when sync time all failed
// set this.now = time.Now()
// sync_all is for test all clients
func (this *TIMESYNC) sync(accept_failed, sync_all bool) error {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				this.log(logger.FATAL, "sync() panic:", v)
			default:
				this.log(logger.FATAL, "sync() panic: unknown panic")
			}
		}
	}()
	// copy map
	clients := make(map[string]*Client)
	this.clients_rwm.RLock()
	for tag, client := range this.clients {
		clients[tag] = client
	}
	this.clients_rwm.RUnlock()
	// query
	for tag, client := range clients {
		var (
			opts_err bool
			err      error
		)
		switch client.Mode {
		case HTTP:
			opts_err, err = this.sync_httptime(client)
		case NTP:
			opts_err, err = this.sync_ntp(client)
		default:
			return fmt.Errorf("client %s use unknown mode", tag)
		}
		if opts_err {
			return fmt.Errorf("client %s has wrong options: %s", tag, err)
		}
		if err != nil {
			err = fmt.Errorf("client %s sync failed: %s", tag, err)
			if sync_all {
				return err
			}
			this.log(logger.WARNING, err)
		} else {
			if sync_all {
				continue
			}
			return nil
		}
	}
	if sync_all {
		return nil
	}
	if accept_failed {
		this.rwm.Lock()
		this.now = time.Now()
		this.rwm.Unlock()
	}
	return ERR_ALL_FAILED
}

func (this *TIMESYNC) sync_httptime(c *Client) (opt_err bool, err error) {
	// http request
	req, err := c.HTTP_Opts.Request.Apply()
	if err != nil {
		opt_err = true
		return
	}
	// http transport
	tr, err := c.HTTP_Opts.Transport.Apply()
	if err != nil {
		opt_err = true
		return
	}
	// set proxy
	proxy, err := this.proxy.Get(c.Proxy)
	if err != nil {
		opt_err = true
		return
	}
	if proxy != nil {
		proxy.HTTP(tr)
	}
	// don't set dns resolve
	client := &http.Client{
		Transport: tr,
		Timeout:   c.Timeout,
	}
	now, err := httptime.Query(req, client)
	if err != nil {
		err = fmt.Errorf("query http time failed: %s", err)
		return
	}
	this.rwm.Lock()
	this.now = now
	this.rwm.Unlock()
	return false, nil
}

// return opt_err
func (this *TIMESYNC) sync_ntp(c *Client) (opt_err bool, err error) {
	host, port, err := net.SplitHostPort(c.Address)
	if err != nil {
		opt_err = true
		return
	}
	ntp_opts := &ntp.Options{
		Version: c.NTP_Opts.Version,
		Network: c.NTP_Opts.Network,
		Timeout: c.Timeout,
	}
	// set proxy
	proxy, err := this.proxy.Get(c.Proxy)
	if err != nil {
		opt_err = true
		return
	}
	if proxy != nil {
		ntp_opts.Dial = proxy.Dial
	}
	// resolve dns
	ip_list, err := this.dns.Resolve(host, &c.NTP_Opts.DNS_Opts)
	if err != nil {
		opt_err = true
		err = fmt.Errorf("resolve dns failed: %s", err)
		return
	}
	switch c.NTP_Opts.DNS_Opts.Type {
	case "", dns.IPV4:
		for i := 0; i < len(ip_list); i++ {
			resp, err := ntp.Query(ip_list[i]+":"+port, ntp_opts)
			if err != nil {
				continue
			}
			this.rwm.Lock()
			this.now = resp.Time
			this.rwm.Unlock()
			return false, nil
		}
	case dns.IPV6:
		for i := 0; i < len(ip_list); i++ {
			resp, err := ntp.Query("["+ip_list[i]+"]:"+port, ntp_opts)
			if err != nil {
				continue
			}
			this.rwm.Lock()
			this.now = resp.Time
			this.rwm.Unlock()
			return false, nil
		}
	default:
		err = fmt.Errorf("timesync internal error: %s", dns.ERR_INVALID_TYPE)
		panic(err)
	}
	return false, ERR_QUERY_NTP
}
