package httpserver

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func SPA(root string) http.Handler {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		absoluteRoot = root
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method is not allowed")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "API route not found")
			return
		}
		for _, segment := range strings.Split(filepath.ToSlash(r.URL.Path), "/") {
			if segment == ".." {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
				return
			}
		}
		clean := strings.TrimPrefix(filepath.Clean(filepath.FromSlash(r.URL.Path)), string(filepath.Separator))
		target := filepath.Join(absoluteRoot, clean)
		if !withinRoot(absoluteRoot, target) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return
		}
		if clean != "." && clean != "" {
			if info, statErr := os.Stat(target); statErr == nil && !info.IsDir() {
				w.Header().Set("Cache-Control", "public, max-age=300")
				http.ServeFile(w, r, target)
				return
			}
		}
		index := filepath.Join(absoluteRoot, "index.html")
		if info, statErr := os.Stat(index); statErr != nil || info.IsDir() {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, index)
	})
}

func withinRoot(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
