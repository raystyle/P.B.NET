package kiwi

import (
	"fmt"
)

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/modules/kull_m_patch.h

// patchGeneric contains special data and offset.
type patchGeneric struct {
	minBuild uint32
	search   *patchPattern
	patch    *patchPattern
	offsets  *patchOffsets
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

// reference:
// https://github.com/gentilkiwi/mimikatz/blob/master/modules/kull_m_patch.c

func selectGenericPatch(patches []*patchGeneric, build uint32) *patchGeneric {
	for i := len(patches) - 1; i > 0; i-- {
		if build >= patches[i].minBuild {
			return patches[i]
		}
	}
	panic(fmt.Sprintf("failed to select generic patch with build %d", build))
}
