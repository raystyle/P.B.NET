package controller

import (
	"bytes"
	"context"
	"database/sql"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/guid"
)

type database struct {
	dbLogger   *dbLogger
	gormLogger *gormLogger
	db         *gorm.DB
	cache      *cache
}

func newDatabase(config *Config) (*database, error) {
	// create database logger
	cfg := config.Database
	dbLogger, err := newDatabaseLogger(cfg.Dialect, cfg.LogFile, cfg.LogWriter)
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
	// table name will not add "s"
	gormDB.SingularTable(true)
	// connection
	gormDB.DB().SetMaxOpenConns(cfg.MaxOpenConns)
	gormDB.DB().SetMaxIdleConns(cfg.MaxIdleConns)
	// gorm logger
	gormLogger, err := newGormLogger(cfg.GORMLogFile, cfg.LogWriter)
	if err != nil {
		return nil, err
	}
	gormDB.SetLogger(gormLogger)
	if cfg.GORMDetailedLog {
		gormDB.LogMode(true)
	}
	return &database{
		dbLogger:   dbLogger,
		gormLogger: gormLogger,
		db:         gormDB,
		cache:      newCache(),
	}, nil
}

func (db *database) Close() {
	_ = db.db.Close()
	db.gormLogger.Close()
	db.dbLogger.Close()
}

type cache struct {
	nodes      map[guid.GUID]*mNode
	nodesRWM   sync.RWMutex
	beacons    map[guid.GUID]*mBeacon
	beaconsRWM sync.RWMutex
}

func newCache() *cache {
	return &cache{
		nodes:   make(map[guid.GUID]*mNode),
		beacons: make(map[guid.GUID]*mBeacon),
	}
}

func (cache *cache) SelectNode(guid *guid.GUID) *mNode {
	cache.nodesRWM.RLock()
	defer cache.nodesRWM.RUnlock()
	return cache.nodes[*guid]
}

func (cache *cache) InsertNode(node *mNode) {
	key := guid.GUID{}
	err := key.Write(node.GUID)
	if err != nil {
		panic("cache internal error: " + err.Error())
	}
	cache.nodesRWM.Lock()
	defer cache.nodesRWM.Unlock()
	if _, ok := cache.nodes[key]; !ok {
		cache.nodes[key] = node
	}
}

func (cache *cache) DeleteNode(guid *guid.GUID) {
	cache.nodesRWM.Lock()
	defer cache.nodesRWM.Unlock()
	delete(cache.nodes, *guid)
}

func (cache *cache) SelectBeacon(guid *guid.GUID) *mBeacon {
	cache.beaconsRWM.RLock()
	defer cache.beaconsRWM.RUnlock()
	return cache.beacons[*guid]
}

func (cache *cache) InsertBeacon(beacon *mBeacon) {
	key := guid.GUID{}
	err := key.Write(beacon.GUID)
	if err != nil {
		panic("cache internal error: " + err.Error())
	}
	cache.beaconsRWM.Lock()
	defer cache.beaconsRWM.Unlock()
	if _, ok := cache.beacons[key]; !ok {
		cache.beacons[key] = beacon
	}
}

func (cache *cache) DeleteBeacon(guid *guid.GUID) {
	cache.beaconsRWM.Lock()
	defer cache.beaconsRWM.Unlock()
	delete(cache.beacons, *guid)
}

func (db *database) InsertCtrlLog(m *mCtrlLog) error {
	return db.db.Table(tableCtrlLog).Create(m).Error
}

// -------------------------------proxy client----------------------------------------

func (db *database) InsertProxyClient(m *mProxyClient) error {
	return db.db.Create(m).Error
}

func (db *database) SelectProxyClient() ([]*mProxyClient, error) {
	var clients []*mProxyClient
	return clients, db.db.Find(&clients).Error
}

func (db *database) UpdateProxyClient(m *mProxyClient) error {
	return db.db.Save(m).Error
}

func (db *database) DeleteProxyClient(id uint64) error {
	return db.db.Delete(&mProxyClient{ID: id}).Error
}

// ---------------------------------dns client----------------------------------------

func (db *database) InsertDNSServer(m *mDNSServer) error {
	return db.db.Create(m).Error
}

func (db *database) SelectDNSServer() ([]*mDNSServer, error) {
	var servers []*mDNSServer
	return servers, db.db.Find(&servers).Error
}

func (db *database) UpdateDNSServer(m *mDNSServer) error {
	return db.db.Save(m).Error
}

func (db *database) DeleteDNSServer(id uint64) error {
	return db.db.Delete(&mDNSServer{ID: id}).Error
}

// -----------------------------time syncer client------------------------------------

func (db *database) InsertTimeSyncerClient(m *mTimeSyncer) error {
	return db.db.Create(m).Error
}

func (db *database) SelectTimeSyncerClient() ([]*mTimeSyncer, error) {
	var timeSyncer []*mTimeSyncer
	return timeSyncer, db.db.Find(&timeSyncer).Error
}

func (db *database) UpdateTimeSyncerClient(m *mTimeSyncer) error {
	return db.db.Save(m).Error
}

func (db *database) DeleteTimeSyncerClient(id uint64) error {
	return db.db.Delete(&mTimeSyncer{ID: id}).Error
}

