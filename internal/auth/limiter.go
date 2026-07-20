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
	if len(kept) >= l.limit {
		entry.times = kept
		l.clients[key] = entry
		return false
	}
	entry.times = append(kept, now)
	l.clients[key] = entry
	if len(l.clients) > 4096 {
		for client, candidate := range l.clients {
			if len(candidate.times) == 0 || candidate.times[len(candidate.times)-1].Before(cutoff) {
				delete(l.clients, client)
			}
		}
	}
	return true
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}
