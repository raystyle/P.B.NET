package msfrpc

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/xpanic"
)

const minWatchInterval = 100 * time.Millisecond

// Callbacks contains about all callback functions.
type Callbacks struct {
	// add or delete
	OnToken func(token string, add bool)

	// active or stopped
	OnJob func(id, name string, active bool)

	// opened or closed
	OnSession func(id uint64, info *SessionInfo, opened bool)

	// add or delete
	OnHost func(workspace string, host *DBHost, add bool)
}

// Monitor is used to monitor changes about token list(security), jobs and sessions.
// if msfrpc connected database, it can  monitor hosts, services, browser clients,
// credentials, loots and framework events.
type Monitor struct {
	ctx *MSFRPC

	callbacks *Callbacks
	interval  time.Duration

	// key = token
	tokens    map[string]struct{}
	tokensRWM sync.RWMutex

	// key = id, value = name
	jobs    map[string]string
	jobsRWM sync.RWMutex

	// key = id
	sessions    map[uint64]*SessionInfo
	sessionsRWM sync.RWMutex

	// key = workspace, value = host information
	hosts    map[string]map[*DBHost]struct{}
	hostsRWM sync.RWMutex

	// key = workspace, value = service information
	services    map[string]map[*DBService]struct{}
	servicesRWM sync.RWMutex

	context   context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewMonitor is used to create a monitor.
func (msf *MSFRPC) NewMonitor(callbacks *Callbacks, interval time.Duration) *Monitor {
	if interval < minWatchInterval {
		interval = minWatchInterval
	}
	monitor := &Monitor{
		ctx:       msf,
		callbacks: callbacks,
		interval:  interval,
	}
	monitor.context, monitor.cancel = context.WithCancel(context.Background())
	monitor.wg.Add(3)
	go monitor.tokenMonitor()
	go monitor.jobMonitor()
	go monitor.sessionMonitor()
	return monitor
}

// Tokens is used to get current tokens.
func (monitor *Monitor) Tokens() []string {
	monitor.tokensRWM.RLock()
	defer monitor.tokensRWM.RUnlock()
	l := len(monitor.tokens)
	tokens := make([]string, 0, l)
	for token := range monitor.tokens {
		tokens = append(tokens, token)
	}
	return tokens
}

// Jobs is used to get current jobs, key = id, value = name.
func (monitor *Monitor) Jobs() map[string]string {
	monitor.jobsRWM.RLock()
	defer monitor.jobsRWM.RUnlock()
	jobs := make(map[string]string, len(monitor.jobs))
	for id, name := range monitor.jobs {
		jobs[id] = name
	}
	return jobs
}

// Sessions is used to get current sessions, key = id.
func (monitor *Monitor) Sessions() map[uint64]*SessionInfo {
	monitor.sessionsRWM.RLock()
	defer monitor.sessionsRWM.RUnlock()
	sessions := make(map[uint64]*SessionInfo, len(monitor.sessions))
	for id, info := range monitor.sessions {
		sessions[id] = info
	}
	return sessions
}

// Hosts is used to get hosts by workspace.
func (monitor *Monitor) Hosts(workspace string) ([]*DBHost, error) {
	monitor.hostsRWM.RLock()
	defer monitor.hostsRWM.RUnlock()
	hosts, ok := monitor.hosts[workspace]
	if !ok {
		return nil, errors.Errorf(ErrInvalidWorkspaceFormat, workspace)
	}
	hostsCp := make([]*DBHost, 0, len(hosts))
	for host := range hosts {
		hostsCp = append(hostsCp, host)
	}
	return hostsCp, nil
}

// Services is used to get services by workspace.
func (monitor *Monitor) Services(workspace string) ([]*DBService, error) {
	monitor.servicesRWM.RLock()
	defer monitor.servicesRWM.RUnlock()
	services, ok := monitor.services[workspace]
	if !ok {
		return nil, errors.Errorf(ErrInvalidWorkspaceFormat, workspace)
	}
	servicesCp := make([]*DBService, 0, len(services))
	for service := range services {
		servicesCp = append(servicesCp, service)
	}
	return servicesCp, nil
}

func (monitor *Monitor) log(lv logger.Level, log ...interface{}) {
	monitor.ctx.logger.Println(lv, "msfrpc-monitor", log...)
}

func (monitor *Monitor) tokenMonitor() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.tokenMonitor"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.tokenMonitor()
		} else {
			monitor.wg.Done()
		}
	}()
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			monitor.watchToken()
		case <-monitor.context.Done():
			return
		}
	}
}

func (monitor *Monitor) watchToken() {
	tokens, err := monitor.ctx.AuthTokenList(monitor.context)
	if err != nil {
		return
	}
	l := len(tokens)
	monitor.tokensRWM.Lock()
	defer monitor.tokensRWM.Unlock()
	// initialize map and skip first compare
	if monitor.tokens == nil {
		monitor.tokens = make(map[string]struct{}, l)
		for i := 0; i < l; i++ {
			monitor.tokens[tokens[i]] = struct{}{}
		}
		return
	}
	// check deleted tokens
loop:
	for oToken := range monitor.tokens {
		for i := 0; i < l; i++ {
			if tokens[i] == oToken {
				continue loop
			}
		}
		delete(monitor.tokens, oToken)
		monitor.callbacks.OnToken(oToken, false)
	}
	// check added tokens
	for i := 0; i < l; i++ {
		if _, ok := monitor.tokens[tokens[i]]; !ok {
			monitor.tokens[tokens[i]] = struct{}{}
			monitor.callbacks.OnToken(tokens[i], true)
		}
	}
}

