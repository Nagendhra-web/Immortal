package dashboard

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static
var static embed.FS

// Handler returns the dashboard HTTP handler. Mount at /dashboard/.
// Go's http.FileServer redirects requests ending in "/index.html" to "./";
// we intercept those paths and serve the file content directly so that
// GET /dashboard/index.html always returns 200.
func Handler() http.Handler {
	sub, _ := fs.Sub(static, "static")
	fileServer := http.StripPrefix("/dashboard/", http.FileServer(http.FS(sub)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve index.html directly to avoid FileServer's 301 redirect.
		if strings.HasSuffix(r.URL.Path, "/index.html") || r.URL.Path == "/index.html" {
			// Strip /dashboard/ prefix to get the bare path.
			bare := strings.TrimPrefix(r.URL.Path, "/dashboard/")
			if bare == "" {
				bare = "index.html"
			}
			f, err := sub.Open(bare)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer f.Close()
			st, err := f.Stat()
			if err != nil {
				http.NotFound(w, r)
				return
			}
			rs, ok := f.(io.ReadSeeker)
			if !ok {
				http.NotFound(w, r)
				return
			}
			http.ServeContent(w, r, bare, st.ModTime(), rs)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
