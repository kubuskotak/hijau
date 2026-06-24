package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// serveStatic serves the built SvelteKit SPA from cfg.WebDir, with a fallback to
// index.html so client-side routes (e.g. /editor, /p/123) resolve. It is
// registered as the catch-all `GET /`; the more specific /api and /health
// routes take precedence in the mux, and this handler refuses those prefixes
// defensively so an unknown API path 404s instead of returning HTML.
func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if s.cfg.WebDir == "" ||
		strings.HasPrefix(r.URL.Path, "/api/") ||
		strings.HasPrefix(r.URL.Path, "/health") {
		http.NotFound(w, r)
		return
	}
	// Resolve the request path under WebDir, neutralising any traversal.
	rel := filepath.Clean("/" + r.URL.Path)
	full := filepath.Join(s.cfg.WebDir, rel)
	if fi, err := os.Stat(full); err == nil && !fi.IsDir() {
		http.ServeFile(w, r, full)
		return
	}
	// Unknown path → hand the SPA its entry point for client-side routing.
	http.ServeFile(w, r, filepath.Join(s.cfg.WebDir, "index.html"))
}
