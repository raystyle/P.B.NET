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
func Copy(sc SameCtrl, src, dst string) error {
	return copyWithContext(context.Background(), sc, src, dst)
}

// CopyWithContext is used to copy file or directory from source path to destination
// path with context.
func CopyWithContext(ctx context.Context, sc SameCtrl, src, dst string) error {
	return copyWithContext(ctx, sc, src, dst)
}

func copyWithContext(ctx context.Context, sc SameCtrl, src, dst string) error {
	stats, err := checkSrcDstPath(src, dst)
	if err != nil {
		return err
	}
	// copy file to a path
	if stats.srcIsFile {
		stats.dst = dst
		return copySrcFile(ctx, sc, stats)
	}
	// walk directory

	// copy C:\test -> D:\test2
	// -- copy C:\test\file.dat -> C:\test2\file.dat

	// skippedDirs is used to store skipped directories
	var skippedDirs []string

	// start walk
	return filepath.Walk(stats.srcAbs, func(srcAbs string, srcStat os.FileInfo, err error) error {
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
				err := noticeSameDirFile(sc, newStats)
				if err != nil {
					return err
				}
				skippedDirs = append(skippedDirs, srcAbs)
			}
			return nil
		}
		newStats.srcIsFile = true
		return copyFile(ctx, sc, newStats)
	})
}

// copySrcFile is used to copy single file to a path.
//
// new path is a dir  and exist
// new path is a file and exist
// new path is a dir  and not exist
// new path is a file and not exist
func copySrcFile(ctx context.Context, sc SameCtrl, stats *srcDstStat) error {
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
	return copyFile(ctx, sc, newStats)
}

// dst abs path doesn't have to exist, two abs path are all file.
func copyFile(ctx context.Context, sc SameCtrl, stats *srcDstStat) error {
	// check dst file is exist
	if stats.dstStat != nil {
		if stats.dstStat.IsDir() {
			return noticeSameFileDir(sc, stats)
		}
		next, err := noticeSameFile(sc, stats)
		if !next {
			return err
		}
	}
	// copy file
	srcFile, err := os.Open(stats.srcAbs)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()
	perm := stats.srcStat.Mode().Perm()
	dstFile, err := os.OpenFile(stats.dstAbs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()
	_, err = xio.CopyWithContext(ctx, dstFile, srcFile)
	if err != nil {
		// if canceled, remove the last file that not finish copy.
		if err == context.Canceled {
			_ = dstFile.Close()
			_ = os.Remove(stats.dstAbs)
		}
		return err
	}
	// set the modification time about the dst file
	t := stats.srcStat.ModTime()
	return os.Chtimes(stats.dstAbs, t, t)
}
