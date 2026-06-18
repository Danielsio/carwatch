package spa

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandler_ServesIndexForSPARoutes(t *testing.T) {
	fs := fstest.MapFS{
		"index.html":             {Data: []byte("<html>app</html>")},
		"assets/index-abc123.js": {Data: []byte("console.log('app')")},
	}

	handler := Handler(fs)

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
		wantCache  string
	}{
		{"/", http.StatusOK, "<html>app</html>", "no-cache"},
		{"/searches/new", http.StatusOK, "<html>app</html>", "no-cache"},
		{"/listings/tok-123", http.StatusOK, "<html>app</html>", "no-cache"},
		{"/admin", http.StatusOK, "<html>app</html>", "no-cache"},
		{"/assets/index-abc123.js", http.StatusOK, "console.log('app')", "public, max-age=31536000, immutable"},
		{"/assets/missing.js", http.StatusNotFound, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantBody != "" {
				if got := w.Body.String(); got != tt.wantBody {
					t.Errorf("body = %q, want %q", got, tt.wantBody)
				}
			}
			if tt.wantCache != "" {
				if got := w.Header().Get("Cache-Control"); got != tt.wantCache {
					t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
				}
			}
			if tt.path == "/" || tt.path == "/searches/new" {
				if got := w.Header().Get("Content-Security-Policy"); got == "" {
					t.Error("expected Content-Security-Policy on HTML responses")
				}
				if got := w.Header().Get("Referrer-Policy"); got != "no-referrer" {
					t.Errorf("Referrer-Policy = %q, want no-referrer", got)
				}
			}
		})
	}
}

// When a prerendered build is present, "/" serves the prerendered landing
// (index.html), /login and /signup serve their prerendered pages, and every
// other SPA route serves the clean app shell so the app does not flash
// marketing content before the bundle mounts.
func TestHandler_ServesShellForNonLandingRoutes(t *testing.T) {
	fs := fstest.MapFS{
		"index.html":             {Data: []byte("<html>prerendered-landing</html>")},
		"app-shell.html":         {Data: []byte("<html>clean-shell</html>")},
		"login.html":             {Data: []byte("<html>prerendered-login</html>")},
		"signup.html":            {Data: []byte("<html>prerendered-signup</html>")},
		"assets/index-abc123.js": {Data: []byte("console.log('app')")},
	}

	handler := Handler(fs)

	tests := []struct {
		path     string
		wantBody string
	}{
		{"/", "<html>prerendered-landing</html>"},
		{"/login", "<html>prerendered-login</html>"},
		{"/signup", "<html>prerendered-signup</html>"},
		{"/dashboard", "<html>clean-shell</html>"},
		{"/searches/new", "<html>clean-shell</html>"},
		{"/listings/tok-123", "<html>clean-shell</html>"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
			}
			if got := w.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := w.Header().Get("Cache-Control"); got != "no-cache" {
				t.Errorf("Cache-Control = %q, want no-cache", got)
			}
		})
	}
}
