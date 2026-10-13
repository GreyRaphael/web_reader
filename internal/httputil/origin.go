// Package httputil holds small HTTP helpers shared between the server and
// auth packages.
package httputil

import (
	"net/http"
	"net/url"
	"strings"
)

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

// RequireSameOrigin enforces a strict same-origin check for state-changing
// requests. It verifies the Origin header (or Referer if Origin is omitted).
func RequireSameOrigin(r *http.Request) bool {
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
