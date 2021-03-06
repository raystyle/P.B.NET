package controller

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/crypto/aes"
	"project/internal/crypto/hmac"
	"project/internal/guid"
	"project/internal/logger"
	"project/internal/messages"
	"project/internal/protocol"
	"project/internal/random"
	"project/internal/security"
	"project/internal/xpanic"
)

type database struct {
	ctx *Ctrl

	dbLogger   *dbLogger
	gormLogger *gormLogger

	db    *gorm.DB
	cache *cache

	// for replace beacon message
	rand *random.Rand
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
		rand:       random.NewRand(),
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

// commit is used to commit and  rollback if err != nil,
// if return true, it means commit is success fully.
func (db *database) commit(name string, tx *gorm.DB, err error) error {
	const rollback = "failed to rollback in %s: %s"
	if r := recover(); r != nil {
		title := fmt.Sprintf("database.%s", name)
		db.log(logger.Fatal, xpanic.Print(r, title))
		// when panic occurred, err maybe nil
		e := tx.Rollback().Error
		if e != nil {
			db.logf(logger.Fatal, rollback, name, e)
		}
		return errors.WithStack(err)
	}
	if err != nil {
		e := tx.Rollback().Error
		if e != nil {
			db.log(logger.Error, rollback, name, e)
		}
		return errors.WithStack(err)
	}
	return tx.Commit().Error
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
	return db.db.Delete(&mProxyClient{ID: id}).Error
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
	return db.db.Delete(&mDNSServer{ID: id}).Error
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
	return db.db.Delete(&mTimeSyncer{ID: id}).Error
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
	return db.db.Delete(&mBoot{ID: id}).Error
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
	return db.db.Delete(&mListener{ID: id}).Error
}

// ----------------------------------------------zone----------------------------------------------

func (db *database) InsertZone(name string) error {
	if name == "" {
		return errors.New("empty zone name")
	}
	return db.db.Create(&mZone{Name: name}).Error
}

func (db *database) SelectZone() ([]*mZone, error) {
	var zones []*mZone
	return zones, db.db.Find(&zones).Error
}

func (db *database) UpdateZone(m *mZone) error {
	return db.db.Save(m).Error
}

func (db *database) DeleteZone(m *mZone) (err error) {
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err = tx.Error
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		err = db.commit("DeleteZone", tx, err)
	}()
	err = db.db.Delete(&mZone{ID: m.ID}).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			err = fmt.Errorf("zone %s is not exist", m.Name)
		}
		return
	}
	return
}

// -------------------------------------------about Node-------------------------------------------

func (db *database) SelectNode(guid *guid.GUID) (*mNode, error) {
	node := db.cache.SelectNode(guid)
	if node != nil {
		return node, nil
	}
	node = new(mNode)
	g := guid[:]
	err := db.db.Find(node, "guid = ?", g).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			err = errors.Errorf("node %s is not exist", guid.Hex())
		}
		return nil, err
	}
	// calculate session key
	sessionKey, err := db.ctx.global.KeyExchange(node.KexPublicKey)
	if err != nil {
		const format = "failed to calculate node %X session key: %s"
		return nil, errors.Errorf(format, g, err)
	}
	defer security.CoverBytes(sessionKey)
	secSessionKey := security.NewBytes(sessionKey)
	node.SessionKey = secSessionKey
	node.HMACPool.New = func() interface{} {
		key := secSessionKey.Get()
		defer secSessionKey.Put(key)
		return hmac.New(sha256.New, key)
	}
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
		return errors.WithStack(err)
	}
	defer func() {
		err = db.commit("InsertNode", tx, err)
		if err == nil {
			db.cache.InsertNode(node)
		}
	}()
	// check zone is exists
	if info.Zone != "" {
		zone := mZone{}
		err = tx.Set("gorm:query_option", "FOR UPDATE").
			Find(&zone, "name = ?", info.Zone).Error
		if err != nil {
			if gorm.IsRecordNotFoundError(err) {
				err = fmt.Errorf("zone %s is not exist", info.Zone)
			}
			return
		}
	}
	for _, model := range [...]interface{}{
		node,
		info,
	} {
		err = tx.Create(model).Error
		if err != nil {
			return
		}
	}
	return
}

func (db *database) DeleteNode(guid *guid.GUID) (err error) {
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err = tx.Error
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		err = db.commit("DeleteNode", tx, err)
		if err == nil {
			db.cache.DeleteNode(guid)
		}
	}()
	const where = "guid = ?"
	g := guid[:]
	for _, model := range [...]interface{}{
		&mNode{},
		&mNodeInfo{},
		&mNodeListener{},
	} {
		err = tx.Delete(model, where, g).Error
		if err != nil {
			return
		}
	}
	return tx.Table(tableNodeLog).Delete(&mRoleLog{}, where, g).Error
}

