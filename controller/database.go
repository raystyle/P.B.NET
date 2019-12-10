package controller

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/guid"
)

type db struct {
	dbLogger   *dbLogger
	gormLogger *gormLogger
	db         *gorm.DB
	cache      *cache
}

func newDB(config *Config) (*db, error) {
	// set db logger
	cfg := config.Database
	dbLogger, err := newDBLogger(cfg.Dialect, cfg.LogFile)
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
		return nil, errors.Wrapf(err, "failed to connect %s server", cfg.Dialect)
	}
	err = gormDB.DB().Ping()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to ping %s server", cfg.Dialect)
	}
	gormDB.SingularTable(true) // not add s
	// connection
	gormDB.DB().SetMaxOpenConns(cfg.MaxOpenConns)
	gormDB.DB().SetMaxIdleConns(cfg.MaxIdleConns)
	// gorm logger
	gormLogger, err := newGormLogger(cfg.GORMLogFile)
	if err != nil {
		return nil, err
	}
	gormDB.SetLogger(gormLogger)
	if cfg.GORMDetailedLog {
		gormDB.LogMode(true)
	}
	return &db{
		dbLogger:   dbLogger,
		gormLogger: gormLogger,
		db:         gormDB,
		cache:      newCache(),
	}, nil
}

func (db *db) Close() {
	_ = db.db.Close()
	db.gormLogger.Close()
	db.dbLogger.Close()
}

// key = hex(guid) lower
type cache struct {
	nodes      map[string]*mNode
	nodesRWM   sync.RWMutex
	beacons    map[string]*mBeacon
	beaconsRWM sync.RWMutex

	// calculate key
	hexPool sync.Pool
}

func newCache() *cache {
	c := cache{
		nodes:   make(map[string]*mNode),
		beacons: make(map[string]*mBeacon),
	}
	c.hexPool.New = func() interface{} {
		return make([]byte, 2*guid.Size)
	}
	return &c
}

func (cache *cache) calculateKey(guid []byte) string {
	dst := cache.hexPool.Get().([]byte)
	defer cache.hexPool.Put(dst)
	hex.Encode(dst, guid)
	return string(dst)
}

func (cache *cache) SelectNode(guid []byte) *mNode {
	key := cache.calculateKey(guid)
	cache.nodesRWM.RLock()
	defer cache.nodesRWM.RUnlock()
	return cache.nodes[key]
}

func (cache *cache) InsertNode(node *mNode) {
	key := hex.EncodeToString(node.GUID) // not use hexPool
	cache.nodesRWM.Lock()
	defer cache.nodesRWM.Unlock()
	if _, ok := cache.nodes[key]; !ok {
		cache.nodes[key] = node
	}
}

func (cache *cache) DeleteNode(guid string) {
	cache.nodesRWM.Lock()
	defer cache.nodesRWM.Unlock()
	delete(cache.nodes, guid)
}

func (cache *cache) SelectBeacon(guid []byte) *mBeacon {
	key := cache.calculateKey(guid)
	cache.beaconsRWM.RLock()
	defer cache.beaconsRWM.RUnlock()
	return cache.beacons[key]
}

func (cache *cache) InsertBeacon(beacon *mBeacon) {
	key := hex.EncodeToString(beacon.GUID) // not use hexPool
	cache.beaconsRWM.Lock()
	defer cache.beaconsRWM.Unlock()
	if _, ok := cache.beacons[key]; !ok {
		cache.beacons[key] = beacon
	}
}

func (cache *cache) DeleteBeacon(guid string) {
	cache.beaconsRWM.Lock()
	defer cache.beaconsRWM.Unlock()
	delete(cache.beacons, guid)
}

func (db *db) InsertCtrlLog(m *mCtrlLog) error {
	return db.db.Table(tableCtrlLog).Create(m).Error
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
	var servers []*mDNSServer
	return servers, db.db.Find(&servers).Error
}

func (db *db) UpdateDNSServer(m *mDNSServer) error {
	return db.db.Save(m).Error
}

func (db *db) DeleteDNSServer(id uint64) error {
	return db.db.Delete(&mDNSServer{ID: id}).Error
}

// -----------------------------time syncer client------------------------------------

func (db *db) InsertTimeSyncerClient(m *mTimeSyncer) error {
	return db.db.Create(m).Error
}

func (db *db) SelectTimeSyncerClient() ([]*mTimeSyncer, error) {
	var timeSyncer []*mTimeSyncer
	return timeSyncer, db.db.Find(&timeSyncer).Error
}

func (db *db) UpdateTimeSyncerClient(m *mTimeSyncer) error {
	return db.db.Save(m).Error
}

func (db *db) DeleteTimeSyncerClient(id uint64) error {
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
	node = db.cache.SelectNode(guid)
	if node != nil {
		return
	}
	node = new(mNode)
	err = db.db.Find(node, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.cache.InsertNode(node)
	return
}

func (db *db) InsertNode(m *mNode) error {
	err := db.db.Create(m).Error
	if err != nil {
		return err
	}
	db.cache.InsertNode(m)
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
		} else {
			db.cache.DeleteNode(base64.StdEncoding.EncodeToString(guid))
		}
	}()
	err = tx.Delete(&mNode{GUID: guid}).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&mNodeListener{GUID: guid}).Error
	if err != nil {
		return err
	}
	err = tx.Table(tableNodeLog).Delete(&mRoleLog{GUID: guid}).Error
	if err != nil {
		return err
	}
	err = tx.Commit().Error
	return err
}

func (db *db) DeleteNodeUnscoped(guid []byte) error {
	err := db.db.Unscoped().Delete(&mNode{GUID: guid}).Error
	if err != nil {
		return err
	}
	db.cache.DeleteNode(base64.StdEncoding.EncodeToString(guid))
	return nil
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
	beacon = db.cache.SelectBeacon(guid)
	if beacon != nil {
		return
	}
	beacon = new(mBeacon)
	err = db.db.Find(beacon, "guid = ?", guid).Error
	if err != nil {
		return nil, err
	}
	db.cache.InsertBeacon(beacon)
	return
}

func (db *db) InsertBeacon(m *mBeacon) error {
	err := db.db.Create(m).Error
	if err != nil {
		return err
	}
	db.cache.InsertBeacon(m)
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
		} else {
			db.cache.DeleteBeacon(base64.StdEncoding.EncodeToString(guid))
		}
	}()
	err = tx.Delete(&mBeacon{GUID: guid}).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&mBeaconListener{GUID: guid}).Error
	if err != nil {
		return err
	}
	err = tx.Table(tableBeaconLog).Delete(&mRoleLog{GUID: guid}).Error
	if err != nil {
		return err
	}
	err = tx.Commit().Error
	return err
}

func (db *db) DeleteBeaconUnscoped(guid []byte) error {
	err := db.db.Unscoped().Delete(&mBeacon{GUID: guid}).Error
	if err != nil {
		return err
	}
	db.cache.DeleteBeacon(base64.StdEncoding.EncodeToString(guid))
	return nil
}

// TODO BeaconMessage

func (db *db) InsertBeaconMessage(guid, message []byte) error {
	return db.db.Create(&mBeaconMessage{GUID: guid, Message: message}).Error
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
