package api

import (
	"io"
	"net/http"
)

func (s *Server) receiveVitals(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	w.WriteHeader(http.StatusNoContent)
}
