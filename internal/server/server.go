package server

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"web_reader/internal/auth"
	"web_reader/internal/config"
	workspacefs "web_reader/internal/filesystem"
	"web_reader/internal/terminal"
)

type Server struct {
	config          Config
	http            *http.Server
	terminalEnabled *bool
}

type Config struct {
	AppConfig       config.Config
	Auth            *auth.Handler
	Sessions        *auth.Store
	Files           *workspacefs.Service
	Assets          fs.FS
	TerminalEnabled bool
}

func New(cfg Config) *Server {
	terminalEnabled := cfg.TerminalEnabled
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/auth/login", cfg.Auth.Login)
	mux.Handle("POST /api/auth/logout", cfg.Sessions.Require(http.HandlerFunc(cfg.Auth.Logout)))
	mux.HandleFunc("GET /api/auth/session", cfg.Auth.Session)
	mux.Handle("GET /api/workspace", cfg.Sessions.Require(http.HandlerFunc(getWorkspaceHandler(cfg.Files))))
	mux.Handle("POST /api/workspace", cfg.Sessions.Require(http.HandlerFunc(setWorkspaceHandler(cfg.Files))))
	mux.Handle("GET /api/fs/list", cfg.Sessions.Require(http.HandlerFunc(listHandler(cfg.Files))))
	mux.Handle("GET /api/fs/browse", cfg.Sessions.Require(http.HandlerFunc(browseHandler())))
	mux.Handle("GET /api/terminal", cfg.Sessions.Require(terminalHandler(cfg.Files, &terminalEnabled)))
	mux.Handle("GET /api/settings", cfg.Sessions.Require(http.HandlerFunc(getSettingsHandler(&terminalEnabled))))
	mux.Handle("POST /api/settings", cfg.Sessions.Require(http.HandlerFunc(setSettingsHandler(&terminalEnabled))))
	mux.Handle("GET /api/fs/meta", cfg.Sessions.Require(http.HandlerFunc(metaHandler(cfg.Files))))
	mux.Handle("GET /api/fs/text", cfg.Sessions.Require(http.HandlerFunc(textHandler(cfg.Files))))
	mux.Handle("GET /api/fs/raw", cfg.Sessions.Require(http.HandlerFunc(rawHandler(cfg.Files))))
	mux.Handle("POST /api/fs/file", cfg.Sessions.Require(http.HandlerFunc(createFileHandler(cfg.Files))))
	mux.Handle("POST /api/fs/dir", cfg.Sessions.Require(http.HandlerFunc(createDirHandler(cfg.Files))))
	mux.Handle("POST /api/fs/upload", cfg.Sessions.Require(http.HandlerFunc(uploadHandler(cfg.Files, cfg.AppConfig.MaxUploadSize))))
	mux.Handle("POST /api/fs/rename", cfg.Sessions.Require(http.HandlerFunc(renameHandler(cfg.Files))))
	mux.Handle("POST /api/fs/move", cfg.Sessions.Require(http.HandlerFunc(moveHandler(cfg.Files))))
	mux.Handle("GET /api/fs/zip", cfg.Sessions.Require(http.HandlerFunc(zipHandler(cfg.Files))))
	mux.Handle("GET /api/fs/events", cfg.Sessions.Require(http.HandlerFunc(eventsHandler(cfg.Files))))
	mux.Handle("DELETE /api/fs/delete", cfg.Sessions.Require(http.HandlerFunc(deleteHandler(cfg.Files))))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "API route not found")
	})
	mux.Handle("/", spaHandler(cfg.Assets))

	handler := securityHeaders(recoverPanic(requestLogger(mux)))
	if cfg.AppConfig.SecureCookie {
		handler = securityHeadersWithHSTS(recoverPanic(requestLogger(mux)))
	}
	return &Server{
		config:          cfg,
		terminalEnabled: &terminalEnabled,
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

func browseHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := workspacefs.BrowseDirectories(r.URL.Query().Get("path"))
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func terminalHandler(service *workspacefs.Service, enabled *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !*enabled {
			writeError(w, http.StatusNotFound, "terminal_disabled", "Terminal is disabled in settings")
			return
		}
		terminal.Handler(service.GetRoot()).ServeHTTP(w, r)
	})
}

func getSettingsHandler(enabled *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{
			"enableTerminal": *enabled,
		})
	}
}

