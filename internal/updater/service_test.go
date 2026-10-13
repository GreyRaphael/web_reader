package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func createMockTarGz(t *testing.T, filename string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: filename,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func createMockZip(t *testing.T, filename string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCheck_NoUpdate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/testowner/testrepo/releases/latest" {
			http.NotFound(w, r)
			return
		}
		release := gitHubRelease{
			TagName:     "v1.7.1",
			Name:        "Release v1.7.1",
			Body:        "No changes",
			PublishedAt: "2026-09-28T00:00:00Z",
			HTMLURL:     "https://github.com/testowner/testrepo/releases/v1.7.1",
			Assets: []gitHubAsset{
				{
					Name:               fmt.Sprintf("web-reader-v1.7.1-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH),
					BrowserDownloadURL: "https://example.com/download.tar.gz",
					Size:               12345,
				},
			},
		}
		if runtime.GOOS == "windows" {
			release.Assets[0].Name = fmt.Sprintf("web-reader-v1.7.1-%s-%s.zip", runtime.GOOS, runtime.GOARCH)
		}
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer ts.Close()

	svc := NewService("v1.7.1",
		WithRepo("testowner", "testrepo"),
		WithAPIBaseURL(ts.URL),
		WithHTTPClient(ts.Client()),
	)

	res, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if res.HasUpdate {
		t.Errorf("expected hasUpdate=false, got true")
	}
	if res.LatestVersion != "v1.7.1" {
		t.Errorf("expected latestVersion=v1.7.1, got %s", res.LatestVersion)
	}
}

func TestCheck_HasUpdateAndMatchAsset(t *testing.T) {
	tempDir := t.TempDir()
	fakeExe := filepath.Join(tempDir, "web-reader")
	if runtime.GOOS == "windows" {
		fakeExe += ".exe"
	}
	if err := os.WriteFile(fakeExe, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	assetName := fmt.Sprintf("web-reader-v1.7.2-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName = fmt.Sprintf("web-reader-v1.7.2-%s-%s.zip", runtime.GOOS, runtime.GOARCH)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := gitHubRelease{
			TagName:     "v1.7.2",
			Name:        "Release v1.7.2",
			Body:        "Fixes bug",
			PublishedAt: "2026-09-28T10:00:00Z",
			HTMLURL:     "https://github.com/testowner/testrepo/releases/v1.7.2",
			Assets: []gitHubAsset{
				{
					Name:               assetName,
					BrowserDownloadURL: "https://example.com/download.tar.gz",
					Size:               54321,
				},
				{
					Name:               assetName + ".sha256",
					BrowserDownloadURL: "https://example.com/download.tar.gz.sha256",
					Size:               64,
				},
			},
		}
		_ = json.NewEncoder(w).Encode(release)
	}))
	defer ts.Close()

	svc := NewService("v1.7.1",
		WithRepo("testowner", "testrepo"),
		WithAPIBaseURL(ts.URL),
		WithHTTPClient(ts.Client()),
		WithExecutablePathFunc(func() (string, error) {
			return fakeExe, nil
		}),
	)

	res, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if !res.HasUpdate {
		t.Errorf("expected hasUpdate=true, got false")
	}
	if res.LatestVersion != "v1.7.2" {
		t.Errorf("expected latestVersion=v1.7.2, got %s", res.LatestVersion)
	}
	if !res.CanAutoUpdate {
		t.Errorf("expected canAutoUpdate=true, got false (reason: %s)", res.Reason)
	}
	if res.AssetName != assetName {
		t.Errorf("expected assetName=%s, got %s", assetName, res.AssetName)
	}
}

func TestCheck_DevMode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(gitHubRelease{
			TagName: "v1.7.2",
			Assets: []gitHubAsset{
				{Name: fmt.Sprintf("web-reader-v1.7.2-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)},
			},
		})
	}))
	defer ts.Close()

	svc := NewService("dev",
		WithAPIBaseURL(ts.URL),
		WithHTTPClient(ts.Client()),
	)

	res, err := svc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if res.CanAutoUpdate {
		t.Errorf("expected CanAutoUpdate=false for dev version")
	}
}

