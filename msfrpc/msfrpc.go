package msfrpc

import (
	"github.com/pkg/errors"

	"project/internal/logger"
)

// Config contains all configurations about MSFRPC.
type Config struct {
	Logger logger.Logger `toml:"-" msgpack:"-"`
	Client struct {
		Address  string         `toml:"address"`
		Username string         `toml:"username"`
		Password string         `toml:"password"`
		Options  *ClientOptions `toml:"options"`
	} `toml:"client"`
	Monitor   *MonitorOptions   `toml:"monitor"`
	IOManager *IOManagerOptions `toml:"io_manager"`
	Web       struct {
		Network string      `toml:"network"`
		Address string      `toml:"address"`
		Options *WebOptions `toml:"options"`
	} `toml:"web"`
}

// MSFRPC is single program that include Client, Monitor, IO Manager and Web UI.
type MSFRPC struct {
	logger    logger.Logger
	client    *Client
	monitor   *Monitor
	ioManager *IOManager
	web       *Web
}

// NewMSFRPC is used to create msfrpc program.
func NewMSFRPC(cfg *Config) (*MSFRPC, error) {
	msfrpc := new(MSFRPC)

	address := cfg.Client.Address
	username := cfg.Client.Username
	password := cfg.Client.Password
	options := cfg.Client.Options
	client, err := NewClient(address, username, password, cfg.Logger, options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client")
	}
	web, err := NewWeb(msfrpc, cfg.Web.Options)
	if err != nil {
		return nil, err
	}
	msfrpc.monitor = NewMonitor(client, web.MonitorCallbacks(), cfg.Monitor)
	msfrpc.ioManager = NewIOManager(client, web.IOEventHandlers(), cfg.IOManager)
	return msfrpc, nil
}

// HijackLogWriter is used to hijack all packages that use log.Print().
func (msfrpc *MSFRPC) HijackLogWriter() {
	logger.HijackLogWriter(logger.Error, "pkg", msfrpc.logger)
}

// Main is used to run msfrpc, it will block until exit or return error.
func (msfrpc *MSFRPC) Main() error {
	return nil
}

// Exit is used to exit msfrpc with an error.
func (msfrpc *MSFRPC) Exit(err error) error {
	return nil
}