func setSettingsHandler(enabled *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			EnableTerminal *bool `json:"enableTerminal"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload")
			return
		}
		if req.EnableTerminal != nil {
			*enabled = *req.EnableTerminal
			if err := config.SaveTerminalSetting(*req.EnableTerminal); err != nil {
				slog.Warn("save terminal setting failed", "error", err)
				writeError(w, http.StatusInternalServerError, "save_failed", "Failed to save setting")
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]bool{
			"enableTerminal": *enabled,
		})
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
		if workspacefs.IsInlineImage(mimeType) {
			w.Header().Set("Cache-Control", "public, max-age=86400")
		} else {
			w.Header().Set("Cache-Control", "private, max-age=60")
		}
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
		writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "File exceeds the configured size limit")
	case errors.Is(err, workspacefs.ErrInvalidEncoding):
		writeError(w, http.StatusUnsupportedMediaType, "invalid_text_encoding", "File is not valid UTF-8")
	case errors.Is(err, workspacefs.ErrAlreadyExists):
		writeError(w, http.StatusConflict, "already_exists", "A file or directory with that name already exists")
	case errors.Is(err, workspacefs.ErrNameEmpty):
		writeError(w, http.StatusBadRequest, "invalid_name", "Name cannot be empty or contain path separators")
	case errors.Is(err, os.ErrNotExist):
		writeError(w, http.StatusNotFound, "not_found", "File or directory not found")
	case errors.Is(err, os.ErrPermission):
		writeError(w, http.StatusForbidden, "permission_denied", "Permission denied")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Unable to access workspace entry")
	}
}

func createFileHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Path string `json:"path"`
		}{}
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON request body")
			return
		}
		item, err := service.CreateFile(body.Path)
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": item})
	}
}

func createDirHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Path string `json:"path"`
		}{}
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON request body")
			return
		}
		item, err := service.CreateDir(body.Path)
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": item})
	}
}

func uploadHandler(service *workspacefs.Service, maxUploadSize int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			writeError(w, http.StatusBadRequest, "invalid_path", "path query parameter is required")
			return
		}
		defer r.Body.Close()
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		item, err := service.SaveUpload(path, r.Body)
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "file_too_large", "Upload exceeds the configured size limit")
				return
			}
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"item": item})
	}
}

func renameHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Path    string `json:"path"`
			NewName string `json:"newName"`
		}{}
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON request body")
			return
		}
		item, err := service.Rename(body.Path, body.NewName)
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"item": item})
	}
}

func moveHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body := struct {
			Path      string `json:"path"`
			TargetDir string `json:"targetDir"`
		}{}
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON request body")
			return
		}
		item, err := service.Move(body.Path, body.TargetDir)
		if err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"item": item})
	}
}

func zipHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dirPath := r.URL.Query().Get("path")
		if dirPath == "" {
			writeError(w, http.StatusBadRequest, "invalid_path", "path query parameter is required")
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Cache-Control", "private, max-age=60")
		tracker := &bodyTrackingWriter{ResponseWriter: w}
		_, err := service.StreamZip(dirPath, tracker, func(name string) {
			if formatted := mime.FormatMediaType("attachment", map[string]string{"filename": name}); formatted != "" {
				w.Header().Set("Content-Disposition", formatted)
			}
		})
		if err != nil {
			if tracker.started {
				slog.Error("zip stream failed after body started", "path", dirPath, "error", err)
				return
			}
			writeFileError(w, err)
			return
		}
	}
}

func deleteHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			writeError(w, http.StatusBadRequest, "invalid_path", "path query parameter is required")
			return
		}
		if err := service.Delete(path); err != nil {
			writeFileError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"deleted": path})
	}
}

func eventsHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "internal_error", "Streaming unsupported")
			return
		}

		ch := service.Events.Subscribe()
		defer service.Events.Unsubscribe(ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case event := <-ch:
				data, err := json.Marshal(event)
				if err == nil {
					w.Write([]byte("data: "))
					w.Write(data)
					w.Write([]byte("\n\n"))
					flusher.Flush()
				}
			}
		}
	}
}

func getWorkspaceHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"workspace": service.GetRoot()})
	}
}

type setWorkspaceRequest struct {
	Workspace string `json:"workspace"`
	Path      string `json:"path"`
}

func setWorkspaceHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req setWorkspaceRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload")
			return
		}
		targetPath := strings.TrimSpace(req.Workspace)
		if targetPath == "" {
			targetPath = strings.TrimSpace(req.Path)
		}
		if targetPath == "" {
			writeError(w, http.StatusBadRequest, "invalid_path", "Workspace path is required")
			return
		}
		cleanPath, err := service.SetRoot(targetPath)
		if err != nil {
			slog.Warn("workspace switch rejected", "error", err)
			writeError(w, http.StatusBadRequest, "invalid_workspace", "Workspace path is invalid, sensitive, or not a directory")
			return
		}
		_ = config.SaveWorkspaceSetting(cleanPath)
		writeJSON(w, http.StatusOK, map[string]string{"workspace": cleanPath})
	}
}

func decodeJSONBody(r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<16)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
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
