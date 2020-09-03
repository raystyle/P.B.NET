package kiwi

import (
	"project/internal/module/windows/api"
)

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/inc/globals.h

var (
	buildWinXP      uint32 = 2600
	buildWin2003    uint32 = 3790
	buildWinVista   uint32 = 6000
	buildWin7       uint32 = 7000
	buildWin8       uint32 = 9200
	buildWin81      uint32 = 9600
	buildWin10v1507 uint32 = 10240
	buildWin10v1511 uint32 = 10586
	buildWin10v1607 uint32 = 14393
	buildWin10v1703 uint32 = 15063
	buildWin10v1709 uint32 = 16299
	buildWin10v1803 uint32 = 17134
	buildWin10v1809 uint32 = 17763
	buildWin10v1903 uint32 = 18362
	buildWin10v1909 uint32 = 18363
	buildWin10v2004 uint32 = 19041
)

var (
	buildMinWinXP    uint32 = 2500
	buildMinWin2003  uint32 = 3000
	buildMinWinVista uint32 = 5000
	buildMinWin7     uint32 = 7000
	buildMinWin8     uint32 = 8000
	buildMinWin81    uint32 = 9400
	buildMinWin10    uint32 = 9800
)

func (kiwi *Kiwi) getWindowsVersion() (major, minor, build uint32) {
	if kiwi.major == 0 {
		kiwi.major, kiwi.minor, kiwi.build = api.GetVersionNumber()
	}
	return kiwi.major, kiwi.minor, kiwi.build
}
