package controller

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/guid"
	"project/internal/logger"
	"project/internal/protocol"
	"project/internal/security"
	"project/internal/xpanic"
)

type database struct {
	ctx *Ctrl

	dbLogger   *dbLogger
	gormLogger *gormLogger
	db         *gorm.DB
	cache      *cache
}

func newDatabase(ctx *Ctrl, config *Config) (*database, error) {
	// create database logger
	cfg := config.Database
	dbLogger, err := newDatabaseLogger(ctx, cfg.Dialect, cfg.LogFile, cfg.LogWriter)
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
	// gorm logger
	gormLogger, err := newGormLogger(ctx, cfg.GORMLogFile, cfg.LogWriter)
	if err != nil {
		return nil, err
	}
	gormDB.SetLogger(gormLogger)
	if cfg.GORMDetailedLog {
		gormDB.LogMode(true)
	}
	// table name will not add "s"
	gormDB.SingularTable(true)
	// set time
	gormDB.SetNowFuncOverride(ctx.global.Now)
	// connection
	gormDB.DB().SetMaxOpenConns(cfg.MaxOpenConns)
	gormDB.DB().SetMaxIdleConns(cfg.MaxIdleConns)
	return &database{
		ctx:        ctx,
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
	db.ctx = nil
	db.db.SetNowFuncOverride(time.Now)
}

func (db *database) logf(lv logger.Level, format string, log ...interface{}) {
	db.ctx.logger.Printf(lv, "database", format, log...)
}

func (db *database) log(lv logger.Level, log ...interface{}) {
	db.ctx.logger.Println(lv, "database", log...)
}

func (db *database) rollback(name string, tx *gorm.DB, err error) bool {
	const format = "failed to rollback in %s: %s"
	if r := recover(); r != nil {
		db.log(logger.Fatal, xpanic.Print(r, fmt.Sprintf("database.%s", name)))
		err := tx.Rollback().Error
		if err != nil {
			db.logf(logger.Fatal, format, name, err)
		}
		return false
	}
	if err != nil {
		err := tx.Rollback().Error
		if err != nil {
			db.log(logger.Error, format, name, err)
		}
		return false
	}
	return true
}

func (db *database) InsertLog(m *mLog) error {
	return db.db.Create(m).Error
}

// ------------------------------------------proxy client------------------------------------------

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
	return db.db.Delete(mProxyClient{ID: id}).Error
}

// -------------------------------------------DNS client-------------------------------------------

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
	return db.db.Delete(mDNSServer{ID: id}).Error
}

// ---------------------------------------time syncer client---------------------------------------

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
	return db.db.Delete(mTimeSyncer{ID: id}).Error
}

// ----------------------------------------------boot----------------------------------------------

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
	return db.db.Delete(mBoot{ID: id}).Error
}

// --------------------------------------------listener--------------------------------------------

func (db *database) InsertListener(m *mListener) error {
	return db.db.Create(m).Error
}

func (db *database) SelectListener() ([]*mListener, error) {
	var listeners []*mListener
	return listeners, db.db.Find(&listeners).Error
}

func (db *database) UpdateListener(m *mListener) error {
	return db.db.Save(m).Error
}

func (db *database) DeleteListener(id uint64) error {
	return db.db.Delete(mListener{ID: id}).Error
}

// ----------------------------------------------zone----------------------------------------------

func (db *database) InsertZone(m *mZone) error {
	return db.db.Create(m).Error
}

func (db *database) SelectZone() ([]*mZone, error) {
	var zones []*mZone
	return zones, db.db.Find(&zones).Error
}

func (db *database) UpdateZone(m *mZone) error {
	return db.db.Save(m).Error
}

func (db *database) DeleteZone(id uint64) error {
	return db.db.Delete(mZone{ID: id}).Error
}

// -------------------------------------------about Node-------------------------------------------

func (db *database) SelectNode(guid *guid.GUID) (*mNode, error) {
	node := db.cache.SelectNode(guid)
	if node != nil {
		return node, nil
	}
	node = new(mNode)
	err := db.db.Find(node, "guid = ?", guid[:]).Error
	if err != nil {
		return nil, err
	}
	// calculate session key
	sessionKey, err := db.ctx.global.KeyExchange(node.KexPublicKey)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to calculate node session key")
	}
	defer security.CoverBytes(sessionKey)
	node.SessionKey = security.NewBytes(sessionKey)
	db.cache.InsertNode(node)
	return node, nil
}

func (db *database) InsertNode(node *mNode, info *mNodeInfo) (err error) {
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err = tx.Error
	if err != nil {
		return
	}
	defer func() {
		if db.rollback("InsertNode", tx, err) {
			db.cache.InsertNode(node)
		}
	}()
	err = tx.Create(node).Error
	if err != nil {
		return
	}
	err = tx.Create(info).Error
	if err != nil {
		return
	}
	err = tx.Commit().Error
	return
}

func (db *database) DeleteNode(guid *guid.GUID) (err error) {
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err = tx.Error
	if err != nil {
		return
	}
	defer func() {
		if db.rollback("DeleteNode", tx, err) {
			db.cache.DeleteNode(guid)
		}
	}()
	const where = "guid = ?"
	err = tx.Delete(mNode{}, where, guid[:]).Error
	if err != nil {
		return
	}
	err = tx.Delete(mNodeListener{}, where, guid[:]).Error
	if err != nil {
		return
	}
	err = tx.Table(tableNodeLog).Delete(mRoleLog{}, where, guid[:]).Error
	if err != nil {
		return
	}
	err = tx.Commit().Error
	return
}