// -------------------------------------boot------------------------------------------

func (db *database) InsertBoot(m *mBoot) error {
	return db.db.Create(m).Error
}

func (db *database) SelectBoot() ([]*mBoot, error) {
	var boot []*mBoot
	return boot, db.db.Find(&boot).Error
}

func (db *database) UpdateBoot(m *mBoot) error {
	return db.db.Save(m).Error
}

func (db *database) DeleteBoot(id uint64) error {
	return db.db.Delete(&mBoot{ID: id}).Error
}

// ----------------------------------listener-----------------------------------------

func (db *database) InsertListener(m *mListener) error {
	return db.db.Create(m).Error
}

func (db *database) SelectListener() ([]*mListener, error) {
	var listener []*mListener
	return listener, db.db.Find(&listener).Error
}

func (db *database) UpdateListener(m *mListener) error {
	return db.db.Save(m).Error
}

func (db *database) DeleteListener(id uint64) error {
	return db.db.Delete(&mListener{ID: id}).Error
}

// ------------------------------------node-------------------------------------------

func (db *database) SelectNode(guid *guid.GUID) (node *mNode, err error) {
	node = db.cache.SelectNode(guid)
	if node != nil {
		return
	}
	node = new(mNode)
	err = db.db.Find(node, "guid = ?", guid[:]).Error
	if err != nil {
		return nil, err
	}
	db.cache.InsertNode(node)
	return
}

func (db *database) InsertNode(m *mNode) error {
	err := db.db.Create(m).Error
	if err != nil {
		return err
	}
	db.cache.InsertNode(m)
	return nil
}

func (db *database) DeleteNode(guid *guid.GUID) error {
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err := tx.Error
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			db.cache.DeleteNode(guid)
		}
	}()
	err = tx.Delete(&mNode{GUID: guid[:]}).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&mNodeListener{GUID: guid[:]}).Error
	if err != nil {
		return err
	}
	err = tx.Table(tableNodeLog).Delete(&mRoleLog{GUID: guid[:]}).Error
	if err != nil {
		return err
	}
	err = tx.Commit().Error
	return err
}

func (db *database) DeleteNodeUnscoped(guid *guid.GUID) error {
	err := db.db.Unscoped().Delete(&mNode{GUID: guid[:]}).Error
	if err != nil {
		return err
	}
	db.cache.DeleteNode(guid)
	return nil
}

func (db *database) InsertNodeListener(m *mNodeListener) error {
	return db.db.Create(m).Error
}

func (db *database) DeleteNodeListener(id uint64) error {
	return db.db.Delete(&mNodeListener{ID: id}).Error
}

func (db *database) InsertNodeLog(m *mRoleLog) error {
	return db.db.Table(tableNodeLog).Create(m).Error
}

func (db *database) DeleteNodeLog(id uint64) error {
	return db.db.Table(tableNodeLog).Delete(&mRoleLog{ID: id}).Error
}

// -----------------------------------beacon------------------------------------------

func (db *database) SelectBeacon(guid *guid.GUID) (beacon *mBeacon, err error) {
	beacon = db.cache.SelectBeacon(guid)
	if beacon != nil {
		return
	}
	beacon = new(mBeacon)
	err = db.db.Find(beacon, "guid = ?", guid[:]).Error
	if err != nil {
		return nil, err
	}
	db.cache.InsertBeacon(beacon)
	return
}

func (db *database) InsertBeacon(m *mBeacon) error {
	err := db.db.Create(m).Error
	if err != nil {
		return err
	}
	db.cache.InsertBeacon(m)
	return nil
}

func (db *database) DeleteBeacon(guid *guid.GUID) error {
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err := tx.Error
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			db.cache.DeleteBeacon(guid)
		}
	}()
	err = tx.Delete(&mBeacon{GUID: guid[:]}).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&mBeaconListener{GUID: guid[:]}).Error
	if err != nil {
		return err
	}
	err = tx.Table(tableBeaconLog).Delete(&mRoleLog{GUID: guid[:]}).Error
	if err != nil {
		return err
	}
	err = tx.Commit().Error
	return err
}

func (db *database) DeleteBeaconUnscoped(guid *guid.GUID) error {
	err := db.db.Unscoped().Delete(&mBeacon{GUID: guid[:]}).Error
	if err != nil {
		return err
	}
	db.cache.DeleteBeacon(guid)
	return nil
}

// TODO BeaconMessage

func (db *database) InsertBeaconMessage(guid *guid.GUID, message *bytes.Buffer) error {
	return db.db.Create(&mBeaconMessage{GUID: guid[:], Message: message.Bytes()}).Error
}

func (db *database) InsertBeaconListener(m *mBeaconListener) error {
	return db.db.Create(m).Error
}

func (db *database) DeleteBeaconListener(id uint64) error {
	return db.db.Delete(&mBeaconListener{ID: id}).Error
}

func (db *database) InsertBeaconLog(m *mRoleLog) error {
	return db.db.Table(tableBeaconLog).Create(m).Error
}

func (db *database) DeleteBeaconLog(id uint64) error {
	return db.db.Table(tableBeaconLog).Delete(&mRoleLog{ID: id}).Error
}
