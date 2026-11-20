package app

import (
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// spaHandler serves the built frontend. Real files are served as-is; any other path gets
// index.html so client-side routes like /inventory survive a refresh.
func spaHandler(dir string) http.HandlerFunc {
	root := http.Dir(dir)
	files := http.FileServer(root)
	index := filepath.Join(dir, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		if f, err := root.Open(path.Clean("/" + r.URL.Path)); err == nil {
			st, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && !st.IsDir() {
				if strings.HasPrefix(r.URL.Path, "/assets/") { // hashed filenames: cache for a year
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	}
}
