//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dsionov/carwatch/internal/api"
	"github.com/dsionov/carwatch/internal/catalog"
	"github.com/dsionov/carwatch/internal/config"
	"github.com/dsionov/carwatch/internal/health"
	"github.com/dsionov/carwatch/internal/spa"
	"github.com/dsionov/carwatch/internal/storage/sqlite"
	"github.com/dsionov/carwatch/web"
)

func TestSmoke_FullStack(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "smoke.db")

	store, err := sqlite.New(dbPath)
	if err != nil {
		t.Fatalf("create store with file-backed DB: %v", err)
	}
	defer func() { _ = store.Close() }()

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cat := catalog.NewDynamic(store, logger)
	cat.Load(ctx)
	cat.Ingest(ctx, 19, "Toyota", 10226, "Corolla")
	cat.Ingest(ctx, 27, "Mazda", 10332, "3")

	h := health.New()
	h.SetUserCounter(store)
	h.SetSearchCounter(store)
	h.RecordSuccess()

	const testToken = "smoke-test-token"
	const testChatID int64 = 999

	if err := store.UpsertUser(ctx, testChatID, "smoketest"); err != nil {
		t.Fatalf("upsert user: %v", err)
	}

	apiServer := api.New(api.Config{
		Catalog:  cat,
		Searches: store,
		Listings: store,
		Users:    store,
		Prices:   store,
		Admin:    store,
		Saved:    store,
		Hidden:   store,
		Notifs:   store,
		Logger:   logger,
		API: config.APIConfig{
			CORSOrigins: []string{"http://localhost:5173"},
			AuthToken:   testToken,
			DevChatID:   testChatID,
		},
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.Handler())
	mux.Handle("/api/v1/", apiServer.Routes())
	mux.Handle("/", spa.Handler(web.DistFS()))

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("healthz returns 200", func(t *testing.T) {
		resp, err := client.Get(srv.URL + "/healthz")
		if err != nil {
			t.Fatalf("GET /healthz: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body["status"] != "ok" {
			t.Errorf("health status = %q, want ok", body["status"])
		}
	})

	authGet := func(url string) (*http.Response, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+testToken)
		return client.Do(req)
	}

	t.Run("catalog manufacturers returns data", func(t *testing.T) {
		resp, err := authGet(srv.URL + "/api/v1/catalog/manufacturers")
		if err != nil {
			t.Fatalf("GET /api/v1/catalog/manufacturers: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var mfrs []map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&mfrs); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(mfrs) < 2 {
			t.Errorf("expected at least 2 manufacturers, got %d", len(mfrs))
		}
	})

	t.Run("SPA serves index.html", func(t *testing.T) {
		resp, err := client.Get(srv.URL + "/")
		if err != nil {
			t.Fatalf("GET /: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "<!doctype html>") && !strings.Contains(string(body), "<!DOCTYPE html>") {
			t.Error("response body does not contain <!doctype html>")
		}
	})

	t.Run("unauthenticated API returns 401", func(t *testing.T) {
		resp, err := client.Get(srv.URL + "/api/v1/searches")
		if err != nil {
			t.Fatalf("GET /api/v1/searches: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("file-backed DB persists", func(t *testing.T) {
		info, err := os.Stat(dbPath)
		if err != nil {
			t.Fatalf("stat DB file: %v", err)
		}
		if info.Size() == 0 {
			t.Error("DB file should not be empty after operations")
		}
	})
}
