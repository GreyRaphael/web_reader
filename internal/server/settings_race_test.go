package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// TestTerminalToggleRace exercises the shared terminalEnabled state: one
// goroutine writes via POST /api/settings while others read via
// GET /api/settings and GET /api/terminal. Must be run with -race.
func TestTerminalToggleRace(t *testing.T) {
	app := newTestServer(t)
	handler := app.Handler()
	cookie := loginCookie(t, handler)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				req := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(`{"enableTerminal": true}`))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Origin", "http://example.com")
				req.AddCookie(cookie)
				handler.ServeHTTP(httptest.NewRecorder(), req)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
				req.AddCookie(cookie)
				handler.ServeHTTP(httptest.NewRecorder(), req)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				req := httptest.NewRequest(http.MethodGet, "/api/terminal", nil)
				req.AddCookie(cookie)
				handler.ServeHTTP(httptest.NewRecorder(), req)
			}
		}()
	}
	wg.Wait()
}