func TestApply_Success(t *testing.T) {
	tempDir := t.TempDir()
	binName := "web-reader"
	if runtime.GOOS == "windows" {
		binName = "web-reader.exe"
	}
	fakeExe := filepath.Join(tempDir, binName)
	if err := os.WriteFile(fakeExe, []byte("old-binary-content"), 0o755); err != nil {
		t.Fatal(err)
	}

	newBinaryContent := []byte("new-binary-content-v1.7.2")
	var archiveBytes []byte
	assetName := fmt.Sprintf("web-reader-v1.7.2-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName = fmt.Sprintf("web-reader-v1.7.2-%s-%s.zip", runtime.GOOS, runtime.GOARCH)
		archiveBytes = createMockZip(t, binName, newBinaryContent)
	} else {
		archiveBytes = createMockTarGz(t, binName, newBinaryContent)
	}

	hasher := sha256.New()
	hasher.Write(archiveBytes)
	checksumHex := hex.EncodeToString(hasher.Sum(nil))

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/GreyRaphael/web_reader/releases/latest":
			release := gitHubRelease{
				TagName: "v1.7.2",
				Assets: []gitHubAsset{
					{
						Name:               assetName,
						BrowserDownloadURL: "/download/asset",
						Size:               int64(len(archiveBytes)),
					},
					{
						Name:               assetName + ".sha256",
						BrowserDownloadURL: "/download/checksum",
						Size:               int64(len(checksumHex)),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(release)
		case "/download/asset":
			_, _ = w.Write(archiveBytes)
		case "/download/checksum":
			_, _ = w.Write([]byte(checksumHex + "  " + assetName))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// Update asset URLs to point to test server
	svc := NewService("v1.7.1",
		WithAPIBaseURL(ts.URL),
		WithHTTPClient(ts.Client()),
		WithExecutablePathFunc(func() (string, error) {
			return fakeExe, nil
		}),
	)

	restarted := false
	applyRes, err := svc.Apply(context.Background(), func() {
		restarted = true
	})
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if applyRes.Status != "success" {
		t.Errorf("expected status=success, got %s", applyRes.Status)
	}
	if !restarted {
		t.Errorf("expected restart callback to be called")
	}

	updatedContent, err := os.ReadFile(fakeExe)
	if err != nil {
		t.Fatalf("read updated executable failed: %v", err)
	}
	if string(updatedContent) != string(newBinaryContent) {
		t.Errorf("executable content mismatch: expected %q, got %q", string(newBinaryContent), string(updatedContent))
	}
}

// TestApply_NoUpdate verifies Apply refuses to reinstall when the current
// version is already up to date, returning ErrNoUpdateFound.
func TestApply_NoUpdate(t *testing.T) {
	tempDir := t.TempDir()
	binName := "web-reader"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	fakeExe := filepath.Join(tempDir, binName)
	if err := os.WriteFile(fakeExe, []byte("current-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	assetName := fmt.Sprintf("web-reader-v1.7.1-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName = fmt.Sprintf("web-reader-v1.7.1-%s-%s.zip", runtime.GOOS, runtime.GOARCH)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(gitHubRelease{
			TagName: "v1.7.1",
			Assets:  []gitHubAsset{{Name: assetName, BrowserDownloadURL: "/download/asset"}},
		})
	}))
	defer ts.Close()

	svc := NewService("v1.7.1",
		WithAPIBaseURL(ts.URL),
		WithHTTPClient(ts.Client()),
		WithExecutablePathFunc(func() (string, error) {
			return fakeExe, nil
		}),
	)

	_, err := svc.Apply(context.Background(), nil)
	if !errors.Is(err, ErrNoUpdateFound) {
		t.Fatalf("Apply with no newer version = %v, want ErrNoUpdateFound", err)
	}
	content, _ := os.ReadFile(fakeExe)
	if string(content) != "current-binary" {
		t.Errorf("binary must remain untouched when no update is available")
	}
}

func TestApply_InvalidChecksum(t *testing.T) {
	tempDir := t.TempDir()
	binName := "web-reader"
	if runtime.GOOS == "windows" {
		binName = "web-reader.exe"
	}
	fakeExe := filepath.Join(tempDir, binName)
	if err := os.WriteFile(fakeExe, []byte("old-binary-content"), 0o755); err != nil {
		t.Fatal(err)
	}

	newBinaryContent := []byte("new-binary-content")
	archiveBytes := createMockTarGz(t, binName, newBinaryContent)
	assetName := fmt.Sprintf("web-reader-v1.7.2-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName = fmt.Sprintf("web-reader-v1.7.2-%s-%s.zip", runtime.GOOS, runtime.GOARCH)
		archiveBytes = createMockZip(t, binName, newBinaryContent)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/GreyRaphael/web_reader/releases/latest":
			_ = json.NewEncoder(w).Encode(gitHubRelease{
				TagName: "v1.7.2",
				Assets: []gitHubAsset{
					{
						Name:               assetName,
						BrowserDownloadURL: "/download/asset",
					},
					{
						Name:               assetName + ".sha256",
						BrowserDownloadURL: "/download/checksum",
					},
				},
			})
		case "/download/asset":
			_, _ = w.Write(archiveBytes)
		case "/download/checksum":
			// Send corrupted checksum
			_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000"))
		}
	}))
	defer ts.Close()

	svc := NewService("v1.7.1",
		WithAPIBaseURL(ts.URL),
		WithHTTPClient(ts.Client()),
		WithExecutablePathFunc(func() (string, error) {
			return fakeExe, nil
		}),
	)

	_, err := svc.Apply(context.Background(), nil)
	if err == nil {
		t.Fatal("expected checksum error, got nil")
	}

	// Verify original file was not modified
	originalContent, _ := os.ReadFile(fakeExe)
	if string(originalContent) != "old-binary-content" {
		t.Errorf("original binary should be untouched")
	}
}

func TestExtract_TarSlip_Prevented(t *testing.T) {
	tempDir := t.TempDir()
	maliciousTar := filepath.Join(tempDir, "malicious.tar.gz")

	// Create archive with path traversal entry
	content := createMockTarGz(t, "../../etc/passwd", []byte("bad-content"))
	if err := os.WriteFile(maliciousTar, content, 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := extractFromTarGz(maliciousTar, "passwd", &out)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestExtract_ZipSlip_Prevented(t *testing.T) {
	tempDir := t.TempDir()
	maliciousZip := filepath.Join(tempDir, "malicious.zip")

	content := createMockZip(t, "../malicious.exe", []byte("bad-content"))
	if err := os.WriteFile(maliciousZip, content, 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := extractFromZip(maliciousZip, "malicious.exe", &out)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}
