// +build windows

package kiwi

import (
	"sort"
	"sync"

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

	debug bool   // privilege
	pid   uint32 // PID about lsass.exe
	mu    sync.Mutex
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
	pid, err := kiwi.getLSASSProcessID()
	if err != nil {
		return nil, err
	}
	kiwi.logf(logger.Info, "PID of lsass.exe is %d", pid)
	pHandle, err := kiwi.getLSASSHandle(pid)
	if err != nil {
		return nil, err
	}
	kiwi.logf(logger.Info, "Handle of lsass.exe is %d", pHandle)

	return nil, nil
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

func (kiwi *Kiwi) getLSASSHandle(pid uint32) (windows.Handle, error) {
	var da uint32

	return windows.OpenProcess(da, false, pid)
}

func (kiwi *Kiwi) loadLSASSMemory() {

}
