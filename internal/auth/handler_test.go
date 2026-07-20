package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginSessionAndLogout(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(time.Hour, false)
	handler := NewHandler("admin", hash, store, NewLoginLimiter(10, time.Minute))

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"correct horse"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
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
	logoutResponse := httptest.NewRecorder()
	handler.Logout(logoutResponse, logoutRequest)
	if _, ok := store.Get(sessionRequest); ok {
		t.Fatal("session remains after logout")
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	handler := NewHandler("admin", hash, NewStore(time.Hour, false), NewLoginLimiter(10, time.Minute))
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"wrong"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Login(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}
