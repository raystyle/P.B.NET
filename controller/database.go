package controller

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	_ "project/internal/gorm"
	"project/internal/logger"
	"project/internal/xpanic"
)

var (
	errNoCache = errors.New("can't find cache")
)

type db struct {
	ctx         *CTRL // use cache
	dbLogger    *dbLogger
	gormLogger  *gormLogger
	db          *gorm.DB
	cacheSyncer *cacheSyncer
	stopSignal  chan struct{}
	wg          sync.WaitGroup
}

func newDB(ctx *CTRL, cfg *Config) (*db, error) {
	// set db logger
	dbLogger, err := newDBLogger(cfg.Dialect, cfg.DBLogFile)
	if err != nil {
		return nil, err
	}
	// if you need, add DB Driver
	switch cfg.Dialect {
	case "mysql":
		_ = mysql.SetLogger(dbLogger)
	default:
		return nil, errors.Errorf("unknown database dialect: %s", cfg.Dialect)
	}
	// connect database
	gormDB, err := gorm.Open(cfg.Dialect, cfg.DSN)
	if err != nil {
		return nil, errors.Wrapf(err, "connect %s server failed", cfg.Dialect)
	}
	gormDB.SingularTable(true) // not add s
	// connection
	gormDB.DB().SetMaxOpenConns(cfg.DBMaxOpenConns)
	gormDB.DB().SetMaxIdleConns(cfg.DBMaxIdleConns)
	// gorm logger
	gormLogger, err := newGormLogger(cfg.GORMLogFile)
	if err != nil {
		return nil, err
	}
	gormDB.SetLogger(gormLogger)
	if cfg.GORMDetailedLog {
		gormDB.LogMode(true)
	}
	db := db{
		ctx:        ctx,
		dbLogger:   dbLogger,
		gormLogger: gormLogger,
		db:         gormDB,
		stopSignal: make(chan struct{}),
	}
	db.cacheSyncer = &cacheSyncer{
		ctx:          &db,
		syncInterval: cfg.DBSyncInterval,
	}
	db.wg.Add(1)
	go db.cacheSyncer.SyncLoop()
	return &db, nil
}

func (db *db) Close() {
	close(db.stopSignal)
	db.wg.Wait()
	_ = db.db.Close()
	db.gormLogger.Close()
	db.dbLogger.Close()
}

func (db *db) logf(l logger.Level, format string, log ...interface{}) {
	db.ctx.Printf(l, "db", format, log...)
}

func (db *db) log(l logger.Level, log ...interface{}) {
	db.ctx.Print(l, "db", log...)
}

func (db *db) logln(l logger.Level, log ...interface{}) {
	db.ctx.Println(l, "db", log...)
}

func (db *db) InsertCtrlLog(m *mCtrlLog) error {
	return db.db.Table(tableLog).Create(m).Error
}

// -------------------------------proxy client----------------------------------------

func (db *db) InsertProxyClient(m *mProxyClient) error {
	return db.db.Create(m).Error
}

func (db *db) SelectProxyClient() ([]*mProxyClient, error) {
	var clients []*mProxyClient
	return clients, db.db.Find(&clients).Error
}

func (db *db) UpdateProxyClient(m *mProxyClient) error {
	return db.db.Save(m).Error
}

func (db *db) DeleteProxyClient(id uint64) error {
	return db.db.Delete(&mProxyClient{ID: id}).Error
}

// ---------------------------------dns client----------------------------------------

func (db *db) InsertDNSServer(m *mDNSServer) error {
	return db.db.Create(m).Error
}

func (db *db) SelectDNSServer() ([]*mDNSServer, error) {
	var clients []*mDNSServer
	return clients, db.db.Find(&clients).Error
}

func (db *db) UpdateDNSServer(m *mDNSServer) error {
	return db.db.Save(m).Error
}

func (db *db) DeleteDNSServer(id uint64) error {
	return db.db.Delete(&mDNSServer{ID: id}).Error
}

