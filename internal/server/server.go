package server

import (
	"bytes"
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
	mux.Handle("POST /api/fs/file", cfg.Sessions.Require(http.HandlerFunc(createFileHandler(cfg.Files))))
	mux.Handle("POST /api/fs/dir", cfg.Sessions.Require(http.HandlerFunc(createDirHandler(cfg.Files))))
	mux.Handle("POST /api/fs/upload", cfg.Sessions.Require(http.HandlerFunc(uploadHandler(cfg.Files))))
	mux.Handle("POST /api/fs/rename", cfg.Sessions.Require(http.HandlerFunc(renameHandler(cfg.Files))))
	mux.Handle("POST /api/fs/move", cfg.Sessions.Require(http.HandlerFunc(moveHandler(cfg.Files))))
	mux.Handle("GET /api/fs/zip", cfg.Sessions.Require(http.HandlerFunc(zipHandler(cfg.Files))))
	mux.Handle("DELETE /api/fs/delete", cfg.Sessions.Require(http.HandlerFunc(deleteHandler(cfg.Files))))
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
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
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
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
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

func uploadHandler(service *workspacefs.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		if path == "" {
			writeError(w, http.StatusBadRequest, "invalid_path", "path query parameter is required")
			return
		}
		defer r.Body.Close()
		item, err := service.SaveUpload(path, r.Body)
		if err != nil {
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
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
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
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
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
		data, filename, err := service.CreateZip(dirPath)
		if err != nil {
			writeFileError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
		w.Header().Set("Cache-Control", "private, max-age=60")
		http.ServeContent(w, r, filename, time.Now(), bytes.NewReader(data))
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

func decodeJSONBody(r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<16)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
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
