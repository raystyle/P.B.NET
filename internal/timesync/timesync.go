package timesync

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xpanic"
)

const (
	ModeHTTP = "http"
	ModeNTP  = "ntp"
)

var (
	ErrNoClient         = fmt.Errorf("no time syncer client")
	ErrAllClientsFailed = fmt.Errorf("all time syncer clients query failed")
	ErrInvalidInterval  = fmt.Errorf("interval < 60 second or > 1 hour")
)

type Client struct {
	Mode   string
	Config []byte
	client
}

type client interface {
	Query() (now time.Time, optsErr bool, err error)
	Import(b []byte) error
	Export() []byte
}

type Syncer struct {
	proxyPool *proxy.Pool
	dnsClient *dns.Client
	logger    logger.Logger

	clients    map[string]*Client // key = tag
	clientsRWM sync.RWMutex
	interval   time.Duration // sync interval
	now        time.Time
	nowRWM     sync.RWMutex // now

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func New(pool *proxy.Pool, client *dns.Client, logger logger.Logger) *Syncer {
	syncer := Syncer{
		proxyPool: pool,
		dnsClient: client,
		logger:    logger,
		now:       time.Now(),
		clients:   make(map[string]*Client),
	}
	syncer.ctx, syncer.cancel = context.WithCancel(context.Background())
	return &syncer
}

func (syncer *Syncer) Add(tag string, client *Client) error {
	switch client.Mode {
	case ModeHTTP:
		client.client = NewHTTP(syncer.ctx, syncer.proxyPool, syncer.dnsClient)
	case ModeNTP:
		client.client = NewNTP(syncer.proxyPool, syncer.dnsClient)
	default:
		return errors.Errorf("unknown mode: %s", client.Mode)
	}
	err := client.client.Import(client.Config)
	if err != nil {
		return err
	}
	security.FlushBytes(client.Config)
	syncer.clientsRWM.Lock()
	defer syncer.clientsRWM.Unlock()
	if _, ok := syncer.clients[tag]; !ok {
		syncer.clients[tag] = client
		return nil
	} else {
		return fmt.Errorf("time syncer client: %s already exists", tag)
	}
}

func (syncer *Syncer) Delete(tag string) error {
	syncer.clientsRWM.Lock()
	defer syncer.clientsRWM.Unlock()
	if _, exist := syncer.clients[tag]; exist {
		delete(syncer.clients, tag)
		return nil
	} else {
		return fmt.Errorf("time syncer client: %s doesn't exist", tag)
	}
}

func (syncer *Syncer) Clients() map[string]*Client {
	clients := make(map[string]*Client)
	syncer.clientsRWM.RLock()
	defer syncer.clientsRWM.RUnlock()
	for tag, client := range syncer.clients {
		clients[tag] = client
	}
	return clients
}

func (syncer *Syncer) Now() time.Time {
	syncer.nowRWM.RLock()
	defer syncer.nowRWM.RUnlock()
	return syncer.now
}

func (syncer *Syncer) GetSyncInterval() time.Duration {
	syncer.clientsRWM.RLock()
	defer syncer.clientsRWM.RUnlock()
	return syncer.interval
}

func (syncer *Syncer) SetSyncInterval(interval time.Duration) error {
	if interval < time.Minute || interval > time.Hour*1 {
		return ErrInvalidInterval
	}
	syncer.clientsRWM.Lock()
	defer syncer.clientsRWM.Unlock()
	syncer.interval = interval
	return nil
}

func (syncer *Syncer) log(lv logger.Level, log ...interface{}) {
	syncer.logger.Println(lv, "timesyncer", log...)
}

func (syncer *Syncer) Start() error {
	if len(syncer.Clients()) == 0 {
		return ErrNoClient
	}
	// first time sync must success
	for {
		err := syncer.sync(false)
		switch err {
		case nil:
			syncer.wg.Add(2)
			go syncer.addLoop()
			go syncer.syncLoop()
			return nil
		case ErrAllClientsFailed:
			syncer.dnsClient.FlushCache()
			syncer.log(logger.Warning, ErrAllClientsFailed)
			random.Sleep(10, 20)
		default:
			return err
		}
	}
}

// stop once
func (syncer *Syncer) Stop() {
	syncer.cancel()
	syncer.wg.Wait()
}

// Test is used to test all client
func (syncer *Syncer) Test() error {
	if len(syncer.Clients()) == 0 {
		return ErrNoClient
	}
	return syncer.sync(true)
}

// self walk
func (syncer *Syncer) addLoop() {
	const addLoopInterval = 500 * time.Millisecond
	defer syncer.wg.Done()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ticker := time.NewTicker(addLoopInterval)
	defer ticker.Stop()
	add := func() {
		syncer.nowRWM.Lock()
		defer syncer.nowRWM.Unlock()
		syncer.now = syncer.now.Add(addLoopInterval)
	}
	for {
		select {
		case <-ticker.C:
			add()
		case <-syncer.ctx.Done():
			return
		}
	}
}

func (syncer *Syncer) syncLoop() {
	defer syncer.wg.Done()
	for {
		select {
		case <-time.After(syncer.GetSyncInterval()):
			err := syncer.sync(false)
			if err != nil {
				syncer.log(logger.Fatal, "failed to sync time:", err)
			}
		case <-syncer.ctx.Done():
			return
		}
	}
}

// all is for test all clients
func (syncer *Syncer) sync(all bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "Syncer.sync() panic:")
			syncer.log(logger.Fatal, err)
		}
	}()
	var (
		now     time.Time
		optsErr bool
	)
	update := func() {
		syncer.nowRWM.Lock()
		defer syncer.nowRWM.Unlock()
		syncer.now = now
	}
	for tag, client := range syncer.Clients() {
		now, optsErr, err = client.Query()
		if err != nil {
			if optsErr {
				return fmt.Errorf("client %s with invalid config: %s", tag, err)
			}
			err = fmt.Errorf("client %s failed to sync time: %s", tag, err)
			if all {
				return err
			}
			syncer.log(logger.Warning, err)
		} else {
			update()
			if all {
				continue
			}
			return
		}
	}
	if all {
		return
	}
	return ErrAllClientsFailed
}
