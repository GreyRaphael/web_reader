package auth

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
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
	if !requireSameOrigin(r) {
		writeJSONError(w, http.StatusForbidden, "invalid_origin", "Request origin is not allowed")
		return
	}
	if !h.limiter.Allow(r) {
		w.Header().Set("Retry-After", strconv.Itoa(int(h.limiter.Window().Seconds())))
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
		h.limiter.RecordFailure(r)
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
	if !requireSameOrigin(r) {
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

// matchHost compares two host strings, ignoring case and normalizing standard default ports.
func matchHost(originHost, targetHost string) bool {
	if strings.EqualFold(originHost, targetHost) {
		return true
	}
	cleanOrigin := originHost
	cleanTarget := targetHost
	for _, suffix := range []string{":80", ":443"} {
		cleanOrigin = strings.TrimSuffix(cleanOrigin, suffix)
		cleanTarget = strings.TrimSuffix(cleanTarget, suffix)
	}
	return strings.EqualFold(cleanOrigin, cleanTarget)
}

// requireSameOrigin enforces a strict same-origin check for state-changing
// requests. It verifies the Origin header (or Referer if Origin is omitted).
func requireSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" {
			return false
		}
		return matchHost(parsed.Host, r.Host)
	}
	referer := r.Header.Get("Referer")
	if referer != "" {
		parsed, err := url.Parse(referer)
		if err != nil || parsed.Host == "" {
			return false
		}
		return matchHost(parsed.Host, r.Host)
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