func (db *database) DeleteNodeUnscoped(guid *guid.GUID) error {
	err := db.db.Unscoped().Delete(&mNode{}, "guid = ?", guid[:]).Error
	if err != nil {
		return err
	}
	db.cache.DeleteNode(guid)
	return nil
}

func (db *database) SelectNodeListener(guid *guid.GUID) ([]*mNodeListener, error) {
	var listeners []*mNodeListener
	g := guid[:]
	err := db.db.Find(&listeners, "guid = ?", g).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			err = fmt.Errorf("node %s is not exist", guid.Hex())
		}
		return nil, err
	}
	return listeners, nil
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

// ------------------------------------------about Beacon------------------------------------------

func (db *database) SelectBeacon(guid *guid.GUID) (*mBeacon, error) {
	beacon := db.cache.SelectBeacon(guid)
	if beacon != nil {
		return beacon, nil
	}
	beacon = new(mBeacon)
	err := db.db.Find(beacon, "guid = ?", guid[:]).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			err = errors.Errorf("beacon %s is not exist", guid.Hex())
		}
		return nil, err
	}
	// calculate session key
	sessionKey, err := db.ctx.global.KeyExchange(beacon.KexPublicKey)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to calculate beacon session key")
	}
	defer security.CoverBytes(sessionKey)
	secSessionKey := security.NewBytes(sessionKey)
	beacon.SessionKey = secSessionKey
	beacon.HMACPool.New = func() interface{} {
		key := secSessionKey.Get()
		defer secSessionKey.Put(key)
		return hmac.New(sha256.New, key)
	}
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
		return errors.WithStack(err)
	}
	defer func() {
		err = db.commit("InsertBeacon", tx, err)
		if err == nil {
			db.cache.InsertBeacon(beacon)
		}
	}()
	// check sleep range
	if time.Duration(info.SleepFixed+info.SleepRandom)*time.Second >= random.MaxSleepTime {
		return errors.Errorf("fixed + random >= %s", random.MaxSleepTime)
	}
	// insert
	for _, model := range [...]interface{}{
		beacon,
		info,
		&mBeaconMessageIndex{GUID: beacon.GUID},
	} {
		err = tx.Create(model).Error
		if err != nil {
			return
		}
	}
	return
}

func (db *database) DeleteBeacon(guid *guid.GUID) (err error) {
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err = tx.Error
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		err = db.commit("DeleteBeacon", tx, err)
		if err == nil {
			db.cache.DeleteBeacon(guid)
		}
	}()
	const where = "guid = ?"
	g := guid[:]
	for _, model := range [...]interface{}{
		&mBeacon{},
		&mBeaconInfo{},
		&mBeaconListener{},
		&mBeaconMessage{},
		&mBeaconMessageIndex{},
		&mBeaconModeChanged{},
	} {
		err = tx.Delete(model, where, g).Error
		if err != nil {
			return
		}
	}
	return tx.Table(tableBeaconLog).Delete(&mRoleLog{}, where, g).Error
}

func (db *database) DeleteBeaconUnscoped(guid *guid.GUID) error {
	err := db.db.Unscoped().Delete(&mBeacon{}, "guid = ?", guid[:]).Error
	if err != nil {
		return err
	}
	db.cache.DeleteBeacon(guid)
	return nil
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

func (db *database) InsertBeaconMessage(send *protocol.Send) (err error) {
	// select message index
	tx := db.db.BeginTx(
		context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable},
	)
	err = tx.Error
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		err = db.commit("InsertBeaconMessage", tx, err)
	}()
	index := mBeaconMessageIndex{}
	err = tx.Set("gorm:query_option", "FOR UPDATE").
		Find(&index, "guid = ?", send.RoleGUID[:]).Error
	if err != nil {
		return
	}
	message := mBeaconMessage{
		GUID:    send.RoleGUID[:],
		Index:   index.Index,
		Deflate: send.Deflate,
		Message: send.Message,
	}
	err = tx.Create(&message).Error
	if err != nil {
		return
	}
	// self add one
	return tx.Model(index).Update("index", index.Index+1).Error
}

func (db *database) DeleteBeaconMessage(query *protocol.Query) error {
	const where = "guid = ? and `index` < ?"
	message := mBeaconMessage{}
	return db.db.Delete(&message, where, query.BeaconGUID[:], query.Index).Error
}

func (db *database) SelectBeaconMessage(query *protocol.Query) (*mBeaconMessage, error) {
	const where = "guid = ? and `index` = ?"
	msg := new(mBeaconMessage)
	err := db.db.Find(msg, where, query.BeaconGUID[:], query.Index).Error
	if err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}
	return msg, nil
}

