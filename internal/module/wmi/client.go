// +build windows

package wmi

import (
	"runtime"
	"sync"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/pkg/errors"

	"project/internal/xpanic"
)

// Client is a WMI client.
type Client struct {
	args    []interface{}
	initErr chan error

	unknown    *ole.IUnknown
	query      *ole.IDispatch
	rawService *ole.VARIANT
	wmi        *ole.IDispatch

	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// NewClient is used to create a WMI client.
func NewClient(host, namespace string, args ...interface{}) (*Client, error) {
	client := Client{
		args:       append([]interface{}{host, namespace}, args...),
		initErr:    make(chan error, 1),
		stopSignal: make(chan struct{}),
	}
	client.wg.Add(1)
	go client.work()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case err := <-client.initErr:
		if err != nil {
			return nil, err
		}
		return &client, nil
	case <-timer.C:
		_ = client.Close()
		return nil, errors.New("initialize client timeout")
	}
}

func (client *Client) work() {
	defer client.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			xpanic.Log(r, "Client.work")
		}
	}()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := client.init()
	client.initErr <- err
	if err != nil {
		return
	}
	defer client.release()

	client.handleRequestLoop()
}

func (client *Client) init() error {
	var ok bool
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		errCode := err.(*ole.OleError).Code()
		// CoInitialize already called
		// https://msdn.microsoft.com/en-us/library/windows/desktop/ms695279%28v=vs.85%29.aspx
		if errCode != ole.S_OK && errCode != 0x00000001 { // S_FALSE
			return err
		}
	}
	defer func() {
		if !ok {
			ole.CoUninitialize()
		}
	}()
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return errors.Wrap(err, "failed to create object")
	}
	defer func() {
		if !ok {
			client.unknown.Release()
		}
	}()
	query, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return errors.Wrap(err, "failed to query interface")
	}
	defer func() {
		if !ok {
			client.query.Release()
		}
	}()
	// service is a SWbemServices
	rawService, err := oleutil.CallMethod(query, "ConnectServer", client.args...)
	if err != nil {
		return errors.Wrap(err, "failed to connect server")
	}
	client.unknown = unknown
	client.query = query
	client.rawService = rawService
	client.wmi = rawService.ToIDispatch()
	ok = true
	return nil
}

func (client *Client) release() {
	client.wmi.Release()
	_ = client.rawService.Clear()
	client.query.Release()
	client.unknown.Release()
	ole.CoUninitialize()
}

func (client *Client) handleRequestLoop() {
	for {
		select {

		case <-client.stopSignal:
			return
		}
	}
}

func (client *Client) handleQuery() {

}

func (client *Client) Query(wql string, dst interface{}) error {
	return nil
}

func (client *Client) Get(wql string, dst interface{}) error {

	return nil
}

func (client *Client) ExecMethod(wql string, dst interface{}) error {

	return nil
}

// Close is used to close WMI client.
func (client *Client) Close() (err error) {
	client.closeOnce.Do(func() {
		close(client.stopSignal)
		client.wg.Wait()
	})
	return
}
