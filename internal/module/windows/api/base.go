package api

import (
	"time"
)

// AnySize is for array.
const AnySize int = 1

// reference:
// https://msdn.microsoft.com/en-us/library/windows/desktop/ms724284%28v=vs.85%29.aspx
// https://msdn.microsoft.com/en-us/library/cc230324.aspx

const unixEpochDiff int64 = 116444736000000000

// FileTime implements the Microsoft FILE_TIME type.
type FileTime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

// Time return a golang Time type from the FileTime
func (ft *FileTime) Time() time.Time {
	ns := (ft.MSEpoch() - unixEpochDiff) * 100
	return time.Unix(0, ns).UTC()
}

// Unix returns the FileTime as a Unix time, the number of seconds
// elapsed since January 1, 1970 UTC.
func (ft *FileTime) Unix() int64 {
	return (ft.MSEpoch() - unixEpochDiff) / 10000000
}

// MSEpoch returns the FileTime as a Microsoft epoch, the number of
// 100 nano second periods elapsed from January 1, 1601 UTC.
func (ft *FileTime) MSEpoch() int64 {
	return (int64(ft.HighDateTime) << 32) + int64(ft.LowDateTime)
}

// GetFileTime returns a FileTime type from the provided Golang Time type.
func GetFileTime(t time.Time) FileTime {
	ns := t.UnixNano()
	fp := (ns / 100) + unixEpochDiff
	hd := fp >> 32
	ld := fp - (hd << 32)
	return FileTime{
		LowDateTime:  uint32(ld),
		HighDateTime: uint32(hd),
	}
}
