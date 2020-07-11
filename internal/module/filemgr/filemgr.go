package filemgr

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// name about task
const (
	TaskNameCopy       = "copy"
	TaskNameMove       = "move"
	TaskNameCompress   = "compress"
	TaskNameDecompress = "decompress"
)

// ErrCtrl is used to tell Move or Copy function how to control the same file,
// directory, or copy, move error. src and dst is the absolute file path.
type ErrCtrl func(typ uint8, err error, src string, dst string) uint8

// errors about ErrCtrl
const (
	_                  uint8 = iota
	ErrCtrlSameFile          // two same name file
	ErrCtrlSameFileDir       // same src file name with dst directory
	ErrCtrlSameDirFile       // same src directory name with dst file name
	ErrCtrlCopyFailed        // appear error when copy file
)

// operation code about ErrCtrl
const (
	_                uint8 = iota
	ErrCtrlOpReplace       // replace same name file
	ErrCtrlOpSkip          // skip same name file, directory or copy
	ErrCtrlOpRetry         // try to copy or move again
	ErrCtrlOpCancel        // cancel whole copy or move operation
)

// ErrUserCanceled is an error about user cancel copy or move.
var ErrUserCanceled = errors.New("user canceled")

// ReplaceAll is used to replace all src file to dst file.
var ReplaceAll = func(uint8, error, string, string) uint8 { return ErrCtrlOpReplace }

// SkipAll is used to skip all existed file or other error.
var SkipAll = func(uint8, error, string, string) uint8 { return ErrCtrlOpSkip }

type srcDstStat struct {
	srcAbs  string // "E:\file.dat" "E:\file", last will not be "/ or "\"
	dstAbs  string
	srcStat os.FileInfo
	dstStat os.FileInfo // check destination file or directory is exists

	// extra info
	dst       string // dstAbs will lost the last "/" or "\" // TODO remove it
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
	// replace the relative path to the absolute path for
	// prevent change current directory.
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return nil, err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return nil, err
	}
	if srcAbs == dstAbs {
		return nil, errors.New("src path as same as the dst path")
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
func noticeSameFile(ec ErrCtrl, stats *srcDstStat) (replace bool, err error) {
	switch code := ec(ErrCtrlSameFile, nil, stats.srcAbs, stats.dstAbs); code {
	case ErrCtrlOpReplace:
		replace = true
	case ErrCtrlOpSkip:
	case ErrCtrlOpCancel:
		err = ErrUserCanceled
	default:
		err = fmt.Errorf("unknown same file operation code: %d", code)
	}
	return
}

// noticeSameFileDir is used to notice appear same name about src file and dst dir.
func noticeSameFileDir(ec ErrCtrl, stats *srcDstStat) (retry bool, err error) {
	switch code := ec(ErrCtrlSameFileDir, nil, stats.srcAbs, stats.dstAbs); code {
	case ErrCtrlOpRetry:
		retry = true
	case ErrCtrlOpReplace, ErrCtrlOpSkip: // for ReplaceAll
	case ErrCtrlOpCancel:
		err = ErrUserCanceled
	default:
		err = fmt.Errorf("unknown same file dir operation code: %d", code)
	}
	return
}

// noticeSameDirFile is used to notice appear same name about src dir and dst file.
func noticeSameDirFile(ec ErrCtrl, stats *srcDstStat) (retry bool, err error) {
	switch code := ec(ErrCtrlSameDirFile, nil, stats.srcAbs, stats.dstAbs); code {
	case ErrCtrlOpRetry:
		retry = true
	case ErrCtrlOpReplace, ErrCtrlOpSkip: // for ReplaceAll
	case ErrCtrlOpCancel:
		err = ErrUserCanceled
	default:
		err = fmt.Errorf("unknown same dir file operation code: %d", code)
	}
	return
}

// noticeFailedToCopy is used to notice appear some error about copy or move.
func noticeFailedToCopy(ec ErrCtrl, stats *srcDstStat, e error) (retry bool, err error) {
	switch code := ec(ErrCtrlCopyFailed, e, stats.srcAbs, stats.dstAbs); code {
	case ErrCtrlOpRetry:
		retry = true
	case ErrCtrlOpReplace, ErrCtrlOpSkip: // for ReplaceAll
	case ErrCtrlOpCancel:
		err = ErrUserCanceled
	default:
		err = fmt.Errorf("unknown failed to copy operation code: %d", code)
	}
	return
}
