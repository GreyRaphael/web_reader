package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	Addr         string
	Workspace    string
	Username     string
	PasswordHash []byte
	SessionTTL   time.Duration
	MaxTextSize  int64
	SecureCookie bool
}

func Parse(args []string) (Config, error) {
	fs := flag.NewFlagSet("web-reader", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var cfg Config
	var sessionTTL string
	var maxTextSize string
	fs.StringVar(&cfg.Addr, "addr", envOr("WEB_READER_ADDR", "0.0.0.0:8848"), "HTTP listen address")
	fs.StringVar(&cfg.Workspace, "workspace", os.Getenv("WEB_READER_WORKSPACE"), "workspace directory")
	fs.StringVar(&cfg.Username, "admin-user", envOr("WEB_READER_ADMIN_USERNAME", "admin"), "administrator username")
	passwordHash := os.Getenv("WEB_READER_ADMIN_PASSWORD_HASH")
	fs.StringVar(&passwordHash, "password-hash", passwordHash, "administrator bcrypt password hash")
	fs.StringVar(&sessionTTL, "session-ttl", envOr("WEB_READER_SESSION_TTL", "24h"), "session lifetime")
	fs.StringVar(&maxTextSize, "max-text-size", envOr("WEB_READER_MAX_TEXT_SIZE", "10MiB"), "maximum text preview size")
	fs.BoolVar(&cfg.SecureCookie, "secure-cookie", envBool("WEB_READER_SECURE_COOKIE", false), "mark session cookie Secure")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg.Username = strings.TrimSpace(cfg.Username)
	if cfg.Username == "" {
		return Config{}, errors.New("admin username cannot be empty")
	}
	if passwordHash == "" {
		return Config{}, errors.New("password hash is required (set WEB_READER_ADMIN_PASSWORD_HASH or --password-hash)")
	}
	if _, err := bcrypt.Cost([]byte(passwordHash)); err != nil {
		return Config{}, fmt.Errorf("invalid bcrypt password hash: %w", err)
	}
	cfg.PasswordHash = []byte(passwordHash)

	var err error
	cfg.SessionTTL, err = time.ParseDuration(sessionTTL)
	if err != nil || cfg.SessionTTL <= 0 {
		return Config{}, fmt.Errorf("invalid session TTL %q", sessionTTL)
	}
	cfg.MaxTextSize, err = parseBytes(maxTextSize)
	if err != nil || cfg.MaxTextSize <= 0 {
		return Config{}, fmt.Errorf("invalid maximum text size %q", maxTextSize)
	}

	if strings.TrimSpace(cfg.Workspace) == "" {
		return Config{}, errors.New("workspace is required (set WEB_READER_WORKSPACE or --workspace)")
	}
	root, err := filepath.Abs(cfg.Workspace)
	if err != nil {
		return Config{}, fmt.Errorf("resolve workspace: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return Config{}, fmt.Errorf("resolve workspace symlinks: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return Config{}, fmt.Errorf("stat workspace: %w", err)
	}
	if !info.IsDir() {
		return Config{}, errors.New("workspace must be a directory")
	}
	cfg.Workspace = filepath.Clean(root)
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseBytes(value string) (int64, error) {
	s := strings.TrimSpace(strings.ToUpper(value))
	multipliers := []struct {
		suffix string
		value  int64
	}{
		{"GIB", 1 << 30}, {"MIB", 1 << 20}, {"KIB", 1 << 10},
		{"GB", 1_000_000_000}, {"MB", 1_000_000}, {"KB", 1_000},
		{"B", 1},
	}
	for _, item := range multipliers {
		if strings.HasSuffix(s, item.suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(s, item.suffix))
			parsed, err := strconv.ParseFloat(number, 64)
			if err != nil {
				return 0, err
			}
			return int64(parsed * float64(item.value)), nil
		}
	}
	return strconv.ParseInt(s, 10, 64)
}
