package api

import (
	"time"
)

// observeHTTPRequest records aggregate request metrics for admin stats (issue #641).
func (s *Server) observeHTTPRequest(status int, duration time.Duration) {
	if s == nil {
		return
	}
	s.httpReqTotal.Add(1)
	switch {
	case status >= 500:
		s.http5xx.Add(1)
	case status >= 400:
		s.http4xx.Add(1)
	default:
		s.http2xx.Add(1)
	}
	s.httpDurationMs.Add(uint64(duration.Milliseconds()))
}

func (s *Server) httpMetricsSnapshot() (total, ok2xx, ok4xx, ok5xx, durationSumMs uint64) {
	if s == nil {
		return
	}
	return s.httpReqTotal.Load(), s.http2xx.Load(), s.http4xx.Load(), s.http5xx.Load(), s.httpDurationMs.Load()
}