func (monitor *Monitor) jobMonitor() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.jobMonitor"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.jobMonitor()
		} else {
			monitor.wg.Done()
		}
	}()
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			monitor.watchJob()
		case <-monitor.context.Done():
			return
		}
	}
}

func (monitor *Monitor) watchJob() {
	jobs, err := monitor.ctx.JobList(monitor.context)
	if err != nil {
		return
	}
	monitor.jobsRWM.Lock()
	defer monitor.jobsRWM.Unlock()
	// initialize map and skip first compare
	if monitor.jobs == nil {
		monitor.jobs = jobs
		return
	}
	// check stopped jobs
loop:
	for oID, oName := range monitor.jobs {
		for id := range jobs {
			if id == oID {
				continue loop
			}
		}
		delete(monitor.jobs, oID)
		monitor.callbacks.OnJob(oID, oName, false)
	}
	// check active jobs
	for id, name := range jobs {
		if _, ok := monitor.jobs[id]; !ok {
			monitor.jobs[id] = name
			monitor.callbacks.OnJob(id, name, true)
		}
	}
}

func (monitor *Monitor) sessionMonitor() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.sessionMonitor"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.sessionMonitor()
		} else {
			monitor.wg.Done()
		}
	}()
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			monitor.watchSession()
		case <-monitor.context.Done():
			return
		}
	}
}

func (monitor *Monitor) watchSession() {
	sessions, err := monitor.ctx.SessionList(monitor.context)
	if err != nil {
		return
	}
	monitor.sessionsRWM.Lock()
	defer monitor.sessionsRWM.Unlock()
	// initialize map and skip first compare
	if monitor.sessions == nil {
		monitor.sessions = sessions
		return
	}
	// check closed sessions
loop:
	for oID, oInfo := range monitor.sessions {
		for id := range sessions {
			if id == oID {
				continue loop
			}
		}
		delete(monitor.sessions, oID)
		monitor.callbacks.OnSession(oID, oInfo, false)
	}
	// check opened sessions
	for id, info := range sessions {
		if _, ok := monitor.sessions[id]; !ok {
			monitor.sessions[id] = info
			monitor.callbacks.OnSession(id, info, true)
		}
	}
}

// StartDatabaseMonitors is used to start monitors about database.
func (monitor *Monitor) StartDatabaseMonitors() {
	monitor.wg.Add(3)
	go monitor.hostMonitor()
	go monitor.serviceMonitor()
	go monitor.clientMonitor()
}

func (monitor *Monitor) hostMonitor() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.hostMonitor"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.hostMonitor()
		} else {
			monitor.wg.Done()
		}
	}()
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			monitor.watchHost()
		case <-monitor.context.Done():
			return
		}
	}
}

func (monitor *Monitor) watchHost() {
	workspaces, err := monitor.ctx.DBWorkspaces(monitor.context)
	if err != nil {
		return
	}
	for i := 0; i < len(workspaces); i++ {
		monitor.watchHostWithWorkspace(workspaces[i].Name)
	}
}

func (monitor *Monitor) watchHostWithWorkspace(workspace string) {
	hosts, err := monitor.ctx.DBHosts(monitor.context, workspace)
	if err != nil {
		return
	}
	l := len(hosts)
	monitor.hostsRWM.Lock()
	defer monitor.hostsRWM.Unlock()
	// initialize map and skip first compare
	if monitor.hosts == nil {
		monitor.hosts = make(map[string]map[*DBHost]struct{})
		monitor.hosts[workspace] = make(map[*DBHost]struct{}, l)
		for i := 0; i < l; i++ {
			monitor.hosts[workspace][hosts[i]] = struct{}{}
		}
		return
	}
	// create map
	if _, ok := monitor.hosts[workspace]; !ok {
		monitor.hosts[workspace] = make(map[*DBHost]struct{}, l)
	}
	// check deleted hosts
loopDel:
	for oHost := range monitor.hosts[workspace] {
		for i := 0; i < l; i++ {
			if reflect.DeepEqual(hosts[i], oHost) {
				continue loopDel
			}
		}
		delete(monitor.hosts[workspace], oHost)
		monitor.callbacks.OnHost(workspace, oHost, false)
	}
	// check added hosts
loopAdd:
	for i := 0; i < l; i++ {
		for oHost := range monitor.hosts[workspace] {
			if reflect.DeepEqual(oHost, hosts[i]) {
				continue loopAdd
			}
		}
		monitor.hosts[workspace][hosts[i]] = struct{}{}
		monitor.callbacks.OnHost(workspace, hosts[i], true)
	}
}

func (monitor *Monitor) serviceMonitor() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.serviceMonitor"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.serviceMonitor()
		} else {
			monitor.wg.Done()
		}
	}()
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			monitor.watchService()
		case <-monitor.context.Done():
			return
		}
	}
}

func (monitor *Monitor) watchService() {
	workspaces, err := monitor.ctx.DBWorkspaces(monitor.context)
	if err != nil {
		return
	}
	for i := 0; i < len(workspaces); i++ {
		monitor.watchServiceWithWorkspace(workspaces[i].Name)
	}
}

func (monitor *Monitor) watchServiceWithWorkspace(workspace string) {

}

func (monitor *Monitor) clientMonitor() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.clientMonitor"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.clientMonitor()
		} else {
			monitor.wg.Done()
		}
	}()
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			monitor.watchClient()
		case <-monitor.context.Done():
			return
		}
	}
}

func (monitor *Monitor) watchClient() {

}

// Close is used to close monitor.
func (monitor *Monitor) Close() {
	monitor.closeOnce.Do(func() {
		monitor.cancel()
		monitor.wg.Wait()
	})
}
