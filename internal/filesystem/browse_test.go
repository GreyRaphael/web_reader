package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseDirectoriesReturnsOnlySubdirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := BrowseDirectories(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Dirs) != 2 {
		t.Fatalf("dir count = %d, want 2", len(result.Dirs))
	}
	if result.Dirs[0].Name != "alpha" || result.Dirs[1].Name != "beta" {
		t.Fatalf("order = %s, %s", result.Dirs[0].Name, result.Dirs[1].Name)
	}
	for _, d := range result.Dirs {
		if filepath.Dir(d.Path) != result.Path {
			t.Fatalf("dir %q not under resolved root %q", d.Path, result.Path)
		}
	}
}

func TestBrowseDirectoriesRejectsSensitivePath(t *testing.T) {
	_, err := BrowseDirectories("/etc")
	if !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("BrowseDirectories(/etc) = %v, want ErrOutsideRoot", err)
	}
}

func TestBrowseDirectoriesRejectsNonexistentPath(t *testing.T) {
	_, err := BrowseDirectories("/nonexistent-path-12345")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v, want os.ErrNotExist", err)
	}
}

func TestBrowseDirectoriesSkipsSymlinkedDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "real-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link-dir")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	result, err := BrowseDirectories(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range result.Dirs {
		if d.Name == "link-dir" {
			t.Fatal("symlinked directory was included in results")
		}
	}
	if len(result.Dirs) != 1 || result.Dirs[0].Name != "real-dir" {
		t.Fatalf("expected only real-dir, got %#v", result.Dirs)
	}
}
