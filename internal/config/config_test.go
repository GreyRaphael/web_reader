package config

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func validHash(t *testing.T) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("reader-test"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"WEB_READER_ADDR",
		"WEB_READER_WORKSPACE",
		"WEB_READER_ADMIN_USERNAME",
		"WEB_READER_ADMIN_PASSWORD_HASH",
		"WEB_READER_SESSION_TTL",
		"WEB_READER_MAX_TEXT_SIZE",
		"WEB_READER_SECURE_COOKIE",
	} {
		t.Setenv(key, "")
	}
}

func TestParseDefaultsAndFlagOverrides(t *testing.T) {
	clearConfigEnvironment(t)
	workspace := t.TempDir()
	hash := validHash(t)

	cfg, err := Parse([]string{"--workspace", workspace, "--password-hash", hash})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "0.0.0.0:8848" || cfg.Username != "admin" {
		t.Fatalf("defaults = addr %q, username %q", cfg.Addr, cfg.Username)
	}
	if cfg.SessionTTL != 24*time.Hour || cfg.MaxTextSize != 10<<20 {
		t.Fatalf("defaults = ttl %s, max text %d", cfg.SessionTTL, cfg.MaxTextSize)
	}
	absolute, _ := filepath.Abs(workspace)
	if cfg.Workspace != filepath.Clean(absolute) {
		t.Fatalf("workspace = %q, want %q", cfg.Workspace, absolute)
	}

	cfg, err = Parse([]string{
		"--addr", "127.0.0.1:9000",
		"--workspace", workspace,
		"--admin-user", "reader",
		"--password-hash", hash,
		"--session-ttl", "2h",
		"--max-text-size", "1.5MiB",
		"--secure-cookie",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != "127.0.0.1:9000" || cfg.Username != "reader" || cfg.SessionTTL != 2*time.Hour || cfg.MaxTextSize != 1572864 || !cfg.SecureCookie {
		t.Fatalf("flag overrides = %#v", cfg)
	}
}

func TestParseRejectsInvalidRequiredValues(t *testing.T) {
	clearConfigEnvironment(t)
	workspace := t.TempDir()
	hash := validHash(t)

	for name, args := range map[string][]string{
		"invalid hash": {"--workspace", workspace, "--password-hash", "plaintext"},
		"invalid ttl":  {"--workspace", workspace, "--password-hash", hash, "--session-ttl", "0s"},
		"invalid size": {"--workspace", workspace, "--password-hash", hash, "--max-text-size", "nope"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(args); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}

func TestParseOptionalWorkspaceDefaults(t *testing.T) {
	clearConfigEnvironment(t)
	hash := validHash(t)

	cfg, err := Parse([]string{"--password-hash", hash})
	if err != nil {
		t.Fatalf("expected parse success when workspace omitted: %v", err)
	}
	if cfg.Workspace == "" {
		t.Fatal("expected non-empty default workspace")
	}
}
