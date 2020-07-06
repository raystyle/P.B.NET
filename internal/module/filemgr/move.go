package filemgr

import (
	"context"
)

// Move is used to move file or directory from src path to dst path.
func Move(replace SameCtrl, src, dst string) error {
	return move(context.Background(), replace, src, dst, true)
}

// MoveWithContext is used to move file or directory from src path to dst path with context.
func MoveWithContext(ctx context.Context, replace SameCtrl, src, dst string) error {
	return move(ctx, replace, src, dst, true)
}

func move(ctx context.Context, replace SameCtrl, src, dst string, delSrc bool) error {
	return nil
}
