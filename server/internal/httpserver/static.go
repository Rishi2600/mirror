package httpserver

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

// webApp serves the built single-page app from dir. A path that names a file
// is served as that file; any other path gets index.html, so the client's
// router can handle deep links.
func webApp(dir string) http.Handler {
	root := os.DirFS(dir)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(root, name); err == nil && !info.IsDir() {
				http.ServeFileFS(w, r, root, name)
				return
			}
		}
		// index.html must not be cached, so a new build reaches users at once.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, root, "index.html")
	})
}
