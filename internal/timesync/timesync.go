package timesync

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xpanic"
)

type Mode = string

const (
	HTTP Mode = "http" // get response header: Date
	NTP  Mode = "ntp"
)

const (
	addLoopInterval = 500 * time.Millisecond
)

var (
	ErrNoClient         = errors.New("no time syncer client")
	ErrAllClientsFailed = errors.New("all time sync clients query failed")
	ErrInvalidInterval  = errors.New("interval < 60s or > 1h")
)

type Client struct {
	Mode   Mode
	Config []byte
	client
}

type client interface {
	Query() (now time.Time, isOptsErr bool, err error)
	ImportConfig(b []byte) error
	ExportConfig() []byte
}

type TimeSyncer struct {
	proxyPool *proxy.Pool
	dnsClient *dns.Client
	logger    logger.Logger

	clients    map[string]*Client // key = tag
	clientsRWM sync.RWMutex
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
	clients map[string]*Client,
	interval time.Duration,
) (*TimeSyncer, error) {
	ts := TimeSyncer{
		proxyPool:  pool,
		dnsClient:  client,
		logger:     logger,
		now:        time.Now(),
		clients:    make(map[string]*Client),
		stopSignal: make(chan struct{}),
	}
	// add clients
	for tag, client := range clients {
		err := ts.Add(tag, client)
		if err != nil {
			return nil, err
		}
	}
	err := ts.SetSyncInterval(interval)
	if err != nil {
		return nil, err
	}
	return &ts, nil
}

func (ts *TimeSyncer) Add(tag string, client *Client) error {
	switch client.Mode {
	case HTTP:
		client.client = &HTTPClient{
			proxyPool: ts.proxyPool,
			dnsClient: ts.dnsClient,
		}
	case NTP:
		client.client = &NTPClient{
			proxyPool: ts.proxyPool,
			dnsClient: ts.dnsClient,
		}
	default:
		return fmt.Errorf("unknown mode: %s", client.Mode)
	}
	err := client.client.ImportConfig(client.Config)
	if err != nil {
		return err
	}
	security.FlushBytes(client.Config)
	ts.clientsRWM.Lock()
	defer ts.clientsRWM.Unlock()
	if _, ok := ts.clients[tag]; !ok {
		ts.clients[tag] = client
		return nil
	} else {
		return errors.New("time syncer client: " + tag + " already exists")
	}
}

func (ts *TimeSyncer) Delete(tag string) error {
	ts.clientsRWM.Lock()
	defer ts.clientsRWM.Unlock()
	if _, exist := ts.clients[tag]; exist {
		delete(ts.clients, tag)
		return nil
	} else {
		return errors.New("time syncer client: " + tag + " doesn't exist")
	}
}

func (ts *TimeSyncer) Clients() map[string]*Client {
	clients := make(map[string]*Client)
	ts.clientsRWM.RLock()
	for tag, client := range ts.clients {
		clients[tag] = client
	}
	ts.clientsRWM.RUnlock()
	return clients
}

func (ts *TimeSyncer) Start() error {
	if len(ts.Clients()) == 0 {
		return ErrNoClient
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
		case ErrAllClientsFailed:
			ts.dnsClient.FlushCache()
			ts.log(logger.Warning, ErrAllClientsFailed)
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

func (ts *TimeSyncer) GetSyncInterval() time.Duration {
	ts.nowRWM.RLock()
	i := ts.interval
	ts.nowRWM.RUnlock()
	return i
}

func (ts *TimeSyncer) SetSyncInterval(interval time.Duration) error {
	if interval < time.Minute || interval > time.Hour*1 {
		return ErrInvalidInterval
	}
	ts.nowRWM.Lock()
	ts.interval = interval
	ts.nowRWM.Unlock()
	return nil
}

// Test is used to test all client
func (ts *TimeSyncer) Test() error {
	if len(ts.Clients()) == 0 {
		return ErrNoClient
	}
	return ts.sync(false, true)
}

func (ts *TimeSyncer) log(lv logger.Level, log ...interface{}) {
	ts.logger.Println(lv, "timesyncer", log...)
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
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error("TimeSyncer.sync() panic:", r)
			ts.log(logger.Fatal, err)
		}
	}()
	var (
		now       time.Time
		isOptsErr bool
	)
	for tag, client := range ts.Clients() {
		now, isOptsErr, err = client.Query()
		if isOptsErr {
			return fmt.Errorf("client %s has invalid config: %s", tag, err)
		}
		if err != nil {
			err = fmt.Errorf("client %s sync time failed: %s", tag, err)
			if syncAll {
				return err
			}
			ts.log(logger.Warning, err)
		} else {
			ts.nowRWM.Lock()
			ts.now = now
			ts.nowRWM.Unlock()
			if syncAll {
				continue
			}
			return
		}
	}
	if syncAll {
		return
	}
	if acceptFailed {
		ts.nowRWM.Lock()
		ts.now = time.Now()
		ts.nowRWM.Unlock()
	}
	return ErrAllClientsFailed
}
