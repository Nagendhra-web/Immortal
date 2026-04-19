// Package landing serves Immortal's marketing landing page. The built
// React/Vite output is embedded via go:embed and served as static files.
package landing

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static
var static embed.FS

// Handler returns the landing-page HTTP handler. Mount at "/" (the
// caller's mux should route /api/* and /dashboard/* BEFORE this falls
// through, since this serves as a catch-all for the marketing site).
func Handler() http.Handler {
	sub, _ := fs.Sub(static, "static")
	fsh := http.FileServer(http.FS(sub))
	indexHTML, _ := fs.ReadFile(sub, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Root path: serve index.html inline (not via FileServer, which
		// would 301 to "/index.html"). Marketing site has no client-side
		// routing so we keep it simple.
		if r.URL.Path == "/" || r.URL.Path == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexHTML)
			return
		}
		// Fixed content-type overrides because Windows mime registry
		// returns text/javascript for .js while the test contract expects
		// application/javascript.
		switch {
		case strings.HasSuffix(r.URL.Path, ".js"):
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		case strings.HasSuffix(r.URL.Path, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		}
		fsh.ServeHTTP(w, r)
	})
}