// ListBeaconMessage will select all Beacon message and decrypt it.
// User can query Beacon's current message that will be queried,
// then they can cancel some message.
func (db *database) ListBeaconMessage(guid *guid.GUID) ([]*mBeaconMessage, error) {
	// get session key
	beacon, err := db.SelectBeacon(guid)
	if err != nil {
		return nil, err
	}
	const (
		columns = "index, deflate, message, created_at"
		where   = "guid = ?"
	)
	var bms []*mBeaconMessage
	err = db.db.Select(columns).Find(&bms, where, guid[:]).Error
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// decrypt message
	sessionKey := beacon.SessionKey.Get()
	defer beacon.SessionKey.Put(sessionKey)
	aesKey := sessionKey
	aesIV := sessionKey[:aes.IVSize]
	buffer := bytes.Buffer{}
	bytesReader := bytes.NewReader(nil)
	deflateReader := flate.NewReader(bytesReader)
	for i := 0; i < len(bms); i++ {
		bms[i].Message, err = aes.CBCDecrypt(bms[i].Message, aesKey, aesIV)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		// may be need decompress
		if bms[i].Deflate != 1 {
			continue
		}
		bytesReader.Reset(bms[i].Message)
		err = deflateReader.(flate.Resetter).Reset(bytesReader, nil)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		buffer.Reset()
		_, err = buffer.ReadFrom(deflateReader)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		err = deflateReader.Close()
		if err != nil {
			return nil, errors.WithStack(err)
		}
		// copy
		b := make([]byte, buffer.Len())
		copy(b, buffer.Bytes())
		bms[i].Message = b
	}
	return bms, nil
}

// CancelBeaconMessage is used to replace raw message to Nop command.
// prevent incorrect message index.
func (db *database) CancelBeaconMessage(guid *guid.GUID, index uint64) (err error) {
	// get session key
	beacon, err := db.SelectBeacon(guid)
	if err != nil {
		return
	}
	// make nop command
	sessionKey := beacon.SessionKey.Get()
	defer beacon.SessionKey.Put(sessionKey)
	aesKey := sessionKey
	aesIV := sessionKey[:aes.IVSize]
	msg := make([]byte, messages.RandomDataSize+messages.MessageTypeSize)
	copy(msg, db.rand.Bytes(messages.RandomDataSize))
	copy(msg[messages.RandomDataSize:], messages.CMDBCtrlBeaconNop)
	msg, err = aes.CBCEncrypt(msg, aesKey, aesIV)
	if err != nil {
		return errors.WithStack(err)
	}
	// replace ole message to nop
	bm := &mBeaconMessage{
		Deflate: 0, // deflate = false
		Message: msg,
	}
	const where = "guid = ? and `index` = ?"
	err = db.db.Model(bm).Where(where, guid[:], index).Updates(bm).Error
	if err != nil {
		return errors.WithStack(err)
	}
	return
}

func (db *database) SelectBeaconSleepTime(guid *guid.GUID) (uint, uint, error) {
	const (
		columns = "sleep_fixed, sleep_random"
		where   = "guid = ?"
	)
	info := mBeaconInfo{}
	err := db.db.Select(columns).Find(&info, where, guid[:]).Error
	if err != nil {
		return 0, 0, err
	}
	// check range
	if time.Duration(info.SleepFixed+info.SleepRandom)*time.Second >= random.MaxSleepTime {
		return 0, 0, errors.Errorf("fixed + random >= %s", random.MaxSleepTime)
	}
	return info.SleepFixed, info.SleepRandom, nil
}

func (db *database) UpdateBeaconSleepTime(guid *guid.GUID, fixed, rand uint) error {
	// check range
	if time.Duration(fixed+rand)*time.Second >= random.MaxSleepTime {
		return errors.Errorf("fixed + random >= %s", random.MaxSleepTime)
	}
	info := &mBeaconInfo{
		SleepFixed:  fixed,
		SleepRandom: rand,
	}
	return db.db.Model(info).Where("guid = ?", guid[:]).Updates(info).Error
}

func (db *database) InsertBeaconModeChanged(guid *guid.GUID, mc *messages.ModeChanged) error {
	bmc := mBeaconModeChanged{
		GUID:        guid[:],
		Interactive: mc.Interactive,
		Reason:      mc.Reason,
	}
	return db.db.Create(&bmc).Error
}

// ------------------------------------------about Module------------------------------------------

func (db *database) InsertShellCodeResult(guid *guid.GUID, error string) error {
	sc := mModuleShellCode{
		GUID:  guid[:],
		Error: error,
	}
	return db.db.Create(&sc).Error
}

func (db *database) InsertSingleShellOutput(guid *guid.GUID, sso *messages.SingleShellOutput) error {
	ss := mModuleSingleShell{
		GUID:   guid[:],
		Output: sso.Output,
		Error:  sso.Err,
	}
	return db.db.Create(&ss).Error
}