// -----------------------------time syncer config------------------------------------

func (db *db) InsertTimeSyncer(m *mTimeSyncer) error {
	return db.db.Create(m).Error
}

func (db *db) SelectTimeSyncer() ([]*mTimeSyncer, error) {
	var timeSyncer []*mTimeSyncer
	return timeSyncer, db.db.Find(&timeSyncer).Error
}

func (db *db) UpdateTimeSyncer(m *mTimeSyncer) error {
	return db.db.Save(m).Error
}

func (db *db) DeleteTimeSyncer(id uint64) error {
	return db.db.Delete(&mTimeSyncer{ID: id}).Error
}

// -------------------------------------boot------------------------------------------

func (db *db) InsertBoot(m *mBoot) error {
	return db.db.Create(m).Error
}

func (db *db) SelectBoot() ([]*mBoot, error) {
	var boot []*mBoot
	return boot, db.db.Find(&boot).Error
}

func (db *db) UpdateBoot(m *mBoot) error {
	return db.db.Save(m).Error
}

func (db *db) DeleteBoot(id uint64) error {
	return db.db.Delete(&mBoot{ID: id}).Error
}

// ----------------------------------listener-----------------------------------------

func (db *db) InsertListener(m *mListener) error {
	return db.db.Create(m).Error
}

func (db *db) SelectListener() ([]*mListener, error) {
	var listener []*mListener
	return listener, db.db.Find(&listener).Error
}

func (db *db) UpdateListener(m *mListener) error {
	return db.db.Save(m).Error
}

func (db *db) DeleteListener(id uint64) error {
	return db.db.Delete(&mListener{ID: id}).Error
}

// ------------------------------------node-------------------------------------------

