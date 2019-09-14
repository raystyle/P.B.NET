package controller

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/xpanic"
)

var (
	errNoCache = errors.New("can't find cache")
)

func init() {
	// gorm custom namer: table name delete "m"
	// table "mProxyClient" -> "m_proxy_client" -> "proxy_client"
	namer := gorm.TheNamingStrategy.Table
	gorm.TheNamingStrategy.Table = func(name string) string {
		return namer(name)[2:]
	}
}

type db struct {
	ctx          *CTRL // use cache
	syncInterval time.Duration
	dbLogger     *dbLogger
	gormLogger   *gormLogger
	db           *gorm.DB
	stopSignal   chan struct{}
	wg           sync.WaitGroup
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
		ctx:          ctx,
		syncInterval: cfg.DBSyncInterval,
		dbLogger:     dbLogger,
		gormLogger:   gormLogger,
		db:           gormDB,
		stopSignal:   make(chan struct{}),
	}
	db.wg.Add(1)
	go db.cacheSyncer()
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
	node = new(mNode)
	err = db.db.Find(node, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.ctx.cache.InsertNode(node)
	// must load syncer
	mns := new(mNodeSyncer)
	err = db.db.Find(mns, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.ctx.cache.InsertNodeSyncer(mns)
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
	beacon = new(mBeacon)
	err = db.db.Find(beacon, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.ctx.cache.InsertBeacon(beacon)
	// must load syncer
	mbs := new(mBeaconSyncer)
	err = db.db.Find(mbs, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.ctx.cache.InsertBeaconSyncer(mbs)
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
	if ns != nil {
		return nil, errNoCache
	}
	return
}

// must save to database immediately
func (db *db) UpdateNSCtrlSend(guid []byte, height uint64) error {
	err := db.db.Save(&mNodeSyncer{GUID: guid, CtrlSend: height}).Error
	if err != nil {
		return err
	}
	ns := db.ctx.cache.SelectNodeSyncer(guid)
	if ns == nil {
		return errNoCache
	}
	ns.Lock()
	ns.CtrlSend = height
	ns.Unlock()
	return nil
}

// only write to cache
func (db *db) UpdateNSNodeReceive(guid []byte, height uint64) error {
	ns := db.ctx.cache.SelectNodeSyncer(guid)
	if ns == nil {
		return errNoCache
	}
	ns.Lock()
	ns.NodeRecv = height
	ns.Unlock()
	return nil
}

// only write to cache
func (db *db) UpdateNSNodeSend(guid []byte, height uint64) error {
	ns := db.ctx.cache.SelectNodeSyncer(guid)
	if ns == nil {
		return errNoCache
	}
	ns.Lock()
	ns.NodeSend = height
	ns.Unlock()
	return nil
}

// only write to cache
func (db *db) UpdateNSCtrlReceive(guid []byte, height uint64) error {
	ns := db.ctx.cache.SelectNodeSyncer(guid)
	if ns == nil {
		return errNoCache
	}
	ns.Lock()
	ns.CtrlRecv = height
	ns.Unlock()
	return nil
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
func (db *db) UpdateBSCtrlSend(guid []byte, height uint64) error {
	err := db.db.Save(&mBeaconSyncer{GUID: guid, CtrlSend: height}).Error
	if err != nil {
		return err
	}
	bs := db.ctx.cache.SelectBeaconSyncer(guid)
	if bs == nil {
		return errNoCache
	}
	bs.Lock()
	bs.CtrlSend = height
	bs.Unlock()
	return nil
}

// can write to cache
func (db *db) UpdateBSBeaconReceive(guid []byte, height uint64) error {
	bs := db.ctx.cache.SelectBeaconSyncer(guid)
	if bs == nil {
		return errNoCache
	}
	bs.Lock()
	bs.BeaconRecv = height
	bs.Unlock()
	return nil
}

// can write to cache
func (db *db) UpdateBSBeaconSend(guid []byte, height uint64) error {
	bs := db.ctx.cache.SelectBeaconSyncer(guid)
	if bs == nil {
		return errNoCache
	}
	bs.Lock()
	bs.BeaconSend = height
	bs.Unlock()
	return nil
}

// can write to cache
func (db *db) UpdateBSCtrlReceive(guid []byte, height uint64) error {
	bs := db.ctx.cache.SelectBeaconSyncer(guid)
	if bs == nil {
		return errNoCache
	}
	bs.Lock()
	bs.CtrlRecv = height
	bs.Unlock()
	return nil
}

// cacheSyncer is used to sync roleSyncer to database
func (db *db) cacheSyncer() {
	defer func() {
		if r := recover(); r != nil {
			err := xpanic.Error("db cache syncer panic:", r)
			db.log(logger.Fatal, err)
			// restart cache syncer
			time.Sleep(time.Second)
			db.wg.Add(1)
			go db.cacheSyncer()
		}
		defer db.wg.Done()
	}()
	ticker := time.NewTicker(db.syncInterval)
	defer ticker.Stop()
	var (
		same     bool
		key      string
		nsCaches map[string]*nodeSyncer
		nsDBs    map[string]*nodeSyncer
		bsCaches map[string]*beaconSyncer
		bsDBs    map[string]*beaconSyncer
		nsCache  *nodeSyncer
		nsDB     *nodeSyncer
		bsCache  *beaconSyncer
		bsDB     *beaconSyncer
		// if sync to database failed rollback
		tmpDBRoleRecv uint64
		tmpDBRoleSend uint64
		tmpDBCtrlRecv uint64
		err           error
	)
	// compare and sync to DB, if not same write to database
	dbSync := func() {
		// ---------------------node syncer------------------------
		nsCaches = db.ctx.cache.SelectAllNodeSyncer()
		nsDBs = db.ctx.cache.SelectAllNodeSyncerDB()
		for key, nsCache = range nsCaches {
			same = true
			nsDB = nsDBs[key]
			// maybe lost
			if nsDB == nil {
				continue
			}
			nsCache.RLock()
			nsDB.Lock()
			tmpDBRoleRecv = nsDB.NodeRecv
			if nsDB.NodeRecv != nsCache.NodeRecv {
				nsDB.NodeRecv = nsCache.NodeRecv
				same = false
			}
			tmpDBRoleSend = nsDB.NodeSend
			if nsDB.NodeSend != nsCache.NodeSend {
				nsDB.NodeSend = nsCache.NodeSend
				same = false
			}
			tmpDBCtrlRecv = nsDB.CtrlRecv
			if nsDB.CtrlRecv != nsCache.CtrlRecv {
				nsDB.CtrlRecv = nsCache.CtrlRecv
				same = false
			}
			if !same { // sync to database
				err = db.db.Save(nsDB.mNodeSyncer).Error
				if err != nil {
					// rollback
					nsDB.NodeRecv = tmpDBRoleRecv
					nsDB.NodeSend = tmpDBRoleSend
					nsDB.CtrlRecv = tmpDBCtrlRecv
					nsDB.Unlock()
					nsCache.RUnlock()
					db.log(logger.Error, "cache syncer synchronize failed:", err)
					return
				}
			}
			nsDB.Unlock()
			nsCache.RUnlock()
		}
		// --------------------beacon syncer-----------------------
		bsCaches = db.ctx.cache.SelectAllBeaconSyncer()
		bsDBs = db.ctx.cache.SelectAllBeaconSyncerDB()
		for key, bsCache = range bsCaches {
			same = true
			bsDB = bsDBs[key]
			// maybe lost
			if bsDB == nil {
				continue
			}
			bsCache.RLock()
			bsDB.Lock()
			tmpDBRoleRecv = bsDB.BeaconRecv
			if bsDB.BeaconRecv != bsCache.BeaconRecv {
				bsDB.BeaconRecv = bsCache.BeaconRecv
				same = false
			}
			tmpDBRoleSend = bsDB.BeaconSend
			if bsDB.BeaconSend != bsCache.BeaconSend {
				bsDB.BeaconSend = bsCache.BeaconSend
				same = false
			}
			tmpDBCtrlRecv = bsDB.CtrlRecv
			if bsDB.CtrlRecv != bsCache.CtrlRecv {
				bsDB.CtrlRecv = bsCache.CtrlRecv
				same = false
			}
			if !same { // sync to database
				err = db.db.Save(bsDB.mBeaconSyncer).Error
				if err != nil {
					// rollback
					bsDB.BeaconRecv = tmpDBRoleRecv
					bsDB.BeaconSend = tmpDBRoleSend
					bsDB.CtrlRecv = tmpDBCtrlRecv
					bsDB.Unlock()
					bsCache.RUnlock()
					db.log(logger.Error, "cache syncer synchronize failed:", err)
					return
				}
			}
			bsDB.Unlock()
			bsCache.RUnlock()
		}
	}
	for {
		select {
		case <-ticker.C:
			dbSync()
		case <-db.stopSignal:
			dbSync()
			return
		}
	}
}
