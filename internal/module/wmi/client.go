// +build windows

package wmi

import (
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/pkg/errors"

	"project/internal/xpanic"
)

type execQuery struct {
	WQL string
	Dst interface{}
	Err chan<- error
}

type get struct {
	Path   string
	Args   []interface{}
	Result chan<- *getResult
}

type getResult struct {
	Object *Object
	Err    error
}

type execMethod struct {
	Path   string
	Method string
	Args   []interface{}
	Dst    interface{}
	Err    chan<- error
}

// Client is a WMI client.
type Client struct {
	args    []interface{}
	initErr chan error

	unknown    *ole.IUnknown
	query      *ole.IDispatch
	rawService *ole.VARIANT
	wmi        *ole.IDispatch

	queryQueue chan *execQuery
	getQueue   chan *get
	execQueue  chan *execMethod

	closeOnce  sync.Once
	stopSignal chan struct{}
	wg         sync.WaitGroup
}

// NewClient is used to create a WMI client.
func NewClient(host, namespace string, args ...interface{}) (*Client, error) {
	client := Client{
		args:       append([]interface{}{host, namespace}, args...),
		initErr:    make(chan error, 1),
		queryQueue: make(chan *execQuery, 16),
		getQueue:   make(chan *get, 16),
		execQueue:  make(chan *execMethod, 16),
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
		client.Close()
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
	defer client.close()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := client.init()
	client.initErr <- err
	if err != nil {
		return
	}
	defer client.release()

	client.handleLoop()
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

func (client *Client) handleLoop() {
	var (
		query *execQuery
		get   *get
		exec  *execMethod
	)
	for {
		select {
		case query = <-client.queryQueue:
			client.handleExecQuery(query)
		case get = <-client.getQueue:
			client.handleGet(get)
		case exec = <-client.execQueue:
			client.handleExecMethod(exec)
		case <-client.stopSignal:
			return
		}
	}
}

func (client *Client) handleExecQuery(query *execQuery) {
	var err error
	defer func() { query.Err <- err }()

	result, err := oleutil.CallMethod(client.wmi, "ExecQuery", query.WQL)
	if err != nil {
		return
	}
	object := Object{raw: result}
	defer object.Clear()

	objects, err := object.objects()
	if err != nil {
		return
	}
	defer func() {
		for i := 0; i < len(objects); i++ {
			objects[i].Clear()
		}
	}()
	err = parseExecQueryResult(objects, query.Dst)
}

func (client *Client) handleGet(get *get) {
	var getResult getResult
	defer func() { get.Result <- &getResult }()

	params := append([]interface{}{get.Path}, get.Args...)
	result, err := oleutil.CallMethod(client.wmi, "Get", params...)
	if err != nil {
		getResult.Err = err
		return
	}
	getResult.Object = &Object{raw: result}
}

func (client *Client) handleExecMethod(exec *execMethod) {
	var err error
	defer func() { exec.Err <- err }()

	var (
		result    *ole.VARIANT
		resultObj *Object
	)
	if strings.Contains(exec.Path, ".") {
		params := append([]interface{}{exec.Path, exec.Method}, exec.Args...)
		result, err = oleutil.CallMethod(client.wmi, "ExecMethod", params...)
		if err != nil {
			return
		}
		resultObj = &Object{raw: result}
	} else {
		// get object by path if path not contain "."
		result, err = oleutil.CallMethod(client.wmi, "Get", exec.Path)
		if err != nil {
			return
		}
		object := Object{raw: result}
		defer object.Clear()
		// execute method
		resultObj, err = object.ExecMethod(exec.Method, exec.Args...)
		if err != nil {
			return
		}
	}
	defer resultObj.Clear()
	err = parseExecMethodResult(resultObj, exec.Dst)
}

const clientClosed = "wmi client is closed"

// Query is used to query with WQL, dst is used to save query result.
// destination interface must be slice pointer like *[]*Type or *[]Type.
func (client *Client) Query(wql string, dst interface{}) error {
	errCh := make(chan error, 1)
	query := execQuery{
		WQL: wql,
		Dst: dst,
		Err: errCh,
	}
	select {
	case client.queryQueue <- &query:
	case <-client.stopSignal:
		return errors.New("failed to query: " + clientClosed)
	}
	var err error
	select {
	case err = <-errCh:
	case <-client.stopSignal:
		return errors.New("failed to receive query error: " + clientClosed)
	}
	return err
}

// Get is used to get a object.
func (client *Client) Get(path string, args ...interface{}) (*Object, error) {
	result := make(chan *getResult, 1)
	get := get{
		Path:   path,
		Args:   args,
		Result: result,
	}
	select {
	case client.getQueue <- &get:
	case <-client.stopSignal:
		return nil, errors.New("failed to get object: " + clientClosed)
	}
	var getResult *getResult
	select {
	case getResult = <-result:
	case <-client.stopSignal:
		return nil, errors.New("failed to receive get object result: " + clientClosed)
	}
	if getResult.Err != nil {
		return nil, getResult.Err
	}
	return getResult.Object, nil
}

// ExecMethod is used to execute a method about object, dst is used to save execute result.
// destination interface must be slice pointer like *[]*Type or *[]Type.
func (client *Client) ExecMethod(path, method string, dst interface{}, args ...interface{}) error {
	errCh := make(chan error, 1)
	exec := execMethod{
		Path:   path,
		Method: method,
		Args:   args,
		Dst:    dst,
		Err:    errCh,
	}
	select {
	case client.execQueue <- &exec:
	case <-client.stopSignal:
		return errors.New("failed to execute method: " + clientClosed)
	}
	var err error
	select {
	case err = <-errCh:
	case <-client.stopSignal:
		return errors.New("failed to receive execute method error: " + clientClosed)
	}
	return err
}

// Close is used to close WMI client.
func (client *Client) Close() {
	client.close()
	client.wg.Wait()
}

func (client *Client) close() {
	client.closeOnce.Do(func() {
		close(client.stopSignal)
	})
}

// BuildWQL is used to build structure to WQL string.
//
// type testWin32Process struct {
//     Name   string
//     PID    uint32 `wmi:"ProcessId"`
//     Ignore string `wmi:"-"`
// }
//
// to select Name, ProcessId from Win32_Process
func BuildWQL(structure interface{}, form string) string {
	fields := getStructFields(reflect.TypeOf(structure))
	fieldsLen := len(fields)
	// remove empty string
	f := make([]string, 0, fieldsLen)
	for i := 0; i < fieldsLen; i++ {
		if fields[i] != "" {
			f = append(f, fields[i])
		}
	}
	return "select " + strings.Join(f, ", ") + " from " + form
}
