package msfrpc

import (
	"bytes"
	"fmt"
	"sync"

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

	Monitor *MonitorOptions `toml:"monitor"`

	IOManager *IOManagerOptions `toml:"io_manager"`

	Web struct {
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

	once  sync.Once
	errCh chan error
}

// NewMSFRPC is used to create a new msfrpc program.
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

// HijackLogWriter is used to hijack all packages that call functions like log.Println().
func (msfrpc *MSFRPC) HijackLogWriter() {
	logger.HijackLogWriter(logger.Error, "pkg", msfrpc.logger)
}

// Main is used to run msfrpc, it will block until exit or return error.
func (msfrpc *MSFRPC) Main() error {

	msfrpc.monitor.Start()

	// receive error
	msfrpc.errCh = make(chan error, 64)
	var errorList []error
	for err := range msfrpc.errCh {
		errorList = append(errorList, err)
	}
	l := len(errorList)
	if l == 0 {
		return nil
	}
	buf := bytes.NewBuffer(make([]byte, 0, 64*l))
	_, _ = fmt.Fprintln(buf, "receive errors when exit msfrpc:")
	for i := 0; i < len(errorList); i++ {
		_, _ = fmt.Fprintf(buf, "id %d: %s\n", i+1, errorList[i])
	}
	return errors.New(buf.String())
}

// Exit is used to exit msfrpc.
func (msfrpc *MSFRPC) Exit() {
	msfrpc.ExitWithError(nil)
}

// ExitWithError is used to exit msfrpc with an error.
func (msfrpc *MSFRPC) ExitWithError(err error) {
	if err != nil {
		msfrpc.logger.Print(logger.Error, "exit", "exit msfrpc with error:", err)
		msfrpc.sendError(err)
	}
	msfrpc.once.Do(msfrpc.exit)
}

func (msfrpc *MSFRPC) exit() {
	const src = "exit"
	// close web server
	err := msfrpc.web.Close()
	if err != nil {
		msfrpc.logger.Print(logger.Error, src, "appear error when close web server:", err)
		msfrpc.sendError(err)
	}
	msfrpc.logger.Print(logger.Info, src, "web server is closed")
	// close io manager
	err = msfrpc.ioManager.Close()
	if err != nil {
		msfrpc.logger.Print(logger.Error, src, "appear error when close io manager:", err)
		msfrpc.sendError(err)
	}
	msfrpc.logger.Print(logger.Info, src, "io manager is closed")
	// close monitor
	msfrpc.monitor.Close()
	msfrpc.logger.Print(logger.Info, src, "monitor is closed")
	// close client
	err = msfrpc.client.Close()
	if err != nil {
		msfrpc.logger.Print(logger.Error, src, "appear error when close client:", err)
		msfrpc.sendError(err)
	}
	msfrpc.logger.Print(logger.Info, src, "client is closed")
	close(msfrpc.errCh)
}

func (msfrpc *MSFRPC) sendError(err error) {
	select {
	case msfrpc.errCh <- err:
	default:
		msfrpc.logger.Print(logger.Info, "failed to send error to channel\nerror: %s", err)
	}
}
