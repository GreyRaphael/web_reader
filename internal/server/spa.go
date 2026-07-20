package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func spaHandler(assets fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		requested := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if requested != "" && requested != "." {
			if info, err := fs.Stat(assets, requested); err == nil && !info.IsDir() {
				if strings.Contains(requested, "/assets/") || strings.HasPrefix(requested, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "frontend_unavailable", "Frontend assets are unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
