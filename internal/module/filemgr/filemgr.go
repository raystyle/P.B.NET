package filemgr

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SameCtrl is used to tell Move or Copy function how to control the same file, directory.
// If replace function return 0, Move or Copy will replace it.
// src and dst is the absolute file path.
type SameCtrl func(typ uint8, src string, dst string) uint8

// same type about SameCtrl
const (
	_           uint8 = iota
	SameFile          // two same name file
	SameFileDir       // same src file name with dst directory
	SameDirFile       // same src directory name with dst file name
)

// control code about SameCtrl
const (
	_               uint8 = iota
	SameCtrlReplace       // replace same file
	SameCtrlSkip          // skip same file
	SameCtrlCancel        // cancel whole copy or move operation
)

// ErrUserCanceled is an error about user cancel copy or move.
var ErrUserCanceled = errors.New("user canceled")

// ReplaceAll is used to replace all src file to dst file.
var ReplaceAll = func(uint8, string, string) uint8 { return SameCtrlReplace }

// SkipAll is used to skip all existed file or other error.
var SkipAll = func(uint8, string, string) uint8 { return SameCtrlSkip }

type srcDstStat struct {
	dst       string // dstAbs will lost the last "/" or "\"
	srcAbs    string // "E:\file.dat" "E:\file", last will not be "/ or "\"
	dstAbs    string
	srcStat   os.FileInfo
	dstStat   os.FileInfo // check destination file or directory is exists
	srcIsFile bool
}

// stat is used to get file stat, if err is NotExist, it will return nil error and os.FileInfo.
func stat(name string) (os.FileInfo, error) {
	stat, err := os.Stat(name)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	return stat, nil
}

// src path [file], dst path [file] --valid
// src path [file], dst path [dir]  --valid
// src path [dir],  dst path [dir]  --valid
// src path [dir],  dst path [file] --invalid
func checkSrcDstPath(src, dst string) (*srcDstStat, error) {
	if src == "" {
		return nil, errors.New("empty src path")
	}
	if dst == "" {
		return nil, errors.New("empty dst path")
	}
	// replace the relative path to the absolute path for prevent call os.Chdir().
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return nil, err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return nil, err
	}
	if srcAbs == dstAbs {
		return nil, errors.New("source path as same as the destination path")
	}
	// check two path is valid
	srcStat, err := os.Stat(srcAbs)
	if err != nil {
		return nil, err
	}
	dstStat, err := stat(dstAbs)
	if err != nil {
		return nil, err
	}
	srcIsDir := srcStat.IsDir()
	if srcIsDir && dstStat != nil && !dstStat.IsDir() {
		const format = "\"%s\" is a directory but \"%s\" is a file"
		return nil, fmt.Errorf(format, srcAbs, dstAbs)
	}
	return &srcDstStat{
		srcAbs:    srcAbs,
		dstAbs:    dstAbs,
		srcStat:   srcStat,
		dstStat:   dstStat,
		srcIsFile: !srcIsDir,
	}, nil
}

// noticeSameFile is used to notice appear same name file.
func noticeSameFile(sc SameCtrl, stats *srcDstStat) (bool, error) {
	switch code := sc(SameFile, stats.srcAbs, stats.dstAbs); code {
	case SameCtrlReplace:
		return true, nil
	case SameCtrlSkip:
		return false, nil
	case SameCtrlCancel:
		return false, ErrUserCanceled
	default:
		return false, fmt.Errorf("unknown same file control code: %d", code)
	}
}

// noticeSameFileDir is used to notice appear same name about src file and dst dir.
func noticeSameFileDir(sc SameCtrl, stats *srcDstStat) error {
	switch code := sc(SameFileDir, stats.srcAbs, stats.dstAbs); code {
	case SameCtrlSkip:
		return nil
	case SameCtrlCancel:
		return ErrUserCanceled
	default:
		return fmt.Errorf("unknown same file dir control code: %d", code)
	}
}

// noticeSameDirFile is used to notice appear same name about src dir and dst file.
func noticeSameDirFile(sc SameCtrl, stats *srcDstStat) error {
	switch code := sc(SameDirFile, stats.srcAbs, stats.dstAbs); code {
	case SameCtrlSkip:
		return nil
	case SameCtrlCancel:
		return ErrUserCanceled
	default:
		return fmt.Errorf("unknown same dir file control code: %d", code)
	}
}
