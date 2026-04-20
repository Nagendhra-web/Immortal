package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleConfig_AlwaysReturnsVersionMeta(t *testing.T) {
	srv := NewFull(ServerConfig{})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	v, ok := body["version"].(map[string]any)
	if !ok {
		t.Fatalf("version block missing or wrong type; got %T", body["version"])
	}
	for _, key := range []string{"tag", "full", "go", "os", "arch", "pid", "hostname", "started_at"} {
		if _, has := v[key]; !has {
			t.Errorf("version.%s missing", key)
		}
	}
}

func TestHandleConfig_FeatureFlagsReflectNilDeps(t *testing.T) {
	srv := NewFull(ServerConfig{})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	features := body["features"].(map[string]any)
	for _, name := range []string{"pqaudit", "twin", "agentic", "intent", "narrator", "evolve"} {
		if features[name] != false {
			t.Errorf("nil-deps server should report %s=false; got %v", name, features[name])
		}
	}
}

func TestHandleConfig_IncludesEngineWhenCallbackSet(t *testing.T) {
	srv := NewFull(ServerConfig{
		EngineConfig: func() any {
			return map[string]any{"data_dir": "/var/lib/immortal", "ghost_mode": false}
		},
	})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	eng, ok := body["engine"].(map[string]any)
	if !ok {
		t.Fatalf("engine block missing when callback set")
	}
	if eng["data_dir"] != "/var/lib/immortal" {
		t.Errorf("engine block content wrong: %v", eng)
	}
}

func TestHandleConfig_POSTRejected(t *testing.T) {
	srv := NewFull(ServerConfig{})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/config", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST should be 405; got %d", rec.Code)
	}
}
