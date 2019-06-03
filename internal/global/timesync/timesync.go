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
)

type Mode uint8

const (
	HTTP Mode = iota // get response header: Date
	NTP
)

const (
	default_interval = 15 * time.Minute
	add_interval     = time.Millisecond * 500
	// 0 = add
	// 1 = stop time sync loop
	stop_signal = 2
)

var (
	ERR_INVALID_INTERVAL = errors.New("invalid time sync interval < 60s or > 1h")
)

type Client struct {
	Mode     Mode
	DNS_Opts dnsclient.Options
	Proxy    string // for NTP_Opts.Dial or H_Transport
	Address  string // if Mode == HTTP useless
	NTP_Opts *ntp.Options
	// for httptime
	H_Request   options.HTTP_Request
	H_Transport *options.HTTP_Transport
	H_Timeout   time.Duration
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
			return nil, err
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

func (this *TIMESYNC) Start() {
	// first time sync must success
	for err := this.sync_time(false); err != nil; {
		this.log(logger.WARNING, "first sync time failed:", err)
		this.dns.Flush_Cache()
		time.Sleep(5 * time.Second)
	}
	for i := 0; i < stop_signal; i++ {
		this.stop_signal[i] = make(chan struct{}, 1)
	}
	this.wg.Add(2)
	go this.add()
	go this.sync_time_loop()
}

func (this *TIMESYNC) Now() time.Time {
	this.rwm.RLock()
	t := this.now
	this.rwm.RUnlock()
	return t
}

func (this *TIMESYNC) Set_Interval(interval time.Duration) error {
	if interval < time.Minute || interval > time.Hour*1 {
		return ERR_INVALID_INTERVAL
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

func (this *TIMESYNC) Add(tag string, client *Client) error {
	defer this.clients_rwm.Unlock()
	this.clients_rwm.Lock()
	if _, exist := this.clients[tag]; !exist {
		this.clients[tag] = client
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

func (this *TIMESYNC) Destroy() {
	for i := 0; i < stop_signal; i++ {
		this.stop_signal[i] <- struct{}{}
		close(this.stop_signal[i])
	}
	this.wg.Wait()
	this.clients_rwm.Lock()
	this.clients = nil
	this.clients_rwm.Unlock()
}

func (this *TIMESYNC) logf(l logger.Level, format string, log ...interface{}) {
	this.logger.Printf(l, "timesync", format, log...)
}

func (this *TIMESYNC) log(l logger.Level, log ...interface{}) {
	this.logger.Println(l, "timesync", log...)
}

// self walk
func (this *TIMESYNC) add() {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				this.log(logger.ERROR, "add() panic:", v)
			default:
				this.log(logger.ERROR, "add() panic: unknown panic")
			}
			go this.add()
		}
	}()
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

func (this *TIMESYNC) sync_time_loop() {
	defer func() {
		if r := recover(); r != nil {
			switch v := r.(type) {
			case error:
				this.log(logger.ERROR, "sync_time_loop() panic:", v)
			default:
				this.log(logger.ERROR, "sync_time_loop() panic: unknown panic")
			}
			go this.sync_time_loop()
		}
	}()
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
			err := this.sync_time(true)
			if err != nil {
				this.log(logger.WARNING, "sync time failed:", err)
			}
		}
	}
}

// if failed == true when sync time all failed
// set this.now = time.Now()
func (this *TIMESYNC) sync_time(failed bool) error {
	// copy map
	clients := make(map[string]*Client)
	this.clients_rwm.RLock()
	for tag, client := range this.clients {
		clients[tag] = client
	}
	this.clients_rwm.RUnlock()
	// query
	for tag, client := range clients {
		// get proxy
		p, err := this.proxy.Get(client.Proxy)
		if err != nil {
			return fmt.Errorf("client %s proxy: %s", tag, err)
		}
		switch client.Mode {
		case HTTP:
			r, err := client.H_Request.Apply()
			if err != nil {
				return fmt.Errorf("client %s http Request apply failed: %s", tag, err)
			}
			hc := &http.Client{
				Timeout: options.DEFAULT_DIAL_TIMEOUT,
			}
			if client.H_Timeout > 0 {
				hc.Timeout = client.H_Timeout
			}
			var tr *http.Transport
			if client.H_Transport != nil {
				tr, err = client.H_Transport.Apply()
				if err != nil {
					return fmt.Errorf("client %s http Transport apply failed: %s", tag, err)
				}
			} else {
				tr, _ = new(options.HTTP_Transport).Apply()
			}
			// don't set dns resolve
			// set proxy
			if p != nil {
				p.HTTP(tr)
			}
			hc.Transport = tr
			t, err := httptime.Query(r, hc)
			if err != nil {
				this.logf(logger.WARNING, "client %s query http time failed: %s", tag, err)
				continue
			}
			this.rwm.Lock()
			this.now = t
			this.rwm.Unlock()
			return nil
		case NTP:
			host, port, err := net.SplitHostPort(client.Address)
			if err != nil {
				return fmt.Errorf("client %s address: %s", tag, err)
			}
			// resolve dns
			ip_list, err := this.dns.Resolve(host, &client.DNS_Opts)
			if err != nil {
				this.logf(logger.WARNING, "client %s resolve dns failed: %s", tag, err)
				continue
			}
			// set proxy
			if p != nil {
				client.NTP_Opts.Dial = p.Dial
			}
			for i := 0; i < len(ip_list); i++ {
				var response *ntp.Response
				switch client.DNS_Opts.Opts.Type {
				case 0, dns.IPV4:
					response, err = ntp.Query(ip_list[i]+":"+port, client.NTP_Opts)
				case dns.IPV6:
					response, err = ntp.Query("["+ip_list[i]+"]:"+port, client.NTP_Opts)
				}
				if err != nil {
					this.logf(logger.WARNING, "client %s query ntp failed: %s", tag, err)
					continue
				}
				this.rwm.Lock()
				this.now = response.Time
				this.rwm.Unlock()
				return nil
			}
		default:
			return fmt.Errorf("client %s invalid client mode", tag)
		}
	}
	if failed {
		this.rwm.Lock()
		this.now = time.Now()
		this.rwm.Unlock()
		return nil
	}
	return errors.New("sync time failed")
}
