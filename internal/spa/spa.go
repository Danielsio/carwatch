package spa

import (
	"io/fs"
	"net/http"
	"strings"
)

func Handler(distFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Reduce token leakage via Referer when navigating away from admin log streams, etc.
		w.Header().Set("Referrer-Policy", "no-referrer")
		// Firebase Auth SDK requires 'unsafe-inline' for scripts; nonce-based CSP
		// is not supported by Firebase's inline script injection.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' 'unsafe-inline' https://www.gstatic.com https://apis.google.com https://www.google.com; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' data: https://fonts.gstatic.com; "+
				"img-src 'self' data: https: blob:; "+
				"connect-src 'self' https://*.googleapis.com https://accounts.google.com "+
				"https://securetoken.googleapis.com https://identitytoolkit.googleapis.com https://www.googleapis.com "+
				"https://www.gstatic.com https://*.firebaseapp.com https://*.firebaseio.com wss://*.firebaseio.com "+
				"https://fonts.gstatic.com https://fonts.googleapis.com https://img.yad2.co.il https://apis.google.com; "+
				"frame-src https://accounts.google.com https://*.firebaseapp.com https://carwatch-5cabf.firebaseapp.com; "+
				"worker-src 'self'; "+
				"frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'")

		path := strings.TrimPrefix(r.URL.Path, "/")

		if path != "" {
			if _, err := fs.Stat(distFS, path); err == nil {
				if strings.HasPrefix(path, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(path, "assets/") {
				http.NotFound(w, r)
				return
			}
		}

		// SPA fallback. The landing route ("/") is prerendered into index.html
		// for SEO and a fast first paint. Every other route is served a clean
		// shell (app-shell.html) when one exists, so the authenticated app does
		// not briefly flash the marketing page before the JS bundle mounts.
		// Falls back to index.html when no prerendered shell is present.
		w.Header().Set("Cache-Control", "no-cache")
		if path != "" {
			if _, err := fs.Stat(distFS, "app-shell.html"); err == nil {
				r.URL.Path = "/app-shell.html"
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
