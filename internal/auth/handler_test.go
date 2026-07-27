package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const testBcryptCost = 10

func TestLoginSessionAndLogout(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), testBcryptCost)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(time.Hour, false)
	handler := NewHandler("admin", hash, store, NewLoginLimiter(10, time.Minute))

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"correct horse"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRequest.Header.Set("Origin", "http://example.com")
	loginResponse := httptest.NewRecorder()
	handler.Login(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != CookieName || !cookies[0].HttpOnly {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}

	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	sessionRequest.AddCookie(cookies[0])
	sessionResponse := httptest.NewRecorder()
	handler.Session(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK || !bytes.Contains(sessionResponse.Body.Bytes(), []byte(`"authenticated":true`)) {
		t.Fatalf("session response = %d %s", sessionResponse.Code, sessionResponse.Body.String())
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutRequest.AddCookie(cookies[0])
	logoutRequest.Header.Set("Origin", "http://example.com")
	logoutResponse := httptest.NewRecorder()
	handler.Logout(logoutResponse, logoutRequest)
	if _, ok := store.Get(sessionRequest); ok {
		t.Fatal("session remains after logout")
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), testBcryptCost)
	handler := NewHandler("admin", hash, NewStore(time.Hour, false), NewLoginLimiter(10, time.Minute))
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"wrong"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://example.com")
	response := httptest.NewRecorder()
	handler.Login(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestLoginRejectsMissingOrigin(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), testBcryptCost)
	handler := NewHandler("admin", hash, NewStore(time.Hour, false), NewLoginLimiter(10, time.Minute))
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Login(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for missing origin, status = %d", response.Code)
	}
}

func TestLoginLimiterLocksAfterFailures(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), testBcryptCost)
	handler := NewHandler("admin", hash, NewStore(time.Hour, false), NewLoginLimiter(3, time.Minute))
	for i := 0; i < 3; i++ {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"wrong"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "http://example.com")
		response := httptest.NewRecorder()
		handler.Login(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", i, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://example.com")
	response := httptest.NewRecorder()
	handler.Login(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("expected rate limit after 3 failures, status = %d", response.Code)
	}
}
