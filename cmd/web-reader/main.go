package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"

	"web_reader/internal/auth"
	"web_reader/internal/config"
	workspacefs "web_reader/internal/filesystem"
	"web_reader/internal/server"
	"web_reader/internal/webui"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hash-password" {
		fmt.Fprint(os.Stderr, "Password: ")
		if err := hashPassword(os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "hash password:", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuration error:", err)
		os.Exit(2)
	}
	files, err := workspacefs.New(cfg.Workspace, cfg.MaxTextSize)
	if err != nil {
		fmt.Fprintln(os.Stderr, "workspace error:", err)
		os.Exit(2)
	}
	assets, err := webui.Dist()
	if err != nil {
		fmt.Fprintln(os.Stderr, "frontend assets error:", err)
		os.Exit(2)
	}

	sessions := auth.NewStore(cfg.SessionTTL, cfg.SecureCookie)
	authHandler := auth.NewHandler(cfg.Username, cfg.PasswordHash, sessions, auth.NewLoginLimiter(10, time.Minute))
	app := server.New(server.Config{
		AppConfig: cfg,
		Auth:      authHandler,
		Sessions:  sessions,
		Files:     files,
		Assets:    assets,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go sessions.RunCleanup(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := app.HTTPServer().Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()

	slog.Info("web reader starting", "addr", cfg.Addr, "workspace", cfg.Workspace, "admin_user", cfg.Username)
	if err := app.HTTPServer().ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("web reader stopped unexpectedly", "error", err)
		os.Exit(1)
	}
	slog.Info("web reader stopped")
}

func hashPassword(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), 64<<10)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}
		return errors.New("no password provided")
	}
	password := strings.TrimSuffix(scanner.Text(), "\r")
	if password == "" {
		return errors.New("password cannot be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, string(hash))
	return err
}
