// Package install — embed.go serves the React install wizard SPA from an embedded
// file system. In production the dist/ bundle is embedded at compile time.
// In dev mode (MINK_DEV=1) a stub handler redirects developers to the Vite dev server.
// SPEC: SPEC-MINK-ONBOARDING-001 Phase 3A
package install

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// StaticHandler returns an http.Handler serving the embedded React bundle.
//
// Dev mode (MINK_DEV=1): returns a stub HTML page instructing the developer to
// run `npm run dev` in web/install/ and visit http://127.0.0.1:5173.
//
// Production mode: serves the embedded dist/ directory with SPA fallback — any
// path that does not match a real static asset returns index.html so that React
// Router can handle client-side routing.
//
// Empty dist (Phase 3A placeholder): when dist/index.html is absent from the
// embedded FS (dist contains only .gitkeep), the handler returns a friendly
// build-instructions page for all /install paths.
//
// @MX:ANCHOR: [AUTO] Entry point for static asset serving — used by NewHandler and tests.
// @MX:REASON: Switching between dev/prod/empty modes must be transparent to all callers;
// any mode-detection change here affects the entire serving pipeline.
func StaticHandler() http.Handler {
	// Dev mode: skip embed entirely.
	if os.Getenv("MINK_DEV") == "1" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(devModePage))
		})
	}

	// Check whether the bundle exists inside the embed.
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return buildRequiredHandler()
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		// dist is empty (Phase 3A .gitkeep only).
		return buildRequiredHandler()
	}

	fileServer := http.FileServer(http.FS(sub))
	return &spaHandler{fileServer: fileServer, fsys: sub}
}

// spaHandler wraps an http.FileServer and falls back to index.html for unknown paths
// so that React Router handles client-side routing.
type spaHandler struct {
	fileServer http.Handler
	fsys       fs.FS
}

// ServeHTTP serves static assets directly and falls back to index.html for SPA routes.
//
// Root-relative path stripping: http.FileServer (backed by embed.FS via fs.Sub) expects
// paths without a leading slash (e.g. "assets/index-abc.js", not "/assets/index-abc.js").
// We trim the leading slash before calling fs.Stat so the existence check is accurate.
// An empty path (bare "/") maps to "index.html".
//
// @MX:WARN: [AUTO] Leading-slash stripping is required for embed.FS compatibility.
// @MX:REASON: fs.Stat on an embed.FS sub-tree fails silently when given an absolute path,
// causing all assets to fall back to index.html and returning HTML instead of JS/CSS.
func (s *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip leading slash for embed.FS fs.Stat compatibility.
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "index.html"
	}
	if _, err := fs.Stat(s.fsys, p); err == nil {
		// File exists: serve it directly via the file server.
		s.fileServer.ServeHTTP(w, r)
		return
	}
	// File not found: SPA fallback — return index.html and let React Router handle routing.
	r2 := r.Clone(r.Context())
	r2.URL.Path = "/"
	s.fileServer.ServeHTTP(w, r2)
}

// buildRequiredHandler returns a handler that displays build instructions.
func buildRequiredHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(buildRequiredPage))
	})
}

// devModePage is the stub page shown in MINK_DEV=1 mode.
const devModePage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>MINK Install — Dev Mode</title></head>
<body>
<h1>MINK Install Wizard — Dev Mode</h1>
<p>The backend is running. Start the Vite dev server:</p>
<pre>cd web/install && npm install && npm run dev</pre>
<p>Then visit <a href="http://127.0.0.1:5173">http://127.0.0.1:5173</a></p>
</body>
</html>`

// buildRequiredPage is shown when dist/index.html is absent from the embed.
const buildRequiredPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="UTF-8"><title>MINK Install — Build Required</title></head>
<body>
<h1>MINK Install Wizard — Web UI Not Built</h1>
<p>The React bundle is not available. Build it first:</p>
<pre>cd web/install && npm install && npm run build</pre>
<p>Then rebuild the mink binary:</p>
<pre>go build ./...</pre>
<p>Or set <code>MINK_DEV=1</code> and run <code>npm run dev</code> for development.</p>
</body>
</html>`
