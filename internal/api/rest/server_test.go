package rest_test

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/immortal-engine/immortal/internal/api/rest"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/health"
	"github.com/immortal-engine/immortal/internal/storage"
)

func setupServer(t *testing.T) (*rest.Server, func()) {
	dir, _ := os.MkdirTemp("", "immortal-test-*")
	store, err := storage.New(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	registry := health.NewRegistry()
	registry.Register("api-server")
	registry.Update("api-server", health.StatusHealthy, "ok")
	healer := healing.NewHealer()

	cleanup := func() {
		store.Close()
		os.RemoveAll(dir)
	}

	return rest.New(store, registry, healer), cleanup
}

func TestHealthEndpoint(t *testing.T) {
	s, cleanup := setupServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != string(health.StatusHealthy) {
		t.Errorf("expected healthy status, got %v", resp["status"])
	}
}

func TestEventsEndpoint(t *testing.T) {
	s, cleanup := setupServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/events", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestStatusEndpoint(t *testing.T) {
	s, cleanup := setupServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["engine"] != "running" {
		t.Error("expected engine running")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s, cleanup := setupServer(t)
	defer cleanup()
	req := httptest.NewRequest("POST", "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 405 {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}
