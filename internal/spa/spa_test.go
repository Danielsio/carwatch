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
		wantBody   string
		wantCache  string
	}{
		{"/", "<html>app</html>", "no-cache"},
		{"/searches/new", "<html>app</html>", "no-cache"},
		{"/listings/tok-123", "<html>app</html>", "no-cache"},
		{"/admin", "<html>app</html>", "no-cache"},
		{"/assets/index-abc123.js", "console.log('app')", "public, max-age=31536000, immutable"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			if got := w.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := w.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
			}
		})
	}
}
