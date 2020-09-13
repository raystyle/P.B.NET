package msfrpc

import (
	"context"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/xpanic"
)

const minWatchInterval = 250 * time.Millisecond

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

	// add or delete
	OnCredential func(workspace string, cred *DBCred, add bool)

	// add or delete
	OnLoot func(workspace string, loot *DBLoot)

	// report monitor error: msfrpcd or database disconnected, reconnected
	OnEvent func(event string)
}

// Monitor is used to monitor changes about token list(security),
// jobs and sessions. If msfrpc connected database, it can monitor
// hosts, credentials and loots.
//
// Use time.Timer to replace time.Ticker for prevent not sleep if the
// network latency is bigger than Monitor.internal.
type Monitor struct {
	ctx *MSFRPC

	callbacks *Callbacks
	interval  time.Duration
	dbOptions *DBConnectOptions

	// notice if msfrpc or database disconnect
	msfErrorCount int
	dbErrorCount  int

	// store status
	msfAlive atomic.Value
	dbAlive  atomic.Value

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

	// key = workspace, value = credential information
	creds    map[string]map[*DBCred]struct{}
	credsRWM sync.RWMutex

	// key = workspace, value = loot information
	loots    map[string]map[*DBLoot]struct{}
	lootsRWM sync.RWMutex

	context context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewMonitor is used to create a data monitor.
func (msf *MSFRPC) NewMonitor(
	callbacks *Callbacks,
	interval time.Duration,
	dbOpts *DBConnectOptions,
) *Monitor {
	if interval < minWatchInterval {
		interval = minWatchInterval
	}
	monitor := &Monitor{
		ctx:       msf,
		callbacks: callbacks,
		interval:  interval,
		dbOptions: dbOpts,
	}
	monitor.msfAlive.Store(true)
	monitor.dbAlive.Store(true)
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
	if monitor.tokens == nil {
		return nil
	}
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
	if monitor.jobs == nil {
		return nil
	}
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
	if monitor.sessions == nil {
		return nil
	}
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
	// not connected database
	if monitor.hosts == nil {
		return nil, nil
	}
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

// Credentials is used to get credentials by workspace.
func (monitor *Monitor) Credentials(workspace string) ([]*DBCred, error) {
	monitor.credsRWM.RLock()
	defer monitor.credsRWM.RUnlock()
	// not connected database
	if monitor.creds == nil {
		return nil, nil
	}
	creds, ok := monitor.creds[workspace]
	if !ok {
		return nil, errors.Errorf(ErrInvalidWorkspaceFormat, workspace)
	}
	credsCp := make([]*DBCred, 0, len(creds))
	for cred := range creds {
		credsCp = append(credsCp, cred)
	}
	return credsCp, nil
}

// Loots is used to get loots by workspace. warning: Loot.Data is nil.
func (monitor *Monitor) Loots(workspace string) ([]*DBLoot, error) {
	monitor.lootsRWM.RLock()
	defer monitor.lootsRWM.RUnlock()
	// not connected database
	if monitor.loots == nil {
		return nil, nil
	}
	loots, ok := monitor.loots[workspace]
	if !ok {
		return nil, errors.Errorf(ErrInvalidWorkspaceFormat, workspace)
	}
	lootsCp := make([]*DBLoot, 0, len(loots))
	for loot := range loots {
		lootsCp = append(lootsCp, loot)
	}
	return lootsCp, nil
}

func (monitor *Monitor) log(lv logger.Level, log ...interface{}) {
	monitor.ctx.logger.Println(lv, "msfrpc-monitor", log...)
}

func (monitor *Monitor) updateMSFErrorCount(add bool) {
	// reset counter
	if !add {
		if monitor.msfErrorCount != 0 {
			monitor.msfErrorCount = 0
			monitor.msfAlive.Store(true)
			const log = "msfrpcd reconnected"
			monitor.log(logger.Info, log)
			monitor.callbacks.OnEvent(log)
		}
		return
	}
	select {
	case <-monitor.context.Done():
		return
	default:
	}
	monitor.msfErrorCount++
	// if use temporary token, need login again.
	if monitor.ctx.GetToken()[:4] == "TEMP" {
		err := monitor.ctx.AuthLogin()
		if err == nil {
			return
		}
	}
	if monitor.msfErrorCount != 3 { // core! core! core!
		return
	}
	monitor.msfAlive.Store(false)
	const log = "msfrpcd disconnected"
	monitor.log(logger.Warning, log)
	monitor.callbacks.OnEvent(log)
}

func (monitor *Monitor) updateDBErrorCount(add bool) {
	// reset counter
	if !add {
		if monitor.dbErrorCount != 0 {
			monitor.dbErrorCount = 0
			monitor.dbAlive.Store(true)
			const log = "database reconnected"
			monitor.log(logger.Info, log)
			monitor.callbacks.OnEvent(log)
		}
		return
	}
	select {
	case <-monitor.context.Done():
		return
	default:
	}
	monitor.dbErrorCount++
	// try to reconnect database
	err := monitor.ctx.DBConnect(monitor.context, monitor.dbOptions)
	if err == nil {
		return
	}
	if monitor.dbErrorCount != 3 { // core! core! core!
		return
	}
	monitor.dbAlive.Store(false)
	const log = "database disconnected"
	monitor.log(logger.Warning, log)
	monitor.callbacks.OnEvent(log)
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
	timer := time.NewTimer(monitor.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			monitor.watchToken()
		case <-monitor.context.Done():
			return
		}
		timer.Reset(monitor.interval)
	}
}

func (monitor *Monitor) watchToken() {
	tokens, err := monitor.ctx.AuthTokenList(monitor.context)
	if err != nil {
		monitor.log(logger.Debug, "failed to watch token:", err)
		monitor.updateMSFErrorCount(true)
		return
	}
	monitor.updateMSFErrorCount(false)
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
	timer := time.NewTimer(monitor.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			monitor.watchJob()
		case <-monitor.context.Done():
			return
		}
		timer.Reset(monitor.interval)
	}
}

