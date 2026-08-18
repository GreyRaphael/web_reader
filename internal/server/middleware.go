package server

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"strings"
	"time"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.WriteHeader(http.StatusOK)
	}
	count, err := r.ResponseWriter.Write(body)
	r.bytes += count
	return count, err
}

func (r *responseRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Hijack delegates to the underlying ResponseWriter so that WebSocket
// upgrade requests can take over the TCP connection.
func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
	}
	return hj.Hijack()
}

// Flush delegates to the underlying ResponseWriter if it supports flushing.
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

type bodyTrackingWriter struct {
	http.ResponseWriter
	started bool
}

func (b *bodyTrackingWriter) WriteHeader(status int) {
	b.ResponseWriter.WriteHeader(status)
}

func (b *bodyTrackingWriter) Write(body []byte) (int, error) {
	b.started = true
	return b.ResponseWriter.Write(body)
}

func (b *bodyTrackingWriter) Unwrap() http.ResponseWriter { return b.ResponseWriter }

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" || !requestIDPattern.MatchString(requestID) {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		slog.Info("http request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", recorder.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("http handler panic", "error", recovered, "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func securityHeadersWithHSTS(next http.Handler) http.Handler {
	base := securityHeaders(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		base.ServeHTTP(w, r)
	})
}

func newRequestID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(raw[:])
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

// requireSameOrigin enforces same-origin validation.
// It verifies the Origin header, falling back to Referer if Origin is omitted.
func requireSameOrigin(r *http.Request) bool {
	rawOrigin := r.Header.Get("Origin")
	if rawOrigin != "" {
		parsed, err := url.Parse(rawOrigin)
		if err != nil || parsed.Host == "" {
			return false
		}
		return matchHost(parsed.Host, r.Host)
	}
	rawReferer := r.Header.Get("Referer")
	if rawReferer != "" {
		parsed, err := url.Parse(rawReferer)
		if err != nil || parsed.Host == "" {
			return false
		}
		return matchHost(parsed.Host, r.Host)
	}
	return false
}

// csrfProtection enforces same-origin checks on all state-modifying HTTP methods
// (POST, PUT, PATCH, DELETE) to protect against CSRF attacks.
func csrfProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if !requireSameOrigin(r) {
				writeError(w, http.StatusForbidden, "invalid_origin", "Request origin is not allowed")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
