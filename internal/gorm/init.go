package gorm

import (
	"github.com/jinzhu/gorm"
)

func init() {
	// gorm custom namer: table name delete "m"
	// table "mProxyClient" -> "m_proxy_client" -> "proxy_client"
	namer := gorm.TheNamingStrategy.Table
	gorm.TheNamingStrategy.Table = func(name string) string {
		return namer(name)[2:]
	}
}
