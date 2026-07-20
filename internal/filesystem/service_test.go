package filesystem

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "book", "imgs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "book", "chapter10.md"), []byte("# Ten"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "book", "chapter2.md"), []byte("# Two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	service, err := New(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	return service, root
}

func TestResolveRejectsTraversalAndAbsolutePaths(t *testing.T) {
	service, _ := newTestService(t)
	cases := []string{"../secret", "book/../../secret", "/etc/passwd", `C:\\Windows\\system.ini`}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			_, _, err := service.Resolve(value)
			if !errors.Is(err, ErrInvalidPath) && !errors.Is(err, ErrOutsideRoot) {
				t.Fatalf("Resolve(%q) error = %v", value, err)
			}
		})
	}
}

func TestResolveRejectsSymlinkOutsideWorkspace(t *testing.T) {
	service, root := newTestService(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "escape.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, _, err := service.Resolve("escape.txt")
	if !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("expected ErrOutsideRoot, got %v", err)
	}
}

func TestListSortsDirectoriesThenNaturalFileNames(t *testing.T) {
	service, _ := newTestService(t)
	items, err := service.List("book")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("len(items) = %d", len(items))
	}
	if items[0].Kind != "directory" || items[0].Name != "imgs" {
		t.Fatalf("first item = %#v", items[0])
	}
	if items[1].Name != "chapter2.md" || items[2].Name != "chapter10.md" {
		t.Fatalf("unexpected order: %#v", items)
	}
}

func TestListHandlesLargeDirectories(t *testing.T) {
	service, root := newTestService(t)
	large := filepath.Join(root, "large")
	if err := os.Mkdir(large, 0o755); err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 1000; index++ {
		name := filepath.Join(large, fmt.Sprintf("file%d.txt", index))
		if err := os.WriteFile(name, []byte("fixture"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	items, err := service.List("large")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1000 {
		t.Fatalf("len(items) = %d", len(items))
	}
	if items[0].Name != "file1.txt" || items[9].Name != "file10.txt" || items[999].Name != "file1000.txt" {
		t.Fatalf("unexpected natural order: first=%q tenth=%q last=%q", items[0].Name, items[9].Name, items[999].Name)
	}
}

func TestReadTextHandlesLongFiles(t *testing.T) {
	_, root := newTestService(t)
	content := bytes.Repeat([]byte("reader content\n"), 70_000)
	if err := os.WriteFile(filepath.Join(root, "long.txt"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	service, err := New(root, int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}

	text, err := service.ReadText("long.txt")
	if err != nil {
		t.Fatal(err)
	}
	if text.Size != int64(len(content)) || !bytes.Equal([]byte(text.Content), content) {
		t.Fatalf("long text size = %d, content bytes = %d", text.Size, len(text.Content))
	}
}

func TestReadTextChecksLimitAndEncoding(t *testing.T) {
	service, root := newTestService(t)
	text, err := service.ReadText("notes.txt")
	if err != nil || text.Content != "hello" {
		t.Fatalf("ReadText = %#v, %v", text, err)
	}
	if err := os.WriteFile(filepath.Join(root, "bad.txt"), []byte{0xff, 0xfe}, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = service.ReadText("bad.txt")
	if !errors.Is(err, ErrInvalidEncoding) {
		t.Fatalf("expected encoding error, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "large.txt"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = service.ReadText("large.txt")
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("expected size error, got %v", err)
	}
}
