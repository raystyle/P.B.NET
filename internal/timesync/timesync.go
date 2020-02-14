package timesync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/dns"
	"project/internal/logger"
	"project/internal/proxy"
	"project/internal/random"
	"project/internal/xpanic"
)

// supported modes
const (
	ModeHTTP = "http"
	ModeNTP  = "ntp"
)

const (
	defaultSleepFixed   = 10
	defaultSleepRandom  = 20
	defaultSyncInterval = 3 * time.Minute
)

// errors
var (
	ErrNoClients        = fmt.Errorf("no time syncer clients")
	ErrAllClientsFailed = fmt.Errorf("all time syncer clients failed to query")
)

// Client contains mode and config
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

	// about random.Sleep() in Start()
	sleepFixed  uint
	sleepRandom uint
	// synchronize interval
	interval time.Duration

	clients    map[string]*Client // key = tag
	clientsRWM sync.RWMutex
	now        time.Time
	nowRWM     sync.RWMutex

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
		sleepFixed:  defaultSleepFixed,
		sleepRandom: defaultSleepRandom,
		interval:    defaultSyncInterval,
		now:         time.Now(),
	}
	syncer.ctx, syncer.cancel = context.WithCancel(context.Background())
	return &syncer
}

// SetSleep is used to set random sleep time
// must execute before Start()
func (syncer *Syncer) SetSleep(fixed, random uint) error {
	if fixed < 3 {
		return errors.New("sleep fixed must >= 3")
	}
	if random < 5 {
		return errors.New("sleep random must >= 5")
	}
	syncer.sleepFixed = fixed
	syncer.sleepRandom = random
	return nil
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
	}
	return fmt.Errorf("time syncer client: %s already exists", tag)
}

// Delete is used to delete syncer client
func (syncer *Syncer) Delete(tag string) error {
	syncer.clientsRWM.Lock()
	defer syncer.clientsRWM.Unlock()
	if _, exist := syncer.clients[tag]; exist {
		delete(syncer.clients, tag)
		return nil
	}
	return fmt.Errorf("time syncer client: %s doesn't exist", tag)
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
		return errors.New("synchronize interval must < 1 minute or > 15 minutes")
	}
	syncer.clientsRWM.Lock()
	defer syncer.clientsRWM.Unlock()
	syncer.interval = interval
	return nil
}

func (syncer *Syncer) log(lv logger.Level, log ...interface{}) {
	syncer.logger.Println(lv, "time syncer", log...)
}

// Start is used to time syncer
func (syncer *Syncer) Start() error {
	if len(syncer.Clients()) == 0 {
		return ErrNoClients
	}
	// first time sync must success
	sleeper := random.NewSleeper()
	defer sleeper.Stop()
	for {
		err := syncer.synchronize()
		switch err {
		case nil:
			syncer.wg.Add(2)
			go syncer.walker()
			go syncer.synchronizeLoop()
			return nil
		case ErrAllClientsFailed:
			syncer.dnsClient.FlushCache()
			syncer.log(logger.Warning, ErrAllClientsFailed)
			select {
			case <-sleeper.Sleep(syncer.sleepFixed, syncer.sleepFixed):
			case <-syncer.ctx.Done():
				return syncer.ctx.Err()
			}
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
	// "It adjusts the intervals or drops ticks to make up for slow receivers."
	const addLoopInterval = 100 * time.Millisecond
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
	rand := random.New()
	calculateInterval := func() time.Duration {
		extra := syncer.sleepFixed + uint(rand.Int(int(syncer.sleepRandom)))
		return syncer.GetSyncInterval() + time.Duration(extra)*time.Second
	}
	timer := time.NewTimer(calculateInterval())
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			err := syncer.synchronize()
			if err != nil {
				syncer.log(logger.Warning, "failed to synchronize time:", err)
			}
		case <-syncer.ctx.Done():
			return
		}
		timer.Reset(calculateInterval())
	}
}

func (syncer *Syncer) synchronize() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = xpanic.Error(r, "Syncer.synchronize")
			syncer.log(logger.Fatal, err)
		}
	}()
	for tag, client := range syncer.Clients() {
		now, optsErr, err := client.Query()
		if err != nil {
			if optsErr {
				return errors.WithMessagef(err, "client %s with invalid config", tag)
			}
			err = errors.WithMessagef(err, "client %s failed to synchronize time", tag)
			syncer.log(logger.Warning, err)
		} else {
			syncer.updateTime(now)
			return nil
		}
	}
	return ErrAllClientsFailed
}

func (syncer *Syncer) updateTime(now time.Time) {
	syncer.nowRWM.Lock()
	defer syncer.nowRWM.Unlock()
	syncer.now = now
}

// Test is used to test all clients
func (syncer *Syncer) Test(ctx context.Context) error {
	l := len(syncer.clients)
	if l == 0 {
		return ErrNoClients
	}
	errChan := make(chan error, l)
	for tag, client := range syncer.clients {
		if client.SkipTest {
			errChan <- nil
			continue
		}
		go func(tag string, client *Client) {
			var err error
			defer func() {
				if r := recover(); r != nil {
					err = xpanic.Error(r, "Syncer.Test")
				}
				errChan <- err
			}()
			_, _, err = client.Query()
			if err != nil {
				err = errors.WithMessagef(err, "failed to test client %s", tag)
			}
		}(tag, client)
	}
	for i := 0; i < l; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	close(errChan)
	return nil
}

// Stop is used to stop time syncer
func (syncer *Syncer) Stop() {
	syncer.cancel()
	syncer.wg.Wait()
}
