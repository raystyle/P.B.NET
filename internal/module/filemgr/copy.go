package filemgr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"project/internal/xio"
)

// Copy is used to copy file or directory from source path to destination path,
// if the target file is exist, will call exist function and replace it if replace
// function return true.
func Copy(ec ErrCtrl, src, dst string) error {
	return copyWithContext(context.Background(), ec, src, dst)
}

// CopyWithContext is used to copy file or directory from source path to destination
// path with context.
func CopyWithContext(ctx context.Context, ec ErrCtrl, src, dst string) error {
	return copyWithContext(ctx, ec, src, dst)
}

func copyWithContext(ctx context.Context, ec ErrCtrl, src, dst string) error {
	stats, err := checkSrcDstPath(src, dst)
	if err != nil {
		return err
	}
	// copy file to a path
	if stats.srcIsFile {
		stats.dst = dst
		return copySrcFile(ctx, ec, stats)
	}
	return copySrcDir(ctx, ec, stats)
}

// copySrcFile is used to copy single file to a path.
//
// new path is a dir  and exist
// new path is a file and exist
// new path is a dir  and not exist
// new path is a file and not exist
func copySrcFile(ctx context.Context, ec ErrCtrl, stats *srcDstStat) error {
	_, srcFileName := filepath.Split(stats.srcAbs)
	var (
		dstFileName string
		dstStat     os.FileInfo
	)
	if stats.dstStat != nil { // dst is exists
		// copyFile will handle the same file, dir
		//
		// copy "a.exe" -> "C:\ExistDir"
		// "a.exe" -> "C:\ExistDir\a.exe"
		if stats.dstStat.IsDir() {
			dstFileName = filepath.Join(stats.dstAbs, srcFileName)
			stat, err := stat(dstFileName)
			if err != nil {
				return err
			}
			dstStat = stat
		} else {
			dstFileName = stats.dstAbs
			dstStat = stats.dstStat
		}
	} else { // dst is doesn't exists
		last := stats.dst[len(stats.dst)-1]
		if os.IsPathSeparator(last) { // is a directory path
			err := os.MkdirAll(stats.dstAbs, 0750)
			if err != nil {
				return err
			}
			dstFileName = filepath.Join(stats.dstAbs, srcFileName)
		} else { // is a file path
			dir, _ := filepath.Split(stats.dstAbs)
			err := os.MkdirAll(dir, 0750)
			if err != nil {
				return err
			}
			dstFileName = stats.dstAbs
		}
	}
	newStats := &srcDstStat{
		srcAbs:    stats.srcAbs,
		dstAbs:    dstFileName,
		srcStat:   stats.srcStat,
		dstStat:   dstStat,
		srcIsFile: true,
	}
	return copyFile(ctx, ec, newStats)
}

// copy C:\test -> D:\test2
// -- copy C:\test\file.dat -> C:\test2\file.dat
func copySrcDir(ctx context.Context, ec ErrCtrl, stats *srcDstStat) error {
	var skippedDirs []string // used to store skipped directories
	walkFunc := func(srcAbs string, srcStat os.FileInfo, err error) error {
		// check is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// check walk error
		if err != nil {
			return fmt.Errorf("failed to walk \"%s\": %s", srcAbs, err)
		}
		// check is root path, and make directory if target path is not exists
		// C:\test -> D:\test[exist]
		if srcAbs == stats.srcAbs {
			if stats.dstStat == nil {
				return os.MkdirAll(stats.dstAbs, stats.srcStat.Mode().Perm())
			}
			return nil
		}
		// skip file in skipped directories
		for i := 0; i < len(skippedDirs); i++ {
			if strings.Contains(srcAbs, skippedDirs[i]) {
				return nil
			}
		}
		// C:\test\a.exe -> a.exe
		// C:\test\dir\a.exe -> dir\a.exe
		relativePath := strings.ReplaceAll(srcAbs, stats.srcAbs, "")
		// remove the first "\" or "/"
		relativePath = string([]rune(relativePath)[1:])
		dstAbs := filepath.Join(stats.dstAbs, relativePath)
	retry: // dstStat maybe updated
		dstStat, err := stat(dstAbs)
		if err != nil {
			return err
		}
		newStats := &srcDstStat{
			srcAbs:  srcAbs,
			dstAbs:  dstAbs,
			srcStat: srcStat,
			dstStat: dstStat,
		}
		if srcStat.IsDir() {
			if dstStat == nil {
				return os.MkdirAll(dstAbs, srcStat.Mode().Perm())
			}
			if !dstStat.IsDir() {
				retry, err := noticeSameDirFile(ec, newStats)
				if retry {
					goto retry
				}
				if err != nil {
					return err
				}
				skippedDirs = append(skippedDirs, srcAbs)
			}
			return nil
		}
		newStats.srcIsFile = true
		return copyFile(ctx, ec, newStats)
	}
	return filepath.Walk(stats.srcAbs, walkFunc)
}

// dst abs path doesn't have to exist, two abs path are all file.
func copyFile(ctx context.Context, ec ErrCtrl, stats *srcDstStat) (err error) {
	// check dst file is exist
	if stats.dstStat != nil {
		if stats.dstStat.IsDir() {
			retry, err := noticeSameFileDir(ec, stats)
			if retry {
				return retryCopyFile(ctx, ec, stats)
			}
			return err
		}
		replace, err := noticeSameFile(ec, stats)
		if !replace {
			return err
		}
	}
	// check copy file error, and maybe retry copy file.
	defer func() {
		if err != nil && err != context.Canceled {
			var retry bool
			retry, err = noticeFailedToCopy(ec, stats, err)
			if retry {
				err = retryCopyFile(ctx, ec, stats)
			}
		}
	}()
	// src file
	srcFile, err := os.Open(stats.srcAbs)
	if err != nil {
		return
	}
	defer func() { _ = srcFile.Close() }()
	// dst file
	perm := stats.srcStat.Mode().Perm()
	dstFile, err := os.OpenFile(stats.dstAbs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return
	}
	defer func() { _ = dstFile.Close() }()
	// copy file
	_, err = xio.CopyWithContext(ctx, dstFile, srcFile)
	if err != nil {
		_ = dstFile.Close()
		_ = os.Remove(stats.dstAbs)
		return
	}
	// sync
	err = dstFile.Sync()
	if err != nil {
		return
	}
	// set the modification time about the dst file
	modTime := stats.srcStat.ModTime()
	return os.Chtimes(stats.dstAbs, modTime, modTime)
}

// retryCopyFile will update src and dst file stat.
func retryCopyFile(ctx context.Context, ec ErrCtrl, stats *srcDstStat) error {
	var err error
	stats.srcStat, err = os.Stat(stats.srcAbs)
	if err != nil {
		return err
	}
	stats.dstStat, err = stat(stats.dstAbs)
	if err != nil {
		return err
	}
	return copyFile(ctx, ec, stats)
}

type copyTask struct {
}
