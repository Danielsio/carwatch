package app

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/dsionov/carwatch/internal/api"
)

func metricsAuthMiddleware(bind, token string, next http.Handler) http.Handler {
	nonLocal := api.IsNonLocalBind(bind)
	requiredToken := strings.TrimSpace(token)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !nonLocal {
			next.ServeHTTP(w, r)
			return
		}
		candidate := strings.TrimSpace(r.Header.Get("X-CarWatch-Telemetry-Token"))
		if candidate == "" {
			candidate = bearerFromHeader(r.Header.Get("Authorization"))
		}
		if requiredToken == "" || subtle.ConstantTimeCompare([]byte(candidate), []byte(requiredToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bearerFromHeader(value string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}
