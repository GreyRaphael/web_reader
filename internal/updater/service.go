package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	ErrUpdateInProgress = errors.New("update is already in progress")
	ErrNoUpdateFound    = errors.New("no newer version found")
	ErrAssetNotFound    = errors.New("no matching release asset found for current platform")
	ErrInvalidChecksum  = errors.New("checksum verification failed")
)

type Service struct {
	currentVersion     string
	repoOwner          string
	repoName           string
	apiBaseURL         string
	httpClient         *http.Client
	executablePathFunc func() (string, error)

	mu         sync.Mutex
	isUpdating bool
}

type ServiceOption func(*Service)

func WithRepo(owner, repo string) ServiceOption {
	return func(s *Service) {
		s.repoOwner = owner
		s.repoName = repo
	}
}

func WithAPIBaseURL(url string) ServiceOption {
	return func(s *Service) {
		s.apiBaseURL = strings.TrimSuffix(url, "/")
	}
}

func WithHTTPClient(client *http.Client) ServiceOption {
	return func(s *Service) {
		s.httpClient = client
	}
}

func WithExecutablePathFunc(fn func() (string, error)) ServiceOption {
	return func(s *Service) {
		s.executablePathFunc = fn
	}
}

// NewService creates a new updater service.
func NewService(currentVersion string, opts ...ServiceOption) *Service {
	s := &Service{
		currentVersion:     currentVersion,
		repoOwner:          "GreyRaphael",
		repoName:           "web_reader",
		apiBaseURL:         "https://api.github.com",
		httpClient:         &http.Client{Timeout: 30 * time.Second},
		executablePathFunc: os.Executable,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Check queries GitHub Releases to determine if a newer version is available.
func (s *Service) Check(ctx context.Context) (*CheckResult, error) {
	release, err := s.fetchLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}

	result := &CheckResult{
		CurrentVersion: s.currentVersion,
		LatestVersion:  release.TagName,
		ReleaseNotes:   release.Body,
		PublishedAt:    release.PublishedAt,
		ReleaseURL:     release.HTMLURL,
		CanAutoUpdate:  true,
	}

	// Compare versions
	cmp := CompareSemver(release.TagName, s.currentVersion)
	result.HasUpdate = cmp > 0

	// Match asset for the current OS and architecture
	matchedAsset, _ := s.findMatchingAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if matchedAsset == nil {
		result.CanAutoUpdate = false
		result.Reason = fmt.Sprintf("未找到适用于 %s-%s 的安装包", runtime.GOOS, runtime.GOARCH)
		return result, nil
	}
	result.AssetName = matchedAsset.Name
	result.AssetSize = matchedAsset.Size

	// Check if current running executable is in an updatable environment
	exePath, err := s.getExecutablePath()
	if err != nil {
		result.CanAutoUpdate = false
		result.Reason = "无法确定当前可执行文件路径: " + err.Error()
		return result, nil
	}

	if s.currentVersion == "dev" || strings.Contains(exePath, "go-build") {
		result.CanAutoUpdate = false
		result.Reason = "当前处于开发模式，请使用源码编译或手动安装"
		return result, nil
	}

	// Check directory writability
	exeDir := filepath.Dir(exePath)
	testFile := filepath.Join(exeDir, fmt.Sprintf(".wr_update_test_%d", time.Now().UnixNano()))
	if testErr := os.WriteFile(testFile, []byte("test"), 0o600); testErr != nil {
		result.CanAutoUpdate = false
		result.Reason = "可执行文件所在目录无写权限，请手动下载更新"
		return result, nil
	}
	_ = os.Remove(testFile)

	return result, nil
}

// Apply downloads the latest asset, validates it, replaces the executable,
// and schedules a restart callback.
func (s *Service) Apply(ctx context.Context, onRestart func()) (*ApplyResult, error) {
	s.mu.Lock()
	if s.isUpdating {
		s.mu.Unlock()
		return nil, ErrUpdateInProgress
	}
	s.isUpdating = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isUpdating = false
		s.mu.Unlock()
	}()

	checkRes, err := s.Check(ctx)
	if err != nil {
		return nil, fmt.Errorf("check update before apply: %w", err)
	}
	if !checkRes.CanAutoUpdate {
		return nil, errors.New("cannot auto update: " + checkRes.Reason)
	}

	release, err := s.fetchLatestRelease(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch release details: %w", err)
	}

	asset, shaAsset := s.findMatchingAsset(release.Assets, runtime.GOOS, runtime.GOARCH)
	if asset == nil {
		return nil, ErrAssetNotFound
	}

	// Fetch expected checksum if available
	var expectedChecksum string
	if shaAsset != nil {
		expectedChecksum, err = s.downloadChecksum(ctx, shaAsset.BrowserDownloadURL)
		if err != nil {
			slog.Warn("failed to fetch sha256 checksum, proceeding with download", "error", err)
		}
	}

	// Download archive to a temporary file
	exePath, err := s.getExecutablePath()
	if err != nil {
		return nil, fmt.Errorf("get executable path: %w", err)
	}
	exeDir := filepath.Dir(exePath)

	tempArchive, err := os.CreateTemp(exeDir, "wr_download_*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temp download file: %w", err)
	}
	tempArchiveName := tempArchive.Name()
	defer os.Remove(tempArchiveName)

	hasher := sha256.New()
	writer := io.MultiWriter(tempArchive, hasher)

	if dlErr := s.downloadFile(ctx, asset.BrowserDownloadURL, writer); dlErr != nil {
		_ = tempArchive.Close()
		return nil, fmt.Errorf("download update asset: %w", dlErr)
	}
	_ = tempArchive.Close()

	if expectedChecksum != "" {
		actualChecksum := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actualChecksum, expectedChecksum) {
			return nil, fmt.Errorf("%w: expected %s, got %s", ErrInvalidChecksum, expectedChecksum, actualChecksum)
		}
	}

	// Extract binary from downloaded archive
	targetBinaryName := "web-reader"
	if runtime.GOOS == "windows" {
		targetBinaryName = "web-reader.exe"
	}

	tempNewExe, err := os.CreateTemp(exeDir, "wr_bin_*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temp binary file: %w", err)
	}
	tempNewExePath := tempNewExe.Name()
	defer os.Remove(tempNewExePath)

	switch {
	case strings.HasSuffix(asset.Name, ".zip"):
		err = extractFromZip(tempArchiveName, targetBinaryName, tempNewExe)
	case strings.HasSuffix(asset.Name, ".tar.gz"), strings.HasSuffix(asset.Name, ".tgz"):
		err = extractFromTarGz(tempArchiveName, targetBinaryName, tempNewExe)
	default:
		err = fmt.Errorf("unsupported archive format: %s", asset.Name)
	}
	_ = tempNewExe.Close()
	if err != nil {
		return nil, fmt.Errorf("extract binary: %w", err)
	}

	// Ensure binary has executable permissions
	if err := os.Chmod(tempNewExePath, 0o755); err != nil {
		return nil, fmt.Errorf("chmod binary: %w", err)
	}

	// Perform in-place replacement
	if err := replaceExecutable(exePath, tempNewExePath); err != nil {
		return nil, fmt.Errorf("replace executable: %w", err)
	}

	slog.Info("executable updated successfully", "new_version", release.TagName, "path", exePath)

	if onRestart != nil {
		onRestart()
	}

	return &ApplyResult{
		Status:        "success",
		Message:       fmt.Sprintf("更新成功，已升级至 %s，服务正在重启...", release.TagName),
		TargetVersion: release.TagName,
	}, nil
}

