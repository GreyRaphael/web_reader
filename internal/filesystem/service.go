package filesystem

import (
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
	root        string
	maxTextSize int64
}

func New(root string, maxTextSize int64) (*Service, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace: %w", err)
	}
	realRoot, err = filepath.Abs(realRoot)
	if err != nil {
		return nil, fmt.Errorf("absolute workspace: %w", err)
	}
	return &Service{root: filepath.Clean(realRoot), maxTextSize: maxTextSize}, nil
}

func (s *Service) Resolve(relative string) (string, string, error) {
	if strings.ContainsRune(relative, 0) {
		return "", "", ErrInvalidPath
	}
	normalized := strings.ReplaceAll(relative, "\\", "/")
	if normalized == "" || normalized == "." {
		return s.root, "", nil
	}
	if strings.HasPrefix(normalized, "/") || filepath.IsAbs(normalized) || hasWindowsVolume(normalized) {
		return "", "", ErrInvalidPath
	}
	cleaned := path.Clean(normalized)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", "", ErrOutsideRoot
	}
	candidate := filepath.Join(s.root, filepath.FromSlash(cleaned))
	real, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", "", err
	}
	real, err = filepath.Abs(real)
	if err != nil {
		return "", "", err
	}
	relToRoot, err := filepath.Rel(s.root, real)
	if err != nil {
		return "", "", ErrOutsideRoot
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) || filepath.IsAbs(relToRoot) {
		return "", "", ErrOutsideRoot
	}
	return real, cleaned, nil
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
	info, err := os.Stat(full)
	if err != nil {
		return TextFile{}, err
	}
	if !info.Mode().IsRegular() {
		return TextFile{}, ErrNotFile
	}
	if info.Size() > s.maxTextSize {
		return TextFile{}, ErrFileTooLarge
	}
	file, err := os.Open(full)
	if err != nil {
		return TextFile{}, err
	}
	defer file.Close()
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
	file, err := os.Open(full)
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