func (monitor *Monitor) watchJob() {
	jobs, err := monitor.ctx.JobList(monitor.context)
	if err != nil {
		monitor.log(logger.Debug, "failed to watch job:", err)
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
	timer := time.NewTimer(monitor.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			monitor.watchSession()
		case <-monitor.context.Done():
			return
		}
		timer.Reset(monitor.interval)
	}
}

func (monitor *Monitor) watchSession() {
	sessions, err := monitor.ctx.SessionList(monitor.context)
	if err != nil {
		monitor.log(logger.Debug, "failed to watch session:", err)
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
	monitor.wg.Add(4)
	go monitor.hostMonitor()
	go monitor.credentialMonitor()
	go monitor.lootMonitor()
	go monitor.workspaceCleaner()
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
	timer := time.NewTimer(monitor.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			monitor.watchHost()
		case <-monitor.context.Done():
			return
		}
		timer.Reset(monitor.interval)
	}
}

func (monitor *Monitor) watchHost() {
	workspaces, err := monitor.ctx.DBWorkspaces(monitor.context)
	if err != nil {
		monitor.log(logger.Debug, "failed to get workspaces for watch host:", err)
		monitor.updateDBErrorCount(true)
		return
	}
	monitor.updateDBErrorCount(false)
	for i := 0; i < len(workspaces); i++ {
		monitor.watchHostWithWorkspace(workspaces[i].Name)
	}
}

func (monitor *Monitor) watchHostWithWorkspace(workspace string) {
	hosts, err := monitor.ctx.DBHosts(monitor.context, workspace)
	if err != nil {
		monitor.log(logger.Debug, "failed to watch host:", err)
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
	// create map for new workspace
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

func (monitor *Monitor) credentialMonitor() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.credentialMonitor"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.credentialMonitor()
		} else {
			monitor.wg.Done()
		}
	}()
	timer := time.NewTimer(monitor.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			monitor.watchCredential()
		case <-monitor.context.Done():
			return
		}
		timer.Reset(monitor.interval)
	}
}

func (monitor *Monitor) watchCredential() {
	workspaces, err := monitor.ctx.DBWorkspaces(monitor.context)
	if err != nil {
		monitor.log(logger.Debug, "failed to get workspaces for watch credential:", err)
		return
	}
	for i := 0; i < len(workspaces); i++ {
		monitor.watchCredentialWithWorkspace(workspaces[i].Name)
	}
}

func (monitor *Monitor) watchCredentialWithWorkspace(workspace string) {
	creds, err := monitor.ctx.DBCreds(monitor.context, workspace)
	if err != nil {
		monitor.log(logger.Debug, "failed to get credential:", err)
		return
	}
	l := len(creds)
	monitor.credsRWM.Lock()
	defer monitor.credsRWM.Unlock()
	// initialize map and skip first compare
	if monitor.creds == nil {
		monitor.creds = make(map[string]map[*DBCred]struct{})
		monitor.creds[workspace] = make(map[*DBCred]struct{}, l)
		for i := 0; i < l; i++ {
			monitor.creds[workspace][creds[i]] = struct{}{}
		}
		return
	}
	// create map for new workspace
	if _, ok := monitor.creds[workspace]; !ok {
		monitor.creds[workspace] = make(map[*DBCred]struct{}, l)
	}
	// check deleted credentials
loopDel:
	for oCred := range monitor.creds[workspace] {
		for i := 0; i < l; i++ {
			if reflect.DeepEqual(creds[i], oCred) {
				continue loopDel
			}
		}
		delete(monitor.creds[workspace], oCred)
		monitor.callbacks.OnCredential(workspace, oCred, false)
	}
	// check added credentials
loopAdd:
	for i := 0; i < l; i++ {
		for oCred := range monitor.creds[workspace] {
			if reflect.DeepEqual(oCred, creds[i]) {
				continue loopAdd
			}
		}
		monitor.creds[workspace][creds[i]] = struct{}{}
		monitor.callbacks.OnCredential(workspace, creds[i], true)
	}
}

