package controller

import (
	"context"
	"database/sql"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"project/internal/xreflect"
)

func init() {
	// gorm custom namer: table name delete "m_"
	// table "m_proxy_client" -> "proxy_client"
	default_namer := gorm.TheNamingStrategy.Table
	gorm.TheNamingStrategy.Table = func(name string) string {
		return default_namer(name)[2:]
	}
}

// first use this project
func Init_Database(c *Config) error {
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
			name:  "",
			model: &m_ctrl_log{},
		},
		{
			name:  "",
			model: &m_proxy_client{},
		},
		{
			name:  "",
			model: &m_dns_client{},
		},
		{
			name:  "",
			model: &m_timesync{},
		},
		{
			name:  "",
			model: &m_boot{},
		},
		{
			name:  "",
			model: &m_listener{},
		},
		{
			name:  "",
			model: &m_node{},
		},
		{
			name:  "",
			model: &m_node_listener{},
		},
		{
			name:  "",
			model: &m_node_syncer{},
		},
		{
			name:  t_node_log,
			model: &m_role_log{},
		},
		{
			name:  t_beacon_log,
			model: &m_role_log{},
		},
	}
	for i := 0; i < len(tables); i++ {
		n := tables[i].name
		m := tables[i].model
		if n == "" {
			db.DropTableIfExists(m)
			err = db.CreateTable(m).Error
			if err != nil {
				table := gorm.ToTableName(xreflect.Struct_Name(m))
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
	add_err := func(err error) error {
		return errors.Wrapf(err, "add foreign key failed")
	}
	table := gorm.ToTableName(xreflect.Struct_Name(&m_node{}))
	err = db.Model(&m_node_listener{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return add_err(err)
	}
	err = db.Model(&m_node_syncer{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return add_err(err)
	}
	err = db.Table(t_node_log).Model(&m_role_log{}).AddForeignKey("guid", table+"(guid)",
		"CASCADE", "CASCADE").Error
	if err != nil {
		return add_err(err)
	}
	return nil
}

/*
	node := &m_node{
		GUID: bytes.Repeat([]byte{52}, 52),
	}

	err = db.Model(node).Association("Listeners").Append(&node.Listeners).Error
	require.Nil(t, err, err)
	err = db.Model(node).Association("Logs").Append(&node.Logs).Error
	require.Nil(t, err, err)

	// var nodes []*m_node
	// err = db.Model(node).Related(&node.Listeners, "Listeners").Error
	// require.Nil(t, err, err)

	spew.Dump(node)

	return
*/

// -------------------------------proxy client----------------------------------------

func (this *CTRL) Insert_Proxy_Client(m *m_proxy_client) error {
	return this.db.Create(m).Error
}

func (this *CTRL) Select_Proxy_Client() ([]*m_proxy_client, error) {
	var clients []*m_proxy_client
	return clients, this.db.Find(&clients).Error
}

func (this *CTRL) Update_Proxy_Client(m *m_proxy_client) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_Proxy_Client(id uint64) error {
	return this.db.Delete(&m_proxy_client{ID: id}).Error
}

// -------------------------------dns client----------------------------------------

func (this *CTRL) Insert_DNS_Client(m *m_dns_client) error {
	return this.db.Create(m).Error
}

func (this *CTRL) Select_DNS_Client() ([]*m_dns_client, error) {
	var clients []*m_dns_client
	return clients, this.db.Find(&clients).Error
}

func (this *CTRL) Update_DNS_Client(m *m_dns_client) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_DNS_Client(id uint64) error {
	return this.db.Delete(&m_dns_client{ID: id}).Error
}

// ---------------------------------timesync----------------------------------------

func (this *CTRL) Insert_Timesync(m *m_timesync) error {
	return this.db.Create(m).Error
}

func (this *CTRL) Select_Timesync() ([]*m_timesync, error) {
	var timesync []*m_timesync
	return timesync, this.db.Find(&timesync).Error
}

func (this *CTRL) Update_Timesync(m *m_timesync) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_Timesync(id uint64) error {
	return this.db.Delete(&m_timesync{ID: id}).Error
}

// ---------------------------------bootstrap----------------------------------------

func (this *CTRL) Insert_Boot(m *m_boot) error {
	return this.db.Create(m).Error
}

func (this *CTRL) Select_Boot() ([]*m_boot, error) {
	var boot []*m_boot
	return boot, this.db.Find(&boot).Error
}

func (this *CTRL) Update_Boot(m *m_boot) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_Boot(id uint64) error {
	return this.db.Delete(&m_boot{ID: id}).Error
}

// ----------------------------------listener----------------------------------------

func (this *CTRL) Insert_Listener(m *m_listener) error {
	return this.db.Create(m).Error
}

func (this *CTRL) Select_Listener() ([]*m_listener, error) {
	var listener []*m_listener
	return listener, this.db.Find(&listener).Error
}

func (this *CTRL) Update_Listener(m *m_listener) error {
	return this.db.Save(m).Error
}

func (this *CTRL) Delete_Listener(id uint64) error {
	return this.db.Delete(&m_listener{ID: id}).Error
}

// -------------------------------------node-----------------------------------------

func (this *CTRL) Insert_Node(m *m_node) error {
	tx := this.db.BeginTx(context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if tx.Error != nil {
		return tx.Error
	}
	err := tx.Create(m).Error
	if err != nil {
		return err
	}
	err = tx.Create(&m_node_syncer{GUID: m.GUID}).Error
	if err != nil {
		return err
	}
	err = tx.Commit().Error
	if err != nil {
		return err
	}
	return nil
}

func (this *CTRL) Delete_Node(guid []byte) error {
	tx := this.db.BeginTx(context.Background(),
		&sql.TxOptions{Isolation: sql.LevelSerializable})
	if tx.Error != nil {
		return tx.Error
	}
	err := tx.Delete(&m_node{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Delete(&m_node_listener{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	err = tx.Table(t_node_log).Delete(&m_role_log{}, "guid = ?", guid).Error
	if err != nil {
		return err
	}
	return tx.Commit().Error
}

func (this *CTRL) Delete_Node_Unscoped(guid []byte) error {
	return this.db.Unscoped().Delete(&m_node{}, "guid = ?", guid).Error
}

func (this *CTRL) Insert_Node_Listener(m *m_node_listener) error {
	return this.db.Create(m).Error
}

func (this *CTRL) Delete_Node_Listener(id uint64) error {
	return this.db.Delete(&m_node_listener{ID: id}).Error
}

func (this *CTRL) Insert_Node_Log(m *m_role_log) error {
	return this.db.Table(t_node_log).Create(m).Error
}

func (this *CTRL) Delete_Node_Log(id uint64) error {
	return this.db.Table(t_node_log).Delete(&m_role_log{ID: id}).Error
}
