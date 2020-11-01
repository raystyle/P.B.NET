package msfrpc

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/nettool"
	"project/internal/xpanic"
)

// Config contains all configurations about MSFRPC.
type Config struct {
	Logger logger.Logger

	Client struct {
		Address  string
		Username string
		Password string
		Options  *ClientOptions
	}

	Monitor *MonitorOptions

	IOManager *IOManagerOptions

	Web struct {
		Network string
		Address string
		Options *WebOptions
	}
}

// MSFRPC is a single program that contains Client, Monitor, IO Manager and Web.
type MSFRPC struct {
	logger    logger.Logger
	client    *Client
	monitor   *Monitor
	ioManager *IOManager
	web       *Web

	// for database
	database *DBConnectOptions
	// for web server
	listener net.Listener

	// wait and exit
	once  sync.Once
	wait  chan struct{}
	errCh chan error
}

// NewMSFRPC is used to create a new msfrpc program.
func NewMSFRPC(cfg *Config) (*MSFRPC, error) {
	msfrpc := &MSFRPC{logger: cfg.Logger}
	address := cfg.Client.Address
	username := cfg.Client.Username
	password := cfg.Client.Password
	options := cfg.Client.Options
	client, err := NewClient(address, username, password, cfg.Logger, options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client")
	}
	msfrpc.client = client
	// copy database configuration
	if cfg.Monitor.EnableDB && cfg.Monitor.Database != nil {
		msfrpc.database = cfg.Monitor.Database
	}
	web, err := NewWeb(msfrpc, cfg.Web.Options)
	if err != nil {
		return nil, err
	}
	if cfg.Web.Address != "" {
		err := nettool.IsTCPNetwork(cfg.Web.Network)
		if err != nil {
			return nil, err
		}
		listener, err := net.Listen(cfg.Web.Network, cfg.Web.Address)
		if err != nil {
			return nil, err
		}
		msfrpc.listener = listener
	}
	msfrpc.web = web
	msfrpc.monitor = NewMonitor(client, web.MonitorCallbacks(), cfg.Monitor)
	msfrpc.ioManager = NewIOManager(client, web.IOEventHandlers(), cfg.IOManager)
	// wait and exit
	msfrpc.wait = make(chan struct{}, 2)
	msfrpc.errCh = make(chan error, 64)
	return msfrpc, nil
}

// Main is used to run msfrpc, it will block until exit or return error.
func (msfrpc *MSFRPC) Main() error {
	const src = "main"
	defer func() { msfrpc.wait <- struct{}{} }()
	// logon to msfrpcd
	token := msfrpc.client.GetToken()
	if token == "" {
		err := msfrpc.client.AuthLogin()
		if err != nil {
			return errors.WithMessage(err, "failed to connect msfrpcd")
		}
	}
	// connect database
	if msfrpc.database.Driver != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := msfrpc.client.DBConnect(ctx, msfrpc.database)
		if err != nil {
			return errors.WithMessage(err, "failed to connect database")
		}
	}
	// start web server
	if msfrpc.listener != nil {
		errCh := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					buf := xpanic.Print(r, "MSFRPC.Main")
					msfrpc.logger.Print(logger.Fatal, src, buf)
				}
			}()
			errCh <- msfrpc.web.Serve(msfrpc.listener)
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := nettool.WaitServerServe(ctx, errCh, msfrpc.web, 1)
		if err != nil {
			return errors.Wrap(err, "failed to start web server")
		}
		var format string
		if msfrpc.web.disableTLS {
			format = "web server: http://%s/"
		} else {
			format = "web server: https://%s/"
		}
		msfrpc.logger.Printf(logger.Info, src, format, msfrpc.listener.Addr())
	}
	msfrpc.monitor.Start()
	msfrpc.logger.Print(logger.Info, src, "start monitor")
	msfrpc.logger.Print(logger.Info, src, "msfrpc is running")
	// send signal
	msfrpc.wait <- struct{}{}
	// receive error
	var errorList []error
	for err := range msfrpc.errCh {
		errorList = append(errorList, err)
	}
	l := len(errorList)
	if l == 0 {
		return nil
	}
	buf := bytes.NewBuffer(make([]byte, 0, 64*l))
	_, _ = fmt.Fprintln(buf, "receive error when exit msfrpc:")
	for i := 0; i < len(errorList); i++ {
		_, _ = fmt.Fprintf(buf, "id %d: %s\n", i+1, errorList[i])
	}
	return errors.New(buf.String())
}

// Wait is used to wait for Main().
func (msfrpc *MSFRPC) Wait() {
	<-msfrpc.wait
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
	// close database
	if msfrpc.database.Driver != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err = msfrpc.client.DBDisconnect(ctx)
		if err != nil {
			msfrpc.logger.Print(logger.Error, src, "appear error when disconnect database:", err)
			msfrpc.sendError(err)
		}
	}
	// close client
	err = msfrpc.client.Close()
	if err != nil {
		msfrpc.logger.Print(logger.Error, src, "appear error when close client:", err)
		msfrpc.sendError(err)
	}
	msfrpc.logger.Print(logger.Info, src, "client is closed")
	msfrpc.logger.Print(logger.Info, src, "msfrpc is exit")
	close(msfrpc.errCh)
}

func (msfrpc *MSFRPC) sendError(err error) {
	select {
	case msfrpc.errCh <- err:
	default:
		msfrpc.logger.Print(logger.Error, "exit", "error channel blocked\nerror to send: %s", err)
	}
}

// Serve is used to serve listener to inner web.
// external program can use internal/virtualconn.Listener for magical.
func (msfrpc *MSFRPC) Serve(listener net.Listener) error {
	return msfrpc.web.Serve(listener)
}

// Addresses is used to get listener addresses in web server.
func (msfrpc *MSFRPC) Addresses() []net.Addr {
	return msfrpc.web.Addresses()
}

// Reload is used to reload resource about web.
func (msfrpc *MSFRPC) Reload() error {
	if msfrpc.web.ui != nil {
		msfrpc.logger.Print(logger.Info, "msfrpc", "reload resource about web")
		return msfrpc.web.ui.Reload()
	}
	return nil
}
