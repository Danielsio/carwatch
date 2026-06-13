package api

import (
	"testing"

	"github.com/dsionov/carwatch/internal/config"
)

func TestIsAdmin_EmailMatching(t *testing.T) {
	// AdminEmail is stored pre-normalized (New lowercases/trims it once).
	srv := &Server{cfg: config.APIConfig{AdminEmail: "admin@example.com", AdminChatID: 42}}

	tests := []struct {
		name   string
		chatID int64
		email  string
		want   bool
	}{
		{"exact email", 0, "admin@example.com", true},
		{"mixed-case input email", 0, "Admin@Example.com", true},
		{"email with surrounding space", 0, "  admin@example.com  ", true},
		{"different email", 0, "someone@example.com", false},
		{"empty email", 0, "", false},
		// Unicode case-fold characters must NOT be treated as ASCII equals.
		{"dotless-i homoglyph rejected", 0, "admın@example.com", false},
		{"admin chat id", 42, "", true},
		{"non-admin chat id", 7, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := srv.isAdmin(tt.chatID, tt.email); got != tt.want {
				t.Errorf("isAdmin(%d, %q) = %v, want %v", tt.chatID, tt.email, got, tt.want)
			}
		})
	}
}

func TestIsAdmin_NoAdminConfigured(t *testing.T) {
	srv := &Server{cfg: config.APIConfig{}}
	if srv.isAdmin(0, "anyone@example.com") {
		t.Error("no admin email configured: must not grant admin")
	}
	if srv.isAdmin(123, "") {
		t.Error("no admin chat id configured: must not grant admin")
	}
}
