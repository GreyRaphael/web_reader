package server

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"golang.org/x/crypto/bcrypt"

	"web_reader/internal/auth"
	"web_reader/internal/config"
	workspacefs "web_reader/internal/filesystem"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "readme.md"), []byte("# Reader\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "image.png"), []byte("\x89PNG\r\n\x1a\nfixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "page.html"), []byte("<script>alert(1)</script>"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := workspacefs.New(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("reader-test"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewStore(time.Hour, false)
	authHandler := auth.NewHandler("admin", hash, sessions, auth.NewLoginLimiter(20, time.Minute))
	assets := fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte(`<!doctype html><div id="app"></div>`)},
		"assets/app.js": &fstest.MapFile{Data: []byte(`console.log("reader")`)},
	}
	return New(Config{
		AppConfig: config.Config{Addr: "127.0.0.1:0"},
		Auth:      authHandler,
		Sessions:  sessions,
		Files:     files,
		Assets:    fs.FS(assets),
	})
}

func loginCookie(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"reader-test"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies = %d", len(cookies))
	}
	return cookies[0]
}

func authenticatedRequest(method, target string, cookie *http.Cookie) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.AddCookie(cookie)
	return request
}

func TestProtectedFilesystemFlow(t *testing.T) {
	app := newTestServer(t)
	handler := app.Handler()

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/fs/list?path=", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", unauthorized.Code)
	}

	cookie := loginCookie(t, handler)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, authenticatedRequest(http.MethodGet, "/api/fs/list?path=", cookie))
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "readme.md") {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}

	textResponse := httptest.NewRecorder()
	handler.ServeHTTP(textResponse, authenticatedRequest(http.MethodGet, "/api/fs/text?path=readme.md", cookie))
	if textResponse.Code != http.StatusOK || !strings.Contains(textResponse.Body.String(), "# Reader") {
		t.Fatalf("text status = %d, body = %s", textResponse.Code, textResponse.Body.String())
	}

	rangeRequest := authenticatedRequest(http.MethodGet, "/api/fs/raw?path=readme.md", cookie)
	rangeRequest.Header.Set("Range", "bytes=0-3")
	rangeResponse := httptest.NewRecorder()
	handler.ServeHTTP(rangeResponse, rangeRequest)
	if rangeResponse.Code != http.StatusPartialContent || rangeResponse.Body.String() != "# Re" {
		t.Fatalf("range status = %d, body = %q", rangeResponse.Code, rangeResponse.Body.String())
	}
	if rangeResponse.Header().Get("Accept-Ranges") != "bytes" || rangeResponse.Header().Get("ETag") == "" {
		t.Fatalf("range headers = %#v", rangeResponse.Header())
	}

	imageResponse := httptest.NewRecorder()
	handler.ServeHTTP(imageResponse, authenticatedRequest(http.MethodGet, "/api/fs/raw?path=image.png", cookie))
	if !strings.HasPrefix(imageResponse.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("image disposition = %q", imageResponse.Header().Get("Content-Disposition"))
	}

	htmlResponse := httptest.NewRecorder()
	handler.ServeHTTP(htmlResponse, authenticatedRequest(http.MethodGet, "/api/fs/raw?path=page.html", cookie))
	if !strings.HasPrefix(htmlResponse.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("html disposition = %q", htmlResponse.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(htmlResponse.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("raw CSP = %q", htmlResponse.Header().Get("Content-Security-Policy"))
	}

	logoutRequest := authenticatedRequest(http.MethodPost, "/api/auth/logout", cookie)
	logoutRequest.Header.Set("Content-Type", "application/json")
	logoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusOK {
		t.Fatalf("logout status = %d", logoutResponse.Code)
	}

	afterLogout := httptest.NewRecorder()
	handler.ServeHTTP(afterLogout, authenticatedRequest(http.MethodGet, "/api/fs/list?path=", cookie))
	if afterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("status after logout = %d", afterLogout.Code)
	}
}

func TestSecurityHeadersAndSPAFallback(t *testing.T) {
	handler := newTestServer(t).Handler()

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d", health.Code)
	}
	for _, header := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if health.Header().Get(header) == "" {
			t.Errorf("missing security header %s", header)
		}
	}

	spa := httptest.NewRecorder()
	handler.ServeHTTP(spa, httptest.NewRequest(http.MethodGet, "/book/chapter", nil))
	body, _ := io.ReadAll(spa.Result().Body)
	if spa.Code != http.StatusOK || !bytes.Contains(body, []byte(`id="app"`)) {
		t.Fatalf("SPA status = %d, body = %s", spa.Code, body)
	}

	apiMissing := httptest.NewRecorder()
	handler.ServeHTTP(apiMissing, httptest.NewRequest(http.MethodGet, "/api/missing", nil))
	if apiMissing.Code != http.StatusNotFound || !strings.Contains(apiMissing.Body.String(), `"code":"not_found"`) {
		t.Fatalf("API fallback status = %d, body = %s", apiMissing.Code, apiMissing.Body.String())
	}
}
