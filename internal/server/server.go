package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"web_reader/internal/auth"
	"web_reader/internal/config"
	workspacefs "web_reader/internal/filesystem"
)

type Server struct {
	config Config
	http   *http.Server
}

type Config struct {
	AppConfig config.Config
	Auth      *auth.Handler
	Sessions  *auth.Store
	Files     *workspacefs.Service
	Assets    fs.FS
}

func New(cfg Config) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/auth/login", cfg.Auth.Login)
	mux.Handle("POST /api/auth/logout", cfg.Sessions.Require(http.HandlerFunc(cfg.Auth.Logout)))
	mux.HandleFunc("GET /api/auth/session", cfg.Auth.Session)

	mux.Handle("GET /api/fs/list", cfg.Sessions.Require(http.HandlerFunc(listHandler(cfg.Files))))
	mux.Handle("GET /api/fs/meta", cfg.Sessions.Require(http.HandlerFunc(metaHandler(cfg.Files))))
	mux.Handle("GET /api/fs/text", cfg.Sessions.Require(http.HandlerFunc(textHandler(cfg.Files))))
	mux.Handle("GET /api/fs/raw", cfg.Sessions.Require(http.HandlerFunc(rawHandler(cfg.Files))))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "API route not found")
	})
	mux.Handle("/", spaHandler(cfg.Assets))

	handler := securityHeaders(recoverPanic(requestLogger(mux)))
	return &Server{
		config: cfg,
		http: &http.Server{
			Addr:              cfg.AppConfig.Addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      5 * time.Minute,
			IdleTimeout:       90 * time.Second,
			MaxHeaderBytes:    1 << 20,
		},
	}
}

func (s *Server) HTTPServer() *http.Server { return s.http }
func (s *Server) Handler() http.Handler    { return s.http.Handler }

func listHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := service.List(r.URL.Query().Get("path"))
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

func metaHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, err := service.Info(r.URL.Query().Get("path"))
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"item": item})
	}
}

func textHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, err := service.ReadText(r.URL.Query().Get("path"))
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, file)
	}
}

func rawHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file, info, item, err := service.Open(r.URL.Query().Get("path"))
		if err != nil {
			writeFileError(w, err)
			return
		}
		defer file.Close()

		mimeType := strings.TrimSpace(strings.Split(item.MIME, ";")[0])
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", mimeType)
		disposition := "attachment"
		if r.URL.Query().Get("download") != "1" && workspacefs.IsInlineImage(mimeType) {
			disposition = "inline"
		}
		if formatted := mime.FormatMediaType(disposition, map[string]string{"filename": item.Name}); formatted != "" {
			w.Header().Set("Content-Disposition", formatted)
		}
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
		w.Header().Set("Cache-Control", "private, max-age=60")
		w.Header().Set("ETag", `W/"`+strconv.FormatInt(info.Size(), 16)+"-"+strconv.FormatInt(info.ModTime().UnixNano(), 16)+`"`)
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	}
}

func writeFileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workspacefs.ErrInvalidPath):
		writeError(w, http.StatusBadRequest, "invalid_path", "Invalid workspace path")
	case errors.Is(err, workspacefs.ErrOutsideRoot):
		writeError(w, http.StatusForbidden, "outside_workspace", "Path is outside workspace")
	case errors.Is(err, workspacefs.ErrNotDirectory):
		writeError(w, http.StatusBadRequest, "not_a_directory", "Path is not a directory")
	case errors.Is(err, workspacefs.ErrNotFile):
		writeError(w, http.StatusBadRequest, "not_a_file", "Path is not a file")
	case errors.Is(err, workspacefs.ErrFileTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "File exceeds the configured preview limit")
	case errors.Is(err, workspacefs.ErrInvalidEncoding):
		writeError(w, http.StatusUnsupportedMediaType, "invalid_text_encoding", "File is not valid UTF-8")
	case errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, "not_found", "File or directory not found")
	case errors.Is(err, os.ErrPermission):
		writeError(w, http.StatusForbidden, "permission_denied", "Permission denied")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Unable to access workspace entry")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"code": code, "message": message})
}
