package api

import (
	"io"
	"net/http"
)

const vitalsMaxBody = 128 << 10 // 128 KB

func (s *Server) receiveVitals(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, vitalsMaxBody)
	if _, err := io.Copy(io.Discard, r.Body); err != nil {
		s.logger.Debug("failed to drain vitals payload", "error", err)
	}
	w.WriteHeader(http.StatusNoContent)
}
