package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const CookieName = "web_reader_session"

type Session struct {
	Username  string
	ExpiresAt time.Time
}

type Store struct {
	mu           sync.RWMutex
	sessions     map[[sha256.Size]byte]Session
	ttl          time.Duration
	secureCookie bool
	now          func() time.Time
}

func NewStore(ttl time.Duration, secureCookie bool) *Store {
	return &Store{
		sessions:     make(map[[sha256.Size]byte]Session),
		ttl:          ttl,
		secureCookie: secureCookie,
		now:          time.Now,
	}
}

func (s *Store) Create(w http.ResponseWriter, username string) error {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	digest := sha256.Sum256([]byte(token))
	expires := s.now().Add(s.ttl)
	s.mu.Lock()
	s.sessions[digest] = Session{Username: username, ExpiresAt: expires}
	s.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(s.ttl.Seconds()),
		HttpOnly: true,
		Secure:   s.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (s *Store) Get(r *http.Request) (Session, bool) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return Session{}, false
	}
	digest := sha256.Sum256([]byte(cookie.Value))
	s.mu.RLock()
	session, ok := s.sessions[digest]
	s.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if !session.ExpiresAt.After(s.now()) {
		s.mu.Lock()
		delete(s.sessions, digest)
		s.mu.Unlock()
		return Session{}, false
	}
	return session, true
}

func (s *Store) Destroy(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(CookieName); err == nil {
		digest := sha256.Sum256([]byte(cookie.Value))
		s.mu.Lock()
		delete(s.sessions, digest)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Store) Cleanup() {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, session := range s.sessions {
		if !session.ExpiresAt.After(now) {
			delete(s.sessions, key)
		}
	}
}

func (s *Store) RunCleanup(ctx context.Context) {
	interval := minDuration(s.ttl/2, 30*time.Minute)
	if interval < time.Minute {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Cleanup()
		}
	}
}

func (s *Store) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.Get(r); !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