func (db *db) SelectNode(guid []byte) (node *mNode, err error) {
	node = db.ctx.cache.SelectNode(guid)
	if node != nil {
		return
	}
	// node syncer must be loaded first
	mns := new(mNodeSyncer)
	err = db.db.Find(mns, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.ctx.cache.InsertNodeSyncer(mns)
	// load node
	node = new(mNode)
	err = db.db.Find(node, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.ctx.cache.InsertNode(node)
	return
}

func (db *db) InsertNode(m *mNode) error {
	tx := db.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	err := tx.Error
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	err = tx.Create(m).Error
	if err != nil {
		return err
	}
	err = tx.Create(&mNodeSyncer{GUID: m.GUID}).Error
	if err != nil {
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		return err
	}
	return nil
}

func (db *db) DeleteNode(guid []byte) error {
	tx := db.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	err := tx.Error
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	err = tx.Delete(&mNode{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&mNodeSyncer{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&mNodeListener{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Table(tableNodeLog).Delete(&mRoleLog{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Commit().Error
	return err
}

func (db *db) DeleteNodeUnscoped(guid []byte) error {
	return db.db.Unscoped().Delete(&mNode{}, "guid = ?", guid).Error
}

func (db *db) InsertNodeListener(m *mNodeListener) error {
	return db.db.Create(m).Error
}

func (db *db) DeleteNodeListener(id uint64) error {
	return db.db.Delete(&mNodeListener{ID: id}).Error
}

func (db *db) InsertNodeLog(m *mRoleLog) error {
	return db.db.Table(tableNodeLog).Create(m).Error
}

func (db *db) DeleteNodeLog(id uint64) error {
	return db.db.Table(tableNodeLog).Delete(&mRoleLog{ID: id}).Error
}

// -----------------------------------beacon------------------------------------------

func (db *db) SelectBeacon(guid []byte) (beacon *mBeacon, err error) {
	beacon = db.ctx.cache.SelectBeacon(guid)
	if beacon != nil {
		return
	}
	// beacon syncer must be loaded first
	mbs := new(mBeaconSyncer)
	err = db.db.Find(mbs, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.ctx.cache.InsertBeaconSyncer(mbs)
	// load beacon
	beacon = new(mBeacon)
	err = db.db.Find(beacon, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.ctx.cache.InsertBeacon(beacon)
	return
}

func (db *db) InsertBeacon(m *mBeacon) error {
	tx := db.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	err := tx.Error
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	err = tx.Create(m).Error
	if err != nil {
		return err
	}
	err = tx.Create(&mBeaconSyncer{GUID: m.GUID}).Error
	if err != nil {
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		return err
	}
	return nil
}

func (db *db) DeleteBeacon(guid []byte) error {
	tx := db.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	err := tx.Error
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()
	err = tx.Delete(&mBeacon{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&mBeaconSyncer{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&mBeaconListener{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Table(tableBeaconLog).Delete(&mRoleLog{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Commit().Error
	return err
}

func (db *db) DeleteBeaconUnscoped(guid []byte) error {
	return db.db.Unscoped().Delete(&mBeacon{}, "guid = ?", guid).Error
}

func (db *db) InsertBeaconListener(m *mBeaconListener) error {
	return db.db.Create(m).Error
}

func (db *db) DeleteBeaconListener(id uint64) error {
	return db.db.Delete(&mBeaconListener{ID: id}).Error
}

func (db *db) InsertBeaconLog(m *mRoleLog) error {
	return db.db.Table(tableBeaconLog).Create(m).Error
}

func (db *db) DeleteBeaconLog(id uint64) error {
	return db.db.Table(tableBeaconLog).Delete(&mRoleLog{ID: id}).Error
}

// -----------------------------sync message(node)------------------------------------
// NS = Node Syncer

func (db *db) SelectNodeSyncer(guid []byte) (ns *nodeSyncer, err error) {
	ns = db.ctx.cache.SelectNodeSyncer(guid)
	if ns == nil {
		return nil, errNoCache
	}
	return
}

// must save to database immediately
func (db *db) UpdateNSCtrlSend(guid []byte, height uint64) (err error) {
	err = db.db.Save(&mNodeSyncer{GUID: guid, CtrlSend: height}).Error
	if err != nil {
		return
	}
	ns := db.ctx.cache.SelectNodeSyncer(guid)
	if ns == nil {
		return errNoCache
	}
	ns.Lock()
	ns.CtrlSend = height
	ns.Unlock()
	return
}

// only write to cache
func (db *db) UpdateNSNodeReceive(guid []byte, height uint64) (err error) {
	ns := db.ctx.cache.SelectNodeSyncer(guid)
	if ns == nil {
		return errNoCache
	}
	ns.Lock()
	if height > ns.NodeRecv {
		ns.NodeRecv = height
	}
	ns.Unlock()
	return
}

// only write to cache
func (db *db) UpdateNSNodeSend(guid []byte, height uint64) (err error) {
	ns := db.ctx.cache.SelectNodeSyncer(guid)
	if ns == nil {
		return errNoCache
	}
	ns.Lock()
	if height > ns.NodeSend {
		ns.NodeSend = height
	}
	ns.Unlock()
	return
}

// only write to cache
func (db *db) UpdateNSCtrlReceive(guid []byte, height uint64) (err error) {
	ns := db.ctx.cache.SelectNodeSyncer(guid)
	if ns == nil {
		return errNoCache
	}
	ns.Lock()
	ns.CtrlRecv = height
	ns.Unlock()
	return
}

// ----------------------------sync message(beacon)-----------------------------------
// BS = Beacon Syncer

func (db *db) SelectBeaconSyncer(guid []byte) (bs *beaconSyncer, err error) {
	bs = db.ctx.cache.SelectBeaconSyncer(guid)
	if bs == nil {
		return nil, errNoCache
	}
	return
}

// must save to database immediately
func (db *db) UpdateBSCtrlSend(guid []byte, height uint64) (err error) {
	err = db.db.Save(&mBeaconSyncer{GUID: guid, CtrlSend: height}).Error
	if err != nil {
		return
	}
	bs := db.ctx.cache.SelectBeaconSyncer(guid)
	if bs == nil {
		return errNoCache
	}
	bs.Lock()
	bs.CtrlSend = height
	bs.Unlock()
	return
}

// can write to cache
func (db *db) UpdateBSBeaconReceive(guid []byte, height uint64) (err error) {
	bs := db.ctx.cache.SelectBeaconSyncer(guid)
	if bs == nil {
		return errNoCache
	}
	bs.Lock()
	if height > bs.BeaconRecv {
		bs.BeaconRecv = height
	}
	bs.Unlock()
	return
}

// can write to cache
func (db *db) UpdateBSBeaconSend(guid []byte, height uint64) (err error) {
	bs := db.ctx.cache.SelectBeaconSyncer(guid)
	if bs == nil {
		return errNoCache
	}
	bs.Lock()
	if height > bs.BeaconSend {
		bs.BeaconSend = height
	}
	bs.Unlock()
	return
}

// can write to cache
func (db *db) UpdateBSCtrlReceive(guid []byte, height uint64) (err error) {
	bs := db.ctx.cache.SelectBeaconSyncer(guid)
	if bs == nil {
		return errNoCache
	}
	bs.Lock()
	bs.CtrlRecv = height
	bs.Unlock()
	return
}

type cacheSyncer struct {
	ctx          *db
	syncInterval time.Duration

	same     bool
	key      string
	nsCaches map[string]*nodeSyncer
	nsDBs    map[string]*nodeSyncerDB
	bsCaches map[string]*beaconSyncer
	bsDBs    map[string]*beaconSyncerDB
	nsCache  *nodeSyncer
	nsDB     *nodeSyncerDB
	bsCache  *beaconSyncer
	bsDB     *beaconSyncerDB
	err      error

	// if sync to database failed rollback
	tmpDBRoleRecv uint64
	tmpDBRoleSend uint64
	tmpDBCtrlRecv uint64

	// used to insert database
	tmpNSDB *mNodeSyncer
	tmpBSDB *mBeaconSyncer

	syncM sync.Mutex
}

func (cs *cacheSyncer) Sync() {
	cs.syncM.Lock()
	defer cs.syncM.Unlock()
	// ---------------------node syncer------------------------
	cs.nsCaches = cs.ctx.ctx.cache.SelectAllNodeSyncer()
	cs.nsDBs = cs.ctx.ctx.cache.SelectAllNodeSyncerDB()
	for cs.key, cs.nsCache = range cs.nsCaches {
		cs.same = true
		cs.nsDB = cs.nsDBs[cs.key]
		// maybe lost
		if cs.nsDB == nil {
			continue
		}
		cs.nsDB.Lock()
		cs.nsCache.RLock()
		cs.tmpDBRoleRecv = cs.nsDB.NodeRecv
		if cs.nsDB.NodeRecv != cs.nsCache.NodeRecv {
			cs.nsDB.NodeRecv = cs.nsCache.NodeRecv
			cs.same = false
		}
		cs.tmpDBRoleSend = cs.nsDB.NodeSend
		if cs.nsDB.NodeSend != cs.nsCache.NodeSend {
			cs.nsDB.NodeSend = cs.nsCache.NodeSend
			cs.same = false
		}
		cs.tmpDBCtrlRecv = cs.nsDB.CtrlRecv
		if cs.nsDB.CtrlRecv != cs.nsCache.CtrlRecv {
			cs.nsDB.CtrlRecv = cs.nsCache.CtrlRecv
			cs.same = false
		}
		if !cs.same { // sync to database
			cs.tmpNSDB.GUID = cs.nsCache.GUID
			cs.nsCache.RUnlock()
			cs.tmpNSDB.CtrlSend = cs.nsDB.CtrlSend
			cs.tmpNSDB.NodeRecv = cs.nsDB.NodeRecv
			cs.tmpNSDB.NodeSend = cs.nsDB.NodeSend
			cs.tmpNSDB.CtrlRecv = cs.nsDB.CtrlRecv
			cs.err = cs.ctx.db.Save(cs.tmpNSDB).Error
			if cs.err != nil {
				// rollback
				cs.nsDB.NodeRecv = cs.tmpDBRoleRecv
				cs.nsDB.NodeSend = cs.tmpDBRoleSend
				cs.nsDB.CtrlRecv = cs.tmpDBCtrlRecv
				cs.nsDB.Unlock()
				cs.ctx.log(logger.Error, "cache syncer synchronize failed:", cs.err)
				return
			}
		} else {
			cs.nsCache.RUnlock()
		}
		cs.nsDB.Unlock()
	}
	// --------------------beacon syncer-----------------------
	cs.bsCaches = cs.ctx.ctx.cache.SelectAllBeaconSyncer()
	cs.bsDBs = cs.ctx.ctx.cache.SelectAllBeaconSyncerDB()
	for cs.key, cs.bsCache = range cs.bsCaches {
		cs.same = true
		cs.bsDB = cs.bsDBs[cs.key]
		// maybe lost
		if cs.bsDB == nil {
			continue
		}
		cs.bsCache.RLock()
		cs.bsDB.Lock()
		cs.tmpDBRoleRecv = cs.bsDB.BeaconRecv
		if cs.bsDB.BeaconRecv != cs.bsCache.BeaconRecv {
			cs.bsDB.BeaconRecv = cs.bsCache.BeaconRecv
			cs.same = false
		}
		cs.tmpDBRoleSend = cs.bsDB.BeaconSend
		if cs.bsDB.BeaconSend != cs.bsCache.BeaconSend {
			cs.bsDB.BeaconSend = cs.bsCache.BeaconSend
			cs.same = false
		}
		cs.tmpDBCtrlRecv = cs.bsDB.CtrlRecv
		if cs.bsDB.CtrlRecv != cs.bsCache.CtrlRecv {
			cs.bsDB.CtrlRecv = cs.bsCache.CtrlRecv
			cs.same = false
		}
		if !cs.same { // sync to database
			cs.tmpBSDB.GUID = cs.bsCache.GUID
			cs.bsCache.RUnlock()
			cs.tmpBSDB.CtrlSend = cs.bsDB.CtrlSend
			cs.tmpBSDB.BeaconRecv = cs.bsDB.BeaconRecv
			cs.tmpBSDB.BeaconSend = cs.bsDB.BeaconSend
			cs.tmpBSDB.CtrlRecv = cs.bsDB.CtrlRecv
			cs.err = cs.ctx.db.Save(cs.tmpBSDB).Error
			if cs.err != nil {
				// rollback
				cs.bsDB.BeaconRecv = cs.tmpDBRoleRecv
				cs.bsDB.BeaconSend = cs.tmpDBRoleSend
				cs.bsDB.CtrlRecv = cs.tmpDBCtrlRecv
				cs.bsDB.Unlock()
				cs.ctx.log(logger.Error, "cache syncer synchronize failed:", cs.err)
				return
			}
		} else {
			cs.bsCache.RUnlock()
		}
		cs.bsDB.Unlock()
	}
}

func (cs *cacheSyncer) SyncLoop() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("db cache syncer panic:", r)
			cs.ctx.log(logger.Fatal, err)
			// restart cache syncer
			time.Sleep(time.Second)
			cs.ctx.wg.Add(1)
			go cs.SyncLoop()
		}
		defer cs.ctx.wg.Done()
	}()
	ticker := time.NewTicker(cs.syncInterval)
	defer ticker.Stop()
	// used to insert database
	cs.tmpNSDB = new(mNodeSyncer)
	cs.tmpBSDB = new(mBeaconSyncer)
	cs.Sync()
	for {
		select {
		case <-ticker.C:
			cs.Sync()
		case <-cs.ctx.stopSignal:
			cs.Sync()
			return
		}
	}
}
