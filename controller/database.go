package controller

import (
	"context"
	"database/sql"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/xreflect"
)

func init() {
	// gorm custom namer: table name delete "m"
	// table "mProxyClient" -> "m_proxy_client" -> "proxy_client"
	namer := gorm.TheNamingStrategy.Table
	gorm.TheNamingStrategy.Table = func(name string) string {
		return namer(name)[2:]
	}
}

// first use this project
func InitDatabase(c *Config) error {
	// connect database
	db, err := gorm.Open(c.Dialect, c.DSN)
	if err != nil {
		return errors.Wrapf(err, "connect %s server failed", c.Dialect)
	}
	// not add s
	db.SingularTable(true)
	defer func() { _ = db.Close() }()
	tables := []*struct {
		name  string
		model interface{}
	}{
		{
			model: &mCtrlLog{},
		},
		{
			model: &mProxyClient{},
		},
		{
			model: &mDNSServer{},
		},
		{
			model: &mTimeSyncer{},
		},
		{
			model: &mBoot{},
		},
		{
			model: &mListener{},
		},
		{
			model: &mNode{},
		},
		{
			model: &mNodeSyncer{},
		},
		{
			model: &mNodeListener{},
		},
		{
			name:  tableNodeLog,
			model: &mRoleLog{},
		},
		{
			model: &mBeacon{},
		},
		{
			model: &mBeaconSyncer{},
		},
		{
			model: &mBeaconListener{},
		},
		{
			name:  tableBeaconLog,
			model: &mRoleLog{},
		},
	}
	for i := 0; i < len(tables); i++ {
		n := tables[i].name
		m := tables[i].model
		if n == "" {
			db.DropTableIfExists(m)
			err = db.CreateTable(m).Error
			if err != nil {
				table := gorm.ToTableName(xreflect.StructName(m))
				return errors.Wrapf(err, "create table %s failed", table)
			}
		} else {
			db.Table(n).DropTableIfExists(m)
			err = db.Table(n).CreateTable(m).Error
			if err != nil {
				return errors.Wrapf(err, "create table %s failed", n)
			}
		}
	}
	// add foreign key
	addErr := func(err error) error {
		return errors.Wrapf(err, "add foreign key failed")
	}
	table := gorm.ToTableName(xreflect.StructName(&mNode{}))
	err = db.Model(&mNodeListener{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(err)
	}
	err = db.Model(&mNodeSyncer{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(err)
	}
	err = db.Table(tableNodeLog).Model(&mRoleLog{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return addErr(err)
	}
	return nil
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

// -------------------------------proxy client----------------------------------------

func (ctrl *CTRL) InsertProxyClient(m *mProxyClient) error {
	return ctrl.db.Create(m).Error
}

func (ctrl *CTRL) SelectProxyClient() ([]*mProxyClient, error) {
	var clients []*mProxyClient
	return clients, ctrl.db.Find(&clients).Error
}

func (ctrl *CTRL) UpdateProxyClient(m *mProxyClient) error {
	return ctrl.db.Save(m).Error
}

func (ctrl *CTRL) DeleteProxyClient(id uint64) error {
	return ctrl.db.Delete(&mProxyClient{ID: id}).Error
}

// ---------------------------------dns client----------------------------------------

func (ctrl *CTRL) InsertDNSServer(m *mDNSServer) error {
	return ctrl.db.Create(m).Error
}

func (ctrl *CTRL) SelectDNSServer() ([]*mDNSServer, error) {
	var clients []*mDNSServer
	return clients, ctrl.db.Find(&clients).Error
}

func (ctrl *CTRL) UpdateDNSServer(m *mDNSServer) error {
	return ctrl.db.Save(m).Error
}

func (ctrl *CTRL) DeleteDNSServer(id uint64) error {
	return ctrl.db.Delete(&mDNSServer{ID: id}).Error
}

// -----------------------------time syncer config------------------------------------

func (ctrl *CTRL) InsertTimeSyncer(m *mTimeSyncer) error {
	return ctrl.db.Create(m).Error
}

func (ctrl *CTRL) SelectTimeSyncer() ([]*mTimeSyncer, error) {
	var timeSyncer []*mTimeSyncer
	return timeSyncer, ctrl.db.Find(&timeSyncer).Error
}

func (ctrl *CTRL) UpdateTimeSyncer(m *mTimeSyncer) error {
	return ctrl.db.Save(m).Error
}

func (ctrl *CTRL) DeleteTimeSyncer(id uint64) error {
	return ctrl.db.Delete(&mTimeSyncer{ID: id}).Error
}

// -------------------------------------boot------------------------------------------

func (ctrl *CTRL) InsertBoot(m *mBoot) error {
	return ctrl.db.Create(m).Error
}

func (ctrl *CTRL) SelectBoot() ([]*mBoot, error) {
	var boot []*mBoot
	return boot, ctrl.db.Find(&boot).Error
}

func (ctrl *CTRL) UpdateBoot(m *mBoot) error {
	return ctrl.db.Save(m).Error
}

func (ctrl *CTRL) DeleteBoot(id uint64) error {
	return ctrl.db.Delete(&mBoot{ID: id}).Error
}

// ----------------------------------listener-----------------------------------------

func (ctrl *CTRL) InsertListener(m *mListener) error {
	return ctrl.db.Create(m).Error
}

func (ctrl *CTRL) SelectListener() ([]*mListener, error) {
	var listener []*mListener
	return listener, ctrl.db.Find(&listener).Error
}

func (ctrl *CTRL) UpdateListener(m *mListener) error {
	return ctrl.db.Save(m).Error
}

func (ctrl *CTRL) DeleteListener(id uint64) error {
	return ctrl.db.Delete(&mListener{ID: id}).Error
}

// ------------------------------------node-------------------------------------------

func (ctrl *CTRL) InsertNode(m *mNode) error {
	tx := ctrl.db.BeginTx(context.Background(),
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

func (ctrl *CTRL) DeleteNode(guid []byte) error {
	tx := ctrl.db.BeginTx(context.Background(),
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

func (ctrl *CTRL) DeleteNodeUnscoped(guid []byte) error {
	return ctrl.db.Unscoped().Delete(&mNode{}, "guid = ?", guid).Error
}

func (ctrl *CTRL) InsertNodeListener(m *mNodeListener) error {
	return ctrl.db.Create(m).Error
}

func (ctrl *CTRL) DeleteNodeListener(id uint64) error {
	return ctrl.db.Delete(&mNodeListener{ID: id}).Error
}

func (ctrl *CTRL) InsertNodeLog(m *mRoleLog) error {
	return ctrl.db.Table(tableNodeLog).Create(m).Error
}

func (ctrl *CTRL) DeleteNodeLog(id uint64) error {
	return ctrl.db.Table(tableNodeLog).Delete(&mRoleLog{ID: id}).Error
}
