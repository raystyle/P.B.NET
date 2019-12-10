package timesync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/options"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/xpanic"
)

const (
	ModeHTTP = "http"
	ModeNTP  = "ntp"
)

var (
	ErrNoClient         = fmt.Errorf("no time syncer clients")
	ErrAllClientsFailed = fmt.Errorf("all time syncer clients failed to query")
)

// Client include mode and config
type Client struct {
	Mode     string `toml:"mode"`
	Config   string `toml:"config"`
	SkipTest bool   `toml:"skip_test"`
	client
}

type client interface {
	Query() (now time.Time, optsErr bool, err error)
	Import(b []byte) error
	Export() []byte
}

// Syncer is used to synchronize time
type Syncer struct {
	proxyPool *proxy.Pool
	dnsClient *dns.Client
	logger    logger.Logger

	clients     map[string]*Client // key = tag
	clientsRWM  sync.RWMutex
	fixedSleep  int           // about Start
	randomSleep int           // about Start
	interval    time.Duration // sync interval
	now         time.Time
	nowRWM      sync.RWMutex // now

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New is used to create time syncer
func New(pool *proxy.Pool, client *dns.Client, logger logger.Logger) *Syncer {
	syncer := Syncer{
		proxyPool:   pool,
		dnsClient:   client,
		logger:      logger,
		clients:     make(map[string]*Client),
		fixedSleep:  options.DefaultTimeSyncFixed,
		randomSleep: options.DefaultTimeSyncRandom,
		interval:    options.DefaultTimeSyncInterval,
		now:         time.Now(),
	}
	syncer.ctx, syncer.cancel = context.WithCancel(context.Background())
	return &syncer
}

// Add is used to add time syncer client
func (syncer *Syncer) Add(tag string, client *Client) error {
	switch client.Mode {
	case ModeHTTP:
		client.client = NewHTTP(syncer.ctx, syncer.proxyPool, syncer.dnsClient)
	case ModeNTP:
		client.client = NewNTP(syncer.ctx, syncer.proxyPool, syncer.dnsClient)
	default:
		return errors.Errorf("unknown mode: %s", client.Mode)
	}
	err := client.client.Import([]byte(client.Config))
	if err != nil {
		return err
	}
	syncer.clientsRWM.Lock()
	defer syncer.clientsRWM.Unlock()
	if _, ok := syncer.clients[tag]; !ok {
		syncer.clients[tag] = client
		return nil
	} else {
		return fmt.Errorf("time syncer client: %s already exists", tag)
	}
}

// Delete is used to delete syncer client
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

// Clients is used to get all time syncer clients
func (syncer *Syncer) Clients() map[string]*Client {
	syncer.clientsRWM.RLock()
	defer syncer.clientsRWM.RUnlock()
	clients := make(map[string]*Client, len(syncer.clients))
	for tag, client := range syncer.clients {
		clients[tag] = client
	}
	return clients
}

// Now is used to get current time
func (syncer *Syncer) Now() time.Time {
	syncer.nowRWM.RLock()
	defer syncer.nowRWM.RUnlock()
	return syncer.now
}

// GetSyncInterval is used to get synchronize time interval
func (syncer *Syncer) GetSyncInterval() time.Duration {
	syncer.clientsRWM.RLock()
	defer syncer.clientsRWM.RUnlock()
	return syncer.interval
}

// SetSyncInterval is used to set synchronize time interval
func (syncer *Syncer) SetSyncInterval(interval time.Duration) error {
	if interval < time.Minute || interval > 15*time.Minute {
		return errors.New("synchronize interval < 1 minute or > 15 minutes")
	}
	syncer.clientsRWM.Lock()
	defer syncer.clientsRWM.Unlock()
	syncer.interval = interval
	return nil
}

// SetSleep is used to set random sleep time
// must execute before Start()
func (syncer *Syncer) SetSleep(fixed, random int) {
	if fixed < 1 {
		fixed = options.DefaultTimeSyncFixed
	}
	if random < 1 {
		random = options.DefaultTimeSyncRandom
	}
	syncer.fixedSleep = fixed
	syncer.randomSleep = random
}

func (syncer *Syncer) log(lv logger.Level, log ...interface{}) {
	syncer.logger.Println(lv, "time syncer", log...)
}

// Start is used to time syncer
func (syncer *Syncer) Start() error {
	if len(syncer.Clients()) == 0 {
		return ErrNoClient
	}
	// first time sync must success
	for {
		err := syncer.synchronize(false)
		switch err {
		case nil:
			syncer.wg.Add(2)
			go syncer.walker()
			go syncer.synchronizeLoop()
			return nil
		case ErrAllClientsFailed:
			syncer.dnsClient.FlushCache()
			syncer.log(logger.Warning, ErrAllClientsFailed)
			random.Sleep(syncer.fixedSleep, syncer.fixedSleep)
		default:
			return err
		}
	}
}

// StartWalker is used to start walker if role skip synchronize time
func (syncer *Syncer) StartWalker() {
	syncer.nowRWM.Lock()
	defer syncer.nowRWM.Unlock()
	syncer.now = time.Now()
	syncer.wg.Add(1)
	go syncer.walker()
}

// Stop is used to stop time syncer
func (syncer *Syncer) Stop() {
	syncer.cancel()
	syncer.wg.Wait()
}

// Test is used to test all client
func (syncer *Syncer) Test() error {
	if len(syncer.Clients()) == 0 {
		return ErrNoClient
	}
	return syncer.synchronize(true)
}

func (syncer *Syncer) walker() {
	defer func() {
		if r := recover(); r != nil {
			syncer.log(logger.Fatal, xpanic.Print(r, "Syncer.walker"))
			// restart walker
			time.Sleep(time.Second)
			go syncer.walker()
		} else {
			syncer.wg.Done()
		}
	}()
	// if addLoopInterval < 2 Millisecond, time will be inaccurate
	// see GOROOT/src/time/tick.go NewTicker()
	//
	// "It adjusts the intervals or drops ticks
	// to make up for slow receivers."
	const addLoopInterval = 10 * time.Millisecond
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

func (syncer *Syncer) synchronizeLoop() {
	defer func() {
		if r := recover(); r != nil {
			syncer.log(logger.Fatal, xpanic.Print(r, "Syncer.synchronizeLoop"))
			// restart synchronizeLoop
			time.Sleep(time.Second)
			go syncer.synchronizeLoop()
		} else {
			syncer.wg.Done()
		}
	}()
	timer := time.NewTimer(syncer.GetSyncInterval())
	defer timer.Stop()
	for {
		timer.Reset(syncer.GetSyncInterval())
		select {
		case <-timer.C:
			err := syncer.synchronize(false)
			if err != nil {
				syncer.log(logger.Warning, "failed to synchronize time:", err)
			}
		case <-syncer.ctx.Done():
			return
		}
	}
}

// all is for test all clients
func (syncer *Syncer) synchronize(all bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "Syncer.synchronize")
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
		// check skip test
		if all {
			if client.SkipTest {
				continue
			}
		}
		now, optsErr, err = client.Query()
		if err != nil {
			if optsErr {
				return fmt.Errorf("client %s with invalid configuartion: %s", tag, err)
			}
			err = fmt.Errorf("client %s failed to synchronize time: %s", tag, err)
			if all {
				return err
			}
			syncer.log(logger.Warning, err)
		} else {
			update()
			if all {
				// test all syncer clients
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
