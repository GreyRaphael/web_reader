package auth

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type attemptWindow struct {
	times []time.Time
}

type LoginLimiter struct {
	mu      sync.Mutex
	clients map[string]attemptWindow
	limit   int
	window  time.Duration
	now     func() time.Time
}

func NewLoginLimiter(limit int, window time.Duration) *LoginLimiter {
	return &LoginLimiter{
		clients: make(map[string]attemptWindow),
		limit:   limit,
		window:  window,
		now:     time.Now,
	}
}

// Allow reports whether the client is permitted to attempt a login.
// It prunes expired entries but does not record a new attempt; call
// RecordFailure after a failed credential check.
func (l *LoginLimiter) Allow(r *http.Request) bool {
	key := clientIP(r.RemoteAddr)
	now := l.now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.clients[key]
	kept := entry.times[:0]
	for _, timestamp := range entry.times {
		if timestamp.After(cutoff) {
			kept = append(kept, timestamp)
		}
	}
	entry.times = kept
	l.clients[key] = entry
	if len(l.clients) > 4096 {
		for client, candidate := range l.clients {
			if len(candidate.times) == 0 || candidate.times[len(candidate.times)-1].Before(cutoff) {
				delete(l.clients, client)
			}
		}
	}
	return len(kept) < l.limit
}

// RecordFailure records a failed login attempt for the client.
func (l *LoginLimiter) RecordFailure(r *http.Request) {
	key := clientIP(r.RemoteAddr)
	now := l.now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	entry := l.clients[key]
	kept := entry.times[:0]
	for _, timestamp := range entry.times {
		if timestamp.After(cutoff) {
			kept = append(kept, timestamp)
		}
	}
	kept = append(kept, now)
	entry.times = kept
	l.clients[key] = entry
}

// Window returns the configured rate-limit window, for use in Retry-After.
func (l *LoginLimiter) Window() time.Duration {
	return l.window
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}