func (s *Service) fetchLatestRelease(ctx context.Context) (*gitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", s.apiBaseURL, s.repoOwner, s.repoName)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "web-reader-updater")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("github api returned status %d: %s", resp.StatusCode, string(body))
	}

	var release gitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func (s *Service) findMatchingAsset(assets []gitHubAsset, goos, goarch string) (*gitHubAsset, *gitHubAsset) {
	var suffix string
	if goos == "windows" {
		suffix = fmt.Sprintf("-%s-%s.zip", goos, goarch)
	} else {
		suffix = fmt.Sprintf("-%s-%s.tar.gz", goos, goarch)
	}

	var matchedAsset *gitHubAsset
	for i := range assets {
		if strings.HasSuffix(assets[i].Name, suffix) {
			matchedAsset = &assets[i]
			break
		}
	}
	if matchedAsset == nil {
		return nil, nil
	}

	shaSuffix := matchedAsset.Name + ".sha256"
	var shaAsset *gitHubAsset
	for i := range assets {
		if assets[i].Name == shaSuffix {
			shaAsset = &assets[i]
			break
		}
	}

	return matchedAsset, shaAsset
}

func (s *Service) downloadFile(ctx context.Context, rawURL string, w io.Writer) error {
	fullURL := rawURL
	if strings.HasPrefix(rawURL, "/") {
		fullURL = s.apiBaseURL + rawURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "web-reader-updater")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status code %d", resp.StatusCode)
	}

	_, err = io.Copy(w, resp.Body)
	return err
}

func (s *Service) downloadChecksum(ctx context.Context, rawURL string) (string, error) {
	fullURL := rawURL
	if strings.HasPrefix(rawURL, "/") {
		fullURL = s.apiBaseURL + rawURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "web-reader-updater")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download checksum status code %d", resp.StatusCode)
	}

	content, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}

	// Usually format is "<hash>  <filename>" or "<hash>"
	fields := strings.Fields(string(content))
	if len(fields) > 0 {
		return fields[0], nil
	}
	return "", errors.New("empty checksum content")
}

func (s *Service) getExecutablePath() (string, error) {
	exe, err := s.executablePathFunc()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

const maxDecompressedBinarySize = 100 << 20 // 100 MiB limit to prevent decompression bomb

func extractFromZip(zipPath, targetName string, out io.Writer) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		// Clean and prevent zip slip
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue
		}
		if filepath.Base(cleanName) == targetName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()
			_, err = io.Copy(out, io.LimitReader(rc, maxDecompressedBinarySize))
			return err
		}
	}
	return fmt.Errorf("file %q not found in zip archive", targetName)
}

func extractFromTarGz(tarGzPath, targetName string, out io.Writer) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		// Prevent tar slip
		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue
		}
		if filepath.Base(cleanName) == targetName && hdr.Typeflag == tar.TypeReg {
			_, err = io.Copy(out, io.LimitReader(tr, maxDecompressedBinarySize))
			return err
		}
	}
	return fmt.Errorf("file %q not found in tar.gz archive", targetName)
}

// replaceExecutable safely replaces the current executable file across platforms.
func replaceExecutable(targetPath, newBinaryPath string) error {
	oldBackupPath := targetPath + ".old"
	_ = os.Remove(oldBackupPath)

	// Step 1: Rename current running binary to .old (permitted on Windows & POSIX)
	if err := os.Rename(targetPath, oldBackupPath); err != nil {
		return fmt.Errorf("rename running binary to .old: %w", err)
	}

	// Step 2: Rename new binary to targetPath
	if err := os.Rename(newBinaryPath, targetPath); err != nil {
		// Attempt rollback
		_ = os.Rename(oldBackupPath, targetPath)
		return fmt.Errorf("move new binary into place: %w", err)
	}

	// Step 3: Cleanup .old file (on Windows, may fail while running, safe to ignore)
	_ = os.Remove(oldBackupPath)
	return nil
}
