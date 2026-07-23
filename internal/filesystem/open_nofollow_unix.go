//go:build unix

package filesystem

import (
	"io/fs"
	"os"
	"syscall"
)

func openFileNoFollow(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag|syscall.O_NOFOLLOW, perm)
}

func isSymlinkMode(m fs.FileMode) bool {
	return m&os.ModeSymlink != 0
}
