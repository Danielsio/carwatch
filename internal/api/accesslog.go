package api

import (
	"net/http"
	"strings"
	"time"
)

// Compile-time interface assertions.
var _ http.Flusher = (*statusRecorder)(nil)

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) withAccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		dur := time.Since(start)
		s.observeHTTPRequest(rec.status, dur)

		reqID := ""
		if v, ok := r.Context().Value(requestIDKey).(string); ok {
			reqID = v
		}

		chatID, _ := chatIDFromContext(r.Context())
		remoteAddr := r.RemoteAddr
		if s.cfg.TrustForwardedFor {
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				if i := strings.IndexByte(fwd, ','); i >= 0 {
					remoteAddr = strings.TrimSpace(fwd[:i])
				} else {
					remoteAddr = strings.TrimSpace(fwd)
				}
			}
		}

		fields := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", dur.Milliseconds(),
			"request_id", reqID,
			"remote_addr", remoteAddr,
			"user_agent", r.UserAgent(),
		}
		if chatID > 0 {
			fields = append(fields, "chat_id", chatID)
		}
		s.logger.Info("http_request", fields...)
	})
}