func (monitor *Monitor) lootMonitor() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.lootMonitor"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.lootMonitor()
		} else {
			monitor.wg.Done()
		}
	}()
	timer := time.NewTimer(monitor.interval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			monitor.watchLoot()
		case <-monitor.context.Done():
			return
		}
		timer.Reset(monitor.interval)
	}
}

func (monitor *Monitor) watchLoot() {
	workspaces, err := monitor.ctx.DBWorkspaces(monitor.context)
	if err != nil {
		monitor.log(logger.Debug, "failed to get workspaces for watch loot:", err)
		return
	}
	for i := 0; i < len(workspaces); i++ {
		monitor.watchLootWithWorkspace(workspaces[i].Name)
	}
}

func (monitor *Monitor) watchLootWithWorkspace(workspace string) {
	opts := DBLootsOptions{
		Workspace: workspace,
		Limit:     math.MaxUint32,
	}
	loots, err := monitor.ctx.DBLoots(monitor.context, &opts)
	if err != nil {
		monitor.log(logger.Debug, "failed to get loot:", err)
		return
	}
	l := len(loots)
	monitor.lootsRWM.Lock()
	defer monitor.lootsRWM.Unlock()
	// initialize map and skip first compare
	if monitor.loots == nil {
		monitor.loots = make(map[string]map[*DBLoot]struct{})
		monitor.loots[workspace] = make(map[*DBLoot]struct{}, l)
		for i := 0; i < l; i++ {
			monitor.loots[workspace][loots[i]] = struct{}{}
		}
		return
	}
	// create map for new workspace
	if _, ok := monitor.loots[workspace]; !ok {
		monitor.loots[workspace] = make(map[*DBLoot]struct{}, l)
	}
	// check added loots
loop:
	for i := 0; i < l; i++ {
		for oLoot := range monitor.loots[workspace] {
			if reflect.DeepEqual(oLoot, loots[i]) {
				continue loop
			}
		}
		monitor.loots[workspace][loots[i]] = struct{}{}
		monitor.callbacks.OnLoot(workspace, loots[i])
	}
	// clean big data (less memory)
	for i := 0; i < l; i++ {
		loots[i].Data = nil
	}
}

func (monitor *Monitor) workspaceCleaner() {
	defer func() {
		if r := recover(); r != nil {
			monitor.log(logger.Fatal, xpanic.Print(r, "Monitor.workspaceCleaner"))
			// restart monitor
			time.Sleep(time.Second)
			go monitor.workspaceCleaner()
		} else {
			monitor.wg.Done()
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			monitor.cleanWorkspace()
		case <-monitor.context.Done():
			return
		}
	}
}

// delete doesn't exist workspace.
func (monitor *Monitor) cleanWorkspace() {
	workspaces, err := monitor.ctx.DBWorkspaces(monitor.context)
	if err != nil {
		monitor.log(logger.Debug, "failed to get workspaces for clean workspace:", err)
		return
	}
	l := len(workspaces)
	monitor.cleanHosts(workspaces, l)
	monitor.cleanCreds(workspaces, l)
	monitor.cleanLoots(workspaces, l)
}

func (monitor *Monitor) cleanHosts(workspaces []*DBWorkspace, l int) {
	monitor.hostsRWM.Lock()
	defer monitor.hostsRWM.Unlock()
loop:
	for workspace := range monitor.hosts {
		for i := 0; i < l; i++ {
			if workspace == workspaces[i].Name {
				continue loop
			}
		}
		delete(monitor.hosts, workspace)
	}
}

func (monitor *Monitor) cleanCreds(workspaces []*DBWorkspace, l int) {
	monitor.credsRWM.Lock()
	defer monitor.credsRWM.Unlock()
loop:
	for workspace := range monitor.creds {
		for i := 0; i < l; i++ {
			if workspace == workspaces[i].Name {
				continue loop
			}
		}
		delete(monitor.creds, workspace)
	}
}

func (monitor *Monitor) cleanLoots(workspaces []*DBWorkspace, l int) {
	monitor.lootsRWM.Lock()
	defer monitor.lootsRWM.Unlock()
loop:
	for workspace := range monitor.loots {
		for i := 0; i < l; i++ {
			if workspace == workspaces[i].Name {
				continue loop
			}
		}
		delete(monitor.loots, workspace)
	}
}

// MSFRPCDAlive is used to check msfrpcd is connected.
func (monitor *Monitor) MSFRPCDAlive() bool {
	return monitor.msfAlive.Load().(bool)
}

// DatabaseAlive is used to check database is connected.
func (monitor *Monitor) DatabaseAlive() bool {
	return monitor.dbAlive.Load().(bool)
}

// Close is used to close monitor.
func (monitor *Monitor) Close() {
	monitor.cancel()
	monitor.wg.Wait()
}
