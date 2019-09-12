package controller

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
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

func (db *db) cacheSyncer() {
	defer db.wg.Done()
}

/*
	node := &mNode{
		GUID: bytes.Repeat([]byte{52}, 52),
	}

	err = db.Model(node).Association("Listeners").Append(&node.Listeners).Error
	require.NoError(t, err)
	err = db.Model(node).Association("Logs").Append(&node.Logs).Error
	require.NoError(t, err)

	// var nodes []*mNode
	// err = db.Model(node).Related(&node.Listeners, "Listeners").Error
	// require.NoError(t, err)

	spew.Dump(node)

	return
*/

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

func (db *db) SelectNode(guid []byte) (*mNode, error) {

	return nil, nil
}

func (db *db) InsertNode(m *mNode) error {
	tx := db.db.BeginTx(context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
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
	tx := db.db.BeginTx(context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
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

func (db *db) SelectBeacon(guid []byte) (*mBeacon, error) {

	return nil, nil
}

// --------------------------------sync message---------------------------------------

// BS = Beacon Syncer , NS = Node Syncer
func (db *db) UpdateBSBeaconReceive(guid []byte, height uint64) error {

	return nil
}

func (db *db) UpdateNSNodeReceive(guid []byte, height uint64) error {

	return nil
}
