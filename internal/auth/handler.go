package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	username     string
	passwordHash []byte
	store        *Store
	limiter      *LoginLimiter
}

func NewHandler(username string, passwordHash []byte, store *Store, limiter *LoginLimiter) *Handler {
	return &Handler{username: username, passwordHash: passwordHash, store: store, limiter: limiter}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeJSONError(w, http.StatusForbidden, "invalid_origin", "Request origin is not allowed")
		return
	}
	if !h.limiter.Allow(r) {
		w.Header().Set("Retry-After", "60")
		writeJSONError(w, http.StatusTooManyRequests, "rate_limited", "Too many login attempts")
		return
	}
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		writeJSONError(w, http.StatusUnsupportedMediaType, "invalid_request", "Content-Type must be application/json")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request loginRequest
	if err := decoder.Decode(&request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "Invalid login request")
		return
	}
	usernameOK := subtle.ConstantTimeCompare([]byte(request.Username), []byte(h.username)) == 1
	passwordOK := bcrypt.CompareHashAndPassword(h.passwordHash, []byte(request.Password)) == nil
	if !usernameOK || !passwordOK {
		writeJSONError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
		return
	}
	if err := h.store.Create(w, h.username); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal_error", "Unable to create session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": h.username})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		writeJSONError(w, http.StatusForbidden, "invalid_origin", "Request origin is not allowed")
		return
	}
	h.store.Destroy(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	if session, ok := h.store.Get(r); ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "username": session.Username})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
