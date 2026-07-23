//go:build !unix

package filesystem

import (
	"io/fs"
	"os"
)

func openFileNoFollow(name string, flag int, perm os.FileMode) (*os.File, error) {
	if info, err := os.Lstat(name); err == nil && isSymlinkMode(info.Mode()) {
		return nil, ErrOutsideRoot
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return os.OpenFile(name, flag, perm)
}

func isSymlinkMode(m fs.FileMode) bool {
	return m&os.ModeSymlink != 0
}
