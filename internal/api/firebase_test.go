package api

import (
	"path/filepath"
	"testing"
)

func TestNewFirebaseVerifier_InvalidJSON(t *testing.T) {
	_, err := NewFirebaseVerifier("", "{not-json", "test-project")
	if err == nil {
		t.Fatal("expected error for invalid credentials JSON")
	}
}

func TestNewFirebaseVerifier_MissingCredentialsFile(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "nonexistent", "creds.json")
	_, err := NewFirebaseVerifier(badPath, "", "test-project")
	if err == nil {
		t.Fatal("expected error for missing credentials file")
	}
}

func TestNewFirebaseVerifier_NoExplicitCredentials(t *testing.T) {
	// Matches buildAPI when ProjectID is set but credential file/JSON are empty:
	// Firebase may use application default credentials or return an init error.
	v, err := NewFirebaseVerifier("", "", "carwatch-firebase-test-project-id")
	if err != nil {
		if err.Error() == "" {
			t.Fatal("error should be non-empty when init fails")
		}
		return
	}
	if v == nil {
		t.Fatal("non-nil verifier expected when err is nil")
	}
}

func TestNewFirebaseVerifier_InvalidServiceAccountJSON(t *testing.T) {
	_, err := NewFirebaseVerifier("", `{"foo":true}`, "test-invalid-sa-project")
	if err == nil {
		t.Fatal("expected error for non-service-account JSON")
	}
}
