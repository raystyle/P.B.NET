package kiwi

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

type patchGeneric struct {
	search  *patchPattern
	patch   *patchPattern
	offsets *patchOffsets
}

// x64
var (
	patternWin5X64LogonSessionList = []byte{
		0x4C, 0x8B, 0xDF, 0x49, 0xC1, 0xE3, 0x04, 0x48, 0x8B, 0xCB, 0x4C, 0x03, 0xD8,
	}
	patternWin60X64LogonSessionList = []byte{
		0x33, 0xFF, 0x45, 0x85, 0xC0, 0x41, 0x89, 0x75, 0x00, 0x4C, 0x8B, 0xE3, 0x0F, 0x84,
	}
	patternWin61X64LogonSessionList = []byte{
		0x33, 0xF6, 0x45, 0x89, 0x2F, 0x4C, 0x8B, 0xF3, 0x85, 0xFF, 0x0F, 0x84,
	}
	patternWin63X64LogonSessionList = []byte{
		0x8B, 0xDE, 0x48, 0x8D, 0x0C, 0x5B, 0x48, 0xC1, 0xE1, 0x05, 0x48, 0x8D, 0x05,
	}
	patternWin6xX64LogonSessionList = []byte{
		0x33, 0xFF, 0x41, 0x89, 0x37, 0x4C, 0x8B, 0xF3, 0x45, 0x85, 0xC0, 0x74,
	}
	patternWin1703X64LogonSessionList = []byte{
		0x33, 0xFF, 0x45, 0x89, 0x37, 0x48, 0x8B, 0xF3, 0x45, 0x85, 0xC9, 0x74,
	}
	patternWin1803X64LogonSessionList = []byte{
		0x33, 0xFF, 0x45, 0x89, 0x37, 0x48, 0x8B, 0xF3, 0x45, 0x85, 0xC9, 0x74,
	}
	// key = build
	lsaSrvX64References = map[uint32]*patchGeneric{
		buildWinXP: {
			search: &patchPattern{
				length: len(patternWin5X64LogonSessionList),
				data:   patternWin5X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: 0},
		},
		buildWin2003: {
			search: &patchPattern{
				length: len(patternWin5X64LogonSessionList),
				data:   patternWin5X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: -4, off1: -45},
		},
		buildWinVista: {
			search: &patchPattern{
				length: len(patternWin60X64LogonSessionList),
				data:   patternWin60X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 21, off1: -4},
		},
		buildWin7: {
			search: &patchPattern{
				length: len(patternWin61X64LogonSessionList),
				data:   patternWin61X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 19, off1: -4},
		},
		buildWin8: {
			search: &patchPattern{
				length: len(patternWin6xX64LogonSessionList),
				data:   patternWin6xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
		buildWinBlue: {
			search: &patchPattern{
				length: len(patternWin63X64LogonSessionList),
				data:   patternWin63X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 36, off1: -6},
		},
		buildWin10v1507: {
			search: &patchPattern{
				length: len(patternWin6xX64LogonSessionList),
				data:   patternWin6xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 16, off1: -4},
		},
		buildWin10v1703: {
			search: &patchPattern{
				length: len(patternWin1703X64LogonSessionList),
				data:   patternWin1703X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 23, off1: -4},
		},
		buildWin10v1803: {
			search: &patchPattern{
				length: len(patternWin1803X64LogonSessionList),
				data:   patternWin1803X64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 23, off1: -4},
		},
		buildWin10v1903: {
			search: &patchPattern{
				length: len(patternWin6xX64LogonSessionList),
				data:   patternWin6xX64LogonSessionList,
			},
			patch:   &patchPattern{length: 0, data: nil},
			offsets: &patchOffsets{off0: 23, off1: -4},
		},
	}
)

// x86
