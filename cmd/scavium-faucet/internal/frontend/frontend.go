package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:web
var embeddedFS embed.FS

// Handler returns an http.Handler that serves the embedded web frontend.
// Requests for static files are served directly; all other paths fall back
// to index.html so that the single-page app handles routing.
func Handler() http.Handler {
	sub, err := fs.Sub(embeddedFS, "web")
	if err != nil {
		// Should never happen with a correctly embedded directory.
		panic("frontend: failed to sub embedded FS: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip leading slash for FS lookup.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if info, err := fs.Stat(sub, path); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Serve index.html for any unmatched path.
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		fileServer.ServeHTTP(w, r2)
	})
}
