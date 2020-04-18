package msfrpc

import (
	"time"
)

// WebOptions contains options about web server.
type WebOptions struct {
	CertFile string        `toml:"cert_file"`
	KeyFile  string        `toml:"key_file"`
	Timeout  time.Duration `toml:"timeout"`
	MaxConns int           `toml:"max_conns"`
	Username string        `toml:"username"`
	Password string        `toml:"password"`
	// about interval
	IOInterval      time.Duration `toml:"io_interval"`
	MonitorInterval time.Duration `toml:"monitor_interval"`
}

// WebServer is provide a web UI.
type WebServer struct {
}

// NewWebServer is used to create a web server.
func (msf *MSFRPC) NewWebServer() {

}