func (db *database) DeleteNodeUnscoped(guid *guid.GUID) error {
	err := db.db.Unscoped().Delete(mNode{}, "guid = ?", guid[:]).Error
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
	return db.db.Delete(mNodeListener{ID: id}).Error
}

func (db *database) InsertNodeLog(m *mRoleLog) error {
	return db.db.Table(tableNodeLog).Create(m).Error
}

func (db *database) DeleteNodeLog(id uint64) error {
	return db.db.Table(tableNodeLog).Delete(mRoleLog{ID: id}).Error
}

// ------------------------------------------about Beacon------------------------------------------

func (db *database) SelectBeacon(guid *guid.GUID) (*mBeacon, error) {
	beacon := db.cache.SelectBeacon(guid)
	if beacon != nil {
		return beacon, nil
	}
	beacon = new(mBeacon)
	err := db.db.Find(beacon, "guid = ?", guid[:]).Error
	if err != nil {
		return nil, err
	}
	// calculate session key
	sessionKey, err := db.ctx.global.KeyExchange(beacon.KexPublicKey)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to calculate beacon session key")
	}
	defer security.CoverBytes(sessionKey)
	beacon.SessionKey = security.NewBytes(sessionKey)
	db.cache.InsertBeacon(beacon)
	return beacon, nil
}

func (db *database) InsertBeacon(beacon *mBeacon, info *mBeaconInfo) (err error) {
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err = tx.Error
	if err != nil {
		return
	}
	defer func() {
		if db.rollback("InsertBeacon", tx, err) {
			db.cache.InsertBeacon(beacon)
		}
	}()
	err = tx.Create(beacon).Error
	if err != nil {
		return
	}
	err = tx.Create(info).Error
	if err != nil {
		return
	}
	err = tx.Create(&mBeaconMessageIndex{GUID: beacon.GUID}).Error
	if err != nil {
		return
	}
	err = tx.Commit().Error
	return
}

func (db *database) DeleteBeacon(guid *guid.GUID) (err error) {
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err = tx.Error
	if err != nil {
		return
	}
	defer func() {
		if db.rollback("DeleteBeacon", tx, err) {
			db.cache.DeleteBeacon(guid)
		}
	}()
	const where = "guid = ?"
	err = tx.Delete(mBeacon{}, where, guid[:]).Error
	if err != nil {
		return
	}
	err = tx.Delete(mBeaconMessage{}, where, guid[:]).Error
	if err != nil {
		return
	}
	err = tx.Delete(mBeaconMessageIndex{}, where, guid[:]).Error
	if err != nil {
		return
	}
	err = tx.Delete(mBeaconListener{}, where, guid[:]).Error
	if err != nil {
		return
	}
	err = tx.Table(tableBeaconLog).Delete(mRoleLog{}, where, guid[:]).Error
	if err != nil {
		return
	}
	err = tx.Commit().Error
	return
}

func (db *database) DeleteBeaconUnscoped(guid *guid.GUID) error {
	err := db.db.Unscoped().Delete(mBeacon{}, "guid = ?", guid[:]).Error
	if err != nil {
		return err
	}
	db.cache.DeleteBeacon(guid)
	return nil
}

func (db *database) InsertBeaconMessage(guid *guid.GUID, send *protocol.Send) (err error) {
	// select message index
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err = tx.Error
	if err != nil {
		return
	}
	defer db.rollback("InsertBeaconMessage", tx, err)
	index := mBeaconMessageIndex{}
	err = tx.Set("gorm:query_option", "FOR UPDATE").
		Find(&index, "guid = ?", guid[:]).Error
	if err != nil {
		return
	}
	err = tx.Create(&mBeaconMessage{
		GUID:    guid[:],
		Index:   index.Index,
		Hash:    send.Hash,
		Deflate: send.Deflate,
		Message: send.Message,
	}).Error
	if err != nil {
		return
	}
	// self add one
	err = tx.Model(index).UpdateColumn("index", index.Index+1).Error
	if err != nil {
		return
	}
	err = tx.Commit().Error
	return
}

func (db *database) DeleteBeaconMessagesWithIndex(guid *guid.GUID, index uint64) error {
	return db.db.Delete(mBeaconMessage{}, "guid = ? and `index` < ?", guid[:], index).Error
}

func (db *database) SelectBeaconMessage(guid *guid.GUID, index uint64) (*mBeaconMessage, error) {
	msg := new(mBeaconMessage)
	err := db.db.Find(msg, "guid = ? and `index` = ?", guid[:], index).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return msg, nil
}

func (db *database) InsertBeaconListener(m *mBeaconListener) error {
	return db.db.Create(m).Error
}

func (db *database) DeleteBeaconListener(id uint64) error {
	return db.db.Delete(mBeaconListener{ID: id}).Error
}

func (db *database) InsertBeaconLog(m *mRoleLog) error {
	return db.db.Table(tableBeaconLog).Create(m).Error
}

func (db *database) DeleteBeaconLog(id uint64) error {
	return db.db.Table(tableBeaconLog).Delete(mRoleLog{ID: id}).Error
}
