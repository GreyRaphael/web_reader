package filesystem

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalidPath     = errors.New("invalid path")
	ErrOutsideRoot     = errors.New("path is outside workspace")
	ErrNotFile         = errors.New("not a file")
	ErrNotDirectory    = errors.New("not a directory")
	ErrFileTooLarge    = errors.New("file exceeds preview limit")
	ErrInvalidEncoding = errors.New("file is not valid UTF-8")
	ErrAlreadyExists   = errors.New("file or directory already exists")
	ErrNameEmpty       = errors.New("name cannot be empty")
)

type Item struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	PreviewKind string    `json:"previewKind"`
	Size        int64     `json:"size"`
	ModifiedAt  time.Time `json:"modifiedAt"`
	MIME        string    `json:"mime"`
}

type TextFile struct {
	Path       string    `json:"path"`
	Content    string    `json:"content"`
	Encoding   string    `json:"encoding"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

type Service struct {
	mu          sync.RWMutex
	root        string
	maxTextSize int64
}

func New(root string, maxTextSize int64) (*Service, error) {
	svc := &Service{maxTextSize: maxTextSize}
	if _, err := svc.SetRoot(root); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Service) GetRoot() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.root
}

func (s *Service) SetRoot(newRoot string) (string, error) {
	newRoot = strings.TrimSpace(newRoot)
	if newRoot == "" {
		return "", errors.New("path cannot be empty")
	}
	if newRoot == "~" || strings.HasPrefix(newRoot, "~/") || strings.HasPrefix(newRoot, "~\\") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if newRoot == "~" {
			newRoot = home
		} else {
			newRoot = filepath.Join(home, newRoot[2:])
		}
	}
	root, err := filepath.Abs(newRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workspace: %w", err)
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0755); err != nil {
			return "", fmt.Errorf("create workspace directory: %w", err)
		}
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace symlinks: %w", err)
	}
	info, err := os.Stat(realRoot)
	if err != nil {
		return "", fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("workspace must be a directory")
	}
	clean := filepath.Clean(realRoot)
	s.mu.Lock()
	s.root = clean
	s.mu.Unlock()
	return clean, nil
}

func (s *Service) Resolve(relative string) (string, string, error) {
	s.mu.RLock()
	root := s.root
	s.mu.RUnlock()

	if strings.ContainsRune(relative, 0) {
		return "", "", ErrInvalidPath
	}
	normalized := strings.ReplaceAll(relative, "\\", "/")
	if normalized == "" || normalized == "." {
		return root, "", nil
	}
	if strings.HasPrefix(normalized, "/") || filepath.IsAbs(normalized) || hasWindowsVolume(normalized) {
		return "", "", ErrInvalidPath
	}
	cleaned := path.Clean(normalized)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", "", ErrOutsideRoot
	}
	candidate := filepath.Join(root, filepath.FromSlash(cleaned))
	real, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", "", err
	}
	real, err = filepath.Abs(real)
	if err != nil {
		return "", "", err
	}
	relToRoot, err := filepath.Rel(root, real)
	if err != nil {
		return "", "", ErrOutsideRoot
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relToRoot) {
		return "", "", ErrOutsideRoot
	}
	return real, cleaned, nil
}

func (s *Service) rejectSymlinkLeaf(full string) error {
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if isSymlinkMode(info.Mode()) {
		return ErrOutsideRoot
	}
	return nil
}

func (s *Service) List(relative string) ([]Item, error) {
	full, logical, err := s.Resolve(relative)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrNotDirectory
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(entries))
	for _, entry := range entries {
		child := path.Join(logical, entry.Name())
		item, err := s.Info(child)
		if err != nil {
			if errors.Is(err, ErrOutsideRoot) || os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind == "directory"
		}
		return naturalLess(items[i].Name, items[j].Name)
	})
	return items, nil
}

func (s *Service) Info(relative string) (Item, error) {
	full, logical, err := s.Resolve(relative)
	if err != nil {
		return Item{}, err
	}
	info, err := os.Stat(full)
	if err != nil {
		return Item{}, err
	}
	item := Item{
		Path:       filepath.ToSlash(logical),
		Name:       info.Name(),
		Size:       info.Size(),
		ModifiedAt: info.ModTime().UTC(),
	}
	if info.IsDir() {
		item.Kind = "directory"
		item.PreviewKind = "unsupported"
		return item, nil
	}
	item.Kind = "file"
	item.MIME = detectMIME(full)
	item.PreviewKind = previewKind(logical, item.MIME)
	return item, nil
}

func (s *Service) ReadText(relative string) (TextFile, error) {
	full, logical, err := s.Resolve(relative)
	if err != nil {
		return TextFile{}, err
	}
	file, err := openFileNoFollow(full, os.O_RDONLY, 0)
	if err != nil {
		return TextFile{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return TextFile{}, err
	}
	if !info.Mode().IsRegular() {
		return TextFile{}, ErrNotFile
	}
	if info.Size() > s.maxTextSize {
		return TextFile{}, ErrFileTooLarge
	}
	content, err := io.ReadAll(io.LimitReader(file, s.maxTextSize+1))
	if err != nil {
		return TextFile{}, err
	}
	if int64(len(content)) > s.maxTextSize {
		return TextFile{}, ErrFileTooLarge
	}
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(content) {
		return TextFile{}, ErrInvalidEncoding
	}
	return TextFile{
		Path:       filepath.ToSlash(logical),
		Content:    string(content),
		Encoding:   "utf-8",
		Size:       info.Size(),
		ModifiedAt: info.ModTime().UTC(),
	}, nil
}

func (s *Service) Open(relative string) (*os.File, os.FileInfo, Item, error) {
	full, _, err := s.Resolve(relative)
	if err != nil {
		return nil, nil, Item{}, err
	}
	file, err := openFileNoFollow(full, os.O_RDONLY, 0)
	if err != nil {
		return nil, nil, Item{}, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, Item{}, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, nil, Item{}, ErrNotFile
	}
	item, err := s.Info(relative)
	if err != nil {
		file.Close()
		return nil, nil, Item{}, err
	}
	return file, info, item, nil
}

func (s *Service) CreateFile(relative string) (Item, error) {
	return s.createEntry(relative, false)
}

func (s *Service) CreateDir(relative string) (Item, error) {
	return s.createEntry(relative, true)
}

func (s *Service) createEntry(relative string, isDir bool) (Item, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/")
	if cleaned == "" || strings.HasSuffix(cleaned, "/") || cleaned == "." || cleaned == ".." {
		return Item{}, ErrInvalidPath
	}
	base := path.Base(cleaned)
	if base == "" || base == "." || base == ".." {
		return Item{}, ErrInvalidPath
	}
	parentDir := path.Dir(cleaned)
	fullParent, _, err := s.Resolve(parentDir)
	if err != nil {
		return Item{}, err
	}
	fullPath := filepath.Join(fullParent, base)
	real, err := filepath.EvalSymlinks(fullParent)
	if err != nil {
		return Item{}, err
	}
	real, err = filepath.Abs(real)
	if err != nil {
		return Item{}, err
	}
	relToRoot, err := filepath.Rel(s.root, real)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relToRoot) {
		return Item{}, ErrOutsideRoot
	}
	if _, err := os.Stat(fullPath); err == nil {
		return Item{}, ErrAlreadyExists
	}
	if isDir {
		if err := os.Mkdir(fullPath, 0o755); err != nil {
			return Item{}, err
		}
	} else {
		f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return Item{}, err
		}
		f.Close()
	}
	return s.Info(cleaned)
}

func (s *Service) Rename(relative, newName string) (Item, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return Item{}, ErrInvalidPath
	}
	newName = strings.TrimSpace(newName)
	if newName == "" || strings.ContainsRune(newName, '/') || strings.ContainsRune(newName, '\\') || newName == "." || newName == ".." {
		return Item{}, ErrNameEmpty
	}
	full, _, err := s.Resolve(cleaned)
	if err != nil {
		return Item{}, err
	}
	if err := s.rejectSymlinkLeaf(full); err != nil {
		return Item{}, err
	}
	parentDir := filepath.Dir(full)
	newFull := filepath.Join(parentDir, newName)
	relNew, err := filepath.Rel(s.root, newFull)
	if err != nil || relNew == ".." || strings.HasPrefix(relNew, ".."+string(filepath.Separator)) || filepath.IsAbs(relNew) {
		return Item{}, ErrOutsideRoot
	}
	if _, err := os.Stat(newFull); err == nil {
		return Item{}, ErrAlreadyExists
	}
	if err := os.Rename(full, newFull); err != nil {
		return Item{}, err
	}
	newLogical := filepath.ToSlash(path.Join(path.Dir(cleaned), newName))
	return s.Info(newLogical)
}

func (s *Service) Move(relative, targetDir string) (Item, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return Item{}, ErrInvalidPath
	}
	targetDir = strings.TrimSpace(targetDir)
	if targetDir == "" {
		targetDir = "."
	}
	full, _, err := s.Resolve(cleaned)
	if err != nil {
		return Item{}, err
	}
	if err := s.rejectSymlinkLeaf(full); err != nil {
		return Item{}, err
	}
	targetFull, targetLogical, err := s.Resolve(targetDir)
	if err != nil {
		return Item{}, err
	}
	targetInfo, err := os.Stat(targetFull)
	if err != nil {
		return Item{}, err
	}
	if !targetInfo.IsDir() {
		return Item{}, ErrNotDirectory
	}
	base := filepath.Base(full)
	newFull := filepath.Join(targetFull, base)
	if _, err := os.Stat(newFull); err == nil {
		return Item{}, ErrAlreadyExists
	}
	if err := os.Rename(full, newFull); err != nil {
		return Item{}, err
	}
	newLogical := filepath.ToSlash(path.Join(targetLogical, base))
	return s.Info(newLogical)
}

func (s *Service) Delete(relative string) error {
	cleaned := strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/")
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return ErrInvalidPath
	}
	full, _, err := s.Resolve(cleaned)
	if err != nil {
		return err
	}
	if err := s.rejectSymlinkLeaf(full); err != nil {
		return err
	}
	info, err := os.Stat(full)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(full)
	}
	return os.Remove(full)
}

func (s *Service) SaveUpload(relative string, body io.Reader) (Item, error) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(relative), "\\", "/")
	if cleaned == "" || strings.HasSuffix(cleaned, "/") || cleaned == "." || cleaned == ".." {
		return Item{}, ErrInvalidPath
	}
	base := path.Base(cleaned)
	if base == "" || base == "." || base == ".." {
		return Item{}, ErrInvalidPath
	}
	parentDir := path.Dir(cleaned)
	fullParent, _, err := s.Resolve(parentDir)
	if err != nil {
		return Item{}, err
	}
	fullPath := filepath.Join(fullParent, base)
	real, err := filepath.EvalSymlinks(fullParent)
	if err != nil {
		return Item{}, err
	}
	real, err = filepath.Abs(real)
	if err != nil {
		return Item{}, err
	}
	relToRoot, err := filepath.Rel(s.root, real)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relToRoot) {
		return Item{}, ErrOutsideRoot
	}
	if info, err := os.Lstat(fullPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return Item{}, ErrOutsideRoot
		}
	} else if !os.IsNotExist(err) {
		return Item{}, err
	}
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return Item{}, err
	}
	defer f.Close()
	if _, err := io.Copy(f, body); err != nil {
		return Item{}, err
	}
	return s.Info(cleaned)
}

func IsInlineImage(mimeType string) bool {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0])) {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/avif", "image/x-icon":
		return true
	default:
		return false
	}
}

func detectMIME(filename string) string {
	file, err := os.Open(filename)
	if err == nil {
		defer file.Close()
		buffer := make([]byte, 512)
		count, _ := file.Read(buffer)
		if count > 0 {
			detected := http.DetectContentType(buffer[:count])
			if detected != "application/octet-stream" {
				return detected
			}
		}
	}
	if byExt := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename))); byExt != "" {
		return byExt
	}
	return "application/octet-stream"
}

func previewKind(filename, mimeType string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".md" || ext == ".markdown" || ext == ".mdown" || ext == ".mkd" {
		return "markdown"
	}
	if IsInlineImage(mimeType) || isRasterExtension(ext) {
		return "image"
	}
	if strings.HasPrefix(mimeType, "text/") || isTextExtension(ext) {
		return "text"
	}
	return "unsupported"
}

func isRasterExtension(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".avif", ".ico":
		return true
	default:
		return false
	}
}

func isTextExtension(ext string) bool {
	switch ext {
	case ".txt", ".log", ".json", ".jsonl", ".yaml", ".yml", ".toml", ".ini", ".conf", ".csv", ".tsv", ".xml", ".html", ".css", ".js", ".jsx", ".ts", ".tsx", ".vue", ".go", ".rs", ".py", ".rb", ".java", ".c", ".h", ".cpp", ".hpp", ".sh", ".bash", ".zsh", ".sql", ".dockerfile":
		return true
	default:
		return false
	}
}

func hasWindowsVolume(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':'
}

func naturalLess(a, b string) bool {
	ar := []rune(strings.ToLower(a))
	br := []rune(strings.ToLower(b))
	for i, j := 0, 0; i < len(ar) && j < len(br); {
		if unicode.IsDigit(ar[i]) && unicode.IsDigit(br[j]) {
			ai, bj := i, j
			for ai < len(ar) && unicode.IsDigit(ar[ai]) {
				ai++
			}
			for bj < len(br) && unicode.IsDigit(br[bj]) {
				bj++
			}
			an := strings.TrimLeft(string(ar[i:ai]), "0")
			bn := strings.TrimLeft(string(br[j:bj]), "0")
			if len(an) != len(bn) {
				return len(an) < len(bn)
			}
			if an != bn {
				return an < bn
			}
			i, j = ai, bj
			continue
		}
		if ar[i] != br[j] {
			return ar[i] < br[j]
		}
		i++
		j++
	}
	return len(ar) < len(br)
}

func (s *Service) StreamZip(relative string, w io.Writer, setDisposition func(filename string)) (string, error) {
	full, _, err := s.Resolve(relative)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", ErrNotDirectory
	}
	baseDir := filepath.Base(full)
	filename := baseDir + ".zip"
	if setDisposition != nil {
		setDisposition(filename)
	}
	zw := zip.NewWriter(w)
	err = filepath.WalkDir(full, func(filePath string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if filePath == full {
			return nil
		}
		if d.IsDir() {
			relPath, rerr := filepath.Rel(full, filePath)
			if rerr != nil {
				return rerr
			}
			zipPath := filepath.ToSlash(filepath.Join(baseDir, relPath)) + "/"
			_, cerr := zw.CreateHeader(&zip.FileHeader{Name: zipPath, Method: zip.Deflate})
			return cerr
		}
		fileInfo, ferr := d.Info()
		if ferr != nil {
			return ferr
		}
		if isSymlinkMode(fileInfo.Mode()) {
			return nil
		}
		relPath, rerr := filepath.Rel(full, filePath)
		if rerr != nil {
			return rerr
		}
		zipPath := filepath.ToSlash(filepath.Join(baseDir, relPath))
		header, herr := zip.FileInfoHeader(fileInfo)
		if herr != nil {
			return herr
		}
		header.Name = zipPath
		header.Method = zip.Deflate
		writer, cerr := zw.CreateHeader(header)
		if cerr != nil {
			return cerr
		}
		f, oerr := openFileNoFollow(filePath, os.O_RDONLY, 0)
		if oerr != nil {
			return oerr
		}
		defer f.Close()
		_, copyErr := io.Copy(writer, f)
		return copyErr
	})
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	return baseDir + ".zip", nil
}
