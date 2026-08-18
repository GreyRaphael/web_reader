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
	largeImage := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0xab}, (2<<20)-8)...)
	if err := os.WriteFile(filepath.Join(root, "large-image.png"), largeImage, 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := workspacefs.New(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("reader-test"), 10)
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
	request.Header.Set("Origin", "http://example.com")
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
	sessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(sessionResponse, authenticatedRequest(http.MethodGet, "/api/auth/session", cookie))
	if sessionResponse.Code != http.StatusOK || !strings.Contains(sessionResponse.Body.String(), `"authenticated":true`) {
		t.Fatalf("session status = %d, body = %s", sessionResponse.Code, sessionResponse.Body.String())
	}

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

	largeRequest := authenticatedRequest(http.MethodGet, "/api/fs/raw?path=large-image.png", cookie)
	largeRequest.Header.Set("Range", "bytes=1048576-1048591")
	largeResponse := httptest.NewRecorder()
	handler.ServeHTTP(largeResponse, largeRequest)
	if largeResponse.Code != http.StatusPartialContent || largeResponse.Body.Len() != 16 {
		t.Fatalf("large image range status = %d, bytes = %d", largeResponse.Code, largeResponse.Body.Len())
	}
	if largeResponse.Header().Get("Content-Type") != "image/png" || !strings.HasPrefix(largeResponse.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("large image headers = %#v", largeResponse.Header())
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
	logoutRequest.Header.Set("Origin", "http://example.com")
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

func TestWorkspaceAPI(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	cookie := loginCookie(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/api/workspace", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"workspace":`) {
		t.Fatalf("GET /api/workspace status = %d, body = %s", rec.Code, rec.Body.String())
	}

	newDir := t.TempDir()
	postReq := httptest.NewRequest(http.MethodPost, "/api/workspace", strings.NewReader(`{"workspace":"`+newDir+`"}`))
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Origin", "http://example.com")
	postReq.AddCookie(cookie)
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK || !strings.Contains(postRec.Body.String(), newDir) {
		t.Fatalf("POST /api/workspace status = %d, body = %s", postRec.Code, postRec.Body.String())
	}
}

func TestBrowseHandlerAuthAndListing(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	// Unauthenticated request is rejected.
	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, httptest.NewRequest(http.MethodGet, "/api/fs/browse?path=/tmp", nil))
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated browse status = %d", unauth.Code)
	}

	cookie := loginCookie(t, handler)

	browseDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(browseDir, "sub1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(browseDir, "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(browseDir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	req := authenticatedRequest(http.MethodGet, "/api/fs/browse?path="+browseDir, cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("browse status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"sub1"`) || !strings.Contains(body, `"sub2"`) {
		t.Fatalf("browse body missing subdirs: %s", body)
	}
	if strings.Contains(body, `"file.txt"`) {
		t.Fatalf("browse body should not contain files: %s", body)
	}
}

func TestBrowseHandlerRejectsSensitivePath(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	cookie := loginCookie(t, handler)

	req := authenticatedRequest(http.MethodGet, "/api/fs/browse?path=/etc", cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("browse /etc status = %d, want 403", rec.Code)
	}
}

func newTestServerWithTerminal(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	files, err := workspacefs.New(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("reader-test"), 10)
	if err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewStore(time.Hour, false)
	authHandler := auth.NewHandler("admin", hash, sessions, auth.NewLoginLimiter(20, time.Minute))
	assets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<!doctype html><div id="app"></div>`)},
	}
	return New(Config{
		AppConfig:       config.Config{Addr: "127.0.0.1:0"},
		Auth:            authHandler,
		Sessions:        sessions,
		Files:           files,
		Assets:          fs.FS(assets),
		TerminalEnabled: true,
	})
}

func TestTerminalDisabledReturns404(t *testing.T) {
	handler := newTestServer(t).Handler()
	cookie := loginCookie(t, handler)
	rec := httptest.NewRecorder()
	req := authenticatedRequest(http.MethodGet, "/api/terminal", cookie)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("terminal route should be 404 when disabled, got %d", rec.Code)
	}
}

func TestTerminalRequiresAuth(t *testing.T) {
	handler := newTestServerWithTerminal(t).Handler()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/terminal", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated terminal status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestSettingsAPI(t *testing.T) {
	handler := newTestServerWithTerminal(t).Handler()
	cookie := loginCookie(t, handler)

	// GET returns current setting (true by default in test config)
	getReq := authenticatedRequest(http.MethodGet, "/api/settings", cookie)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK || !strings.Contains(getRec.Body.String(), `"enableTerminal":true`) {
		t.Fatalf("GET /api/settings status = %d, body = %s", getRec.Code, getRec.Body.String())
	}
}

func TestCSRFProtectionOnWriteOperations(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	cookie := loginCookie(t, handler)

	testCases := []struct {
		name       string
		method     string
		target     string
		body       string
		origin     string
		referer    string
		wantStatus int
	}{
		{
			name:       "POST settings with matching origin allowed",
			method:     http.MethodPost,
			target:     "/api/settings",
			body:       `{"enableTerminal":true}`,
			origin:     "http://example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST settings with matching referer allowed",
			method:     http.MethodPost,
			target:     "/api/settings",
			body:       `{"enableTerminal":true}`,
			referer:    "http://example.com/settings",
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST settings with mismatched origin rejected",
			method:     http.MethodPost,
			target:     "/api/settings",
			body:       `{"enableTerminal":false}`,
			origin:     "http://evil.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "POST settings with missing origin rejected",
			method:     http.MethodPost,
			target:     "/api/settings",
			body:       `{"enableTerminal":false}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "POST create file with mismatched origin rejected",
			method:     http.MethodPost,
			target:     "/api/fs/file",
			body:       `{"path":"csrf.txt"}`,
			origin:     "http://evil.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "POST create file with matching origin allowed",
			method:     http.MethodPost,
			target:     "/api/fs/file",
			body:       `{"path":"legit.txt"}`,
			origin:     "http://example.com",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "DELETE file with mismatched origin rejected",
			method:     http.MethodDelete,
			target:     "/api/fs/delete?path=legit.txt",
			origin:     "http://attacker.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "DELETE file with matching origin allowed",
			method:     http.MethodDelete,
			target:     "/api/fs/delete?path=legit.txt",
			origin:     "http://example.com",
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST workspace switch with mismatched origin rejected",
			method:     http.MethodPost,
			target:     "/api/workspace",
			body:       `{"workspace":"/tmp"}`,
			origin:     "http://evil.com",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader *strings.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			} else {
				bodyReader = strings.NewReader("")
			}
			req := httptest.NewRequest(tc.method, tc.target, bodyReader)
			req.AddCookie(cookie)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d, body: %s", tc.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTerminalCSWSHProtection(t *testing.T) {
	handler := newTestServerWithTerminal(t).Handler()
	cookie := loginCookie(t, handler)

	// Missing origin is rejected with 403 Forbidden
	reqMissingOrigin := authenticatedRequest(http.MethodGet, "/api/terminal", cookie)
	recMissing := httptest.NewRecorder()
	handler.ServeHTTP(recMissing, reqMissingOrigin)
	if recMissing.Code != http.StatusForbidden {
		t.Fatalf("terminal without origin got %d, want 403 Forbidden", recMissing.Code)
	}

	// Mismatched cross-site origin is rejected with 403 Forbidden
	reqCrossSite := authenticatedRequest(http.MethodGet, "/api/terminal", cookie)
	reqCrossSite.Header.Set("Origin", "http://evil.com")
	recCrossSite := httptest.NewRecorder()
	handler.ServeHTTP(recCrossSite, reqCrossSite)
	if recCrossSite.Code != http.StatusForbidden {
		t.Fatalf("terminal with evil origin got %d, want 403 Forbidden", recCrossSite.Code)
	}
}
