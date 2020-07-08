package filemgr

import (
	"context"
)

// Move is used to move file or directory from source path to destination path,
// if the target file is exist, will call exist function and replace it if replace
// function return true.
func Move(ec ErrCtrl, src, dst string) error {
	return moveWithContext(context.Background(), ec, src, dst)
}

// MoveWithContext is used to move file or directory from source path to destination
// path with context.
func MoveWithContext(ctx context.Context, ec ErrCtrl, src, dst string) error {
	return moveWithContext(ctx, ec, src, dst)
}

func moveWithContext(ctx context.Context, ec ErrCtrl, src, dst string) error {
	return nil
}
