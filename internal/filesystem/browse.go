package filesystem

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"web_reader/internal/config"
)

// DirEntry is a directory discovered during path browsing.
type DirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// BrowseResult is the payload returned by BrowseDirectories.
type BrowseResult struct {
	Path string     `json:"path"`
	Dirs []DirEntry `json:"dirs"`
}

// BrowseDirectories lists the subdirectories of an absolute server path. It
// is intentionally NOT scoped to the current workspace root, because the
// settings modal needs to navigate the server filesystem to pick a new
// workspace. Sensitive system directories are rejected, and symlinked
// directories are skipped to avoid revealing targets outside the browsed tree.
func BrowseDirectories(raw string) (BrowseResult, error) {
	expanded, err := config.ExpandTilde(raw)
	if err != nil {
		return BrowseResult{}, err
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return BrowseResult{}, err
	}
	if _, statErr := os.Stat(abs); os.IsNotExist(statErr) {
		return BrowseResult{}, os.ErrNotExist
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return BrowseResult{}, err
	}
	if config.IsSensitiveSystemPath(real) {
		return BrowseResult{}, ErrOutsideRoot
	}
	info, err := os.Stat(real)
	if err != nil {
		return BrowseResult{}, err
	}
	if !info.IsDir() {
		return BrowseResult{}, ErrNotDirectory
	}
	entries, err := os.ReadDir(real)
	if err != nil {
		return BrowseResult{}, err
	}
	dirs := make([]DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		full := filepath.Join(real, entry.Name())
		if lInfo, lErr := os.Lstat(full); lErr == nil {
			if isSymlinkMode(lInfo.Mode()) {
				continue
			}
		}
		dirs = append(dirs, DirEntry{
			Name: entry.Name(),
			Path: filepath.ToSlash(full),
		})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return naturalLess(dirs[i].Name, dirs[j].Name)
	})
	return BrowseResult{Path: real, Dirs: dirs}, nil
}

// SplitPathSegments breaks an absolute path into breadcrumb segments. The
// root "/" is represented as a single empty-name segment so the UI can render
// it as a clickable root.
func SplitPathSegments(abs string) []DirEntry {
	clean := filepath.Clean(abs)
	var segments []DirEntry
	if clean == string(filepath.Separator) {
		return []DirEntry{{Name: "/", Path: clean}}
	}
	cur := ""
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == "" {
			cur = string(filepath.Separator)
			segments = append(segments, DirEntry{Name: "/", Path: cur})
			continue
		}
		cur = filepath.Join(cur, part)
		segments = append(segments, DirEntry{Name: part, Path: cur})
	}
	return segments
}
