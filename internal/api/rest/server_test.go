package rest_test

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/immortal-engine/immortal/internal/api/rest"
	"github.com/immortal-engine/immortal/internal/audit"
	"github.com/immortal-engine/immortal/internal/causality"
	"github.com/immortal-engine/immortal/internal/dependency"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/export"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/health"
	"github.com/immortal-engine/immortal/internal/pattern"
	"github.com/immortal-engine/immortal/internal/predict"
	"github.com/immortal-engine/immortal/internal/selfmonitor"
	"github.com/immortal-engine/immortal/internal/sla"
	"github.com/immortal-engine/immortal/internal/storage"
	"github.com/immortal-engine/immortal/internal/timetravel"
	"time"
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

func setupFullServer(t *testing.T) (*rest.Server, func()) {
	dir, _ := os.MkdirTemp("", "immortal-test-*")
	store, err := storage.New(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	registry := health.NewRegistry()
	registry.Register("api-server")
	registry.Update("api-server", health.StatusHealthy, "ok")
	healer := healing.NewHealer()
	d := dna.New("test")
	d.Record("cpu_percent", 50.0)
	al := audit.New(100)
	al.Log("test-action", "tester", "target", "detail", true)
	slaT := sla.New()
	slaT.RecordStatus("api-server", true)
	pred := predict.New()
	pred.SetThreshold("cpu_percent", 90.0)
	depG := dependency.New()
	depG.AddDependency("api", "database")

	recs := func() []healing.Recommendation {
		return []healing.Recommendation{{RuleName: "test-rule", Message: "test recommendation"}}
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(dir)
	}

	s := rest.NewFull(rest.ServerConfig{
		Store:           store,
		Registry:        registry,
		Healer:          healer,
		DNA:             d,
		Causality:       causality.New(),
		TimeTravel:      timetravel.New(100),
		Monitor:         selfmonitor.New(),
		Exporter:        export.NewPrometheus(),
		PatternDet:      pattern.New(5*time.Minute, 3),
		Predictor:       pred,
		SLATracker:      slaT,
		AuditLog:        al,
		DepGraph:        depG,
		Recommendations: recs,
	})

	return s, cleanup
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

// --- Advanced endpoint tests ---

func TestRecommendationsEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/recommendations", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var recs []map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&recs)
	if len(recs) != 1 {
		t.Errorf("expected 1 recommendation, got %d", len(recs))
	}
}

func TestMetricsEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/metrics", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("expected text/plain content-type, got %s", ct)
	}
}

func TestDNABaselineEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/dna/baseline", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var data map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&data)
	if _, ok := data["cpu_percent"]; !ok {
		t.Error("expected cpu_percent in baseline")
	}
}

func TestDNAHealthScoreEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/dna/health-score", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var data map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&data)
	if _, ok := data["health_score"]; !ok {
		t.Error("expected health_score")
	}
}

func TestDNAAnomalyEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/dna/anomaly?metric=cpu_percent&value=50", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestDNAAnomalyMissingParams(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/dna/anomaly", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestPatternsEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/patterns", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestPredictionsEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/predictions", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestSLAEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/sla", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var report []map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&report)
	if len(report) == 0 {
		t.Error("expected at least one SLA entry")
	}
}

func TestAuditEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/audit?limit=10", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var entries []map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(entries))
	}
}

func TestAuditSearchEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/audit?q=test", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestDependenciesEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/dependencies", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var data map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&data)
	if _, ok := data["nodes"]; !ok {
		t.Error("expected nodes in response")
	}
}

func TestDependencyImpactEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/dependencies/impact?service=database", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var data map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&data)
	if data["service"] != "database" {
		t.Errorf("expected service=database, got %v", data["service"])
	}
}

func TestDependencyImpactMissingService(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/dependencies/impact", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestMonitorEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/monitor", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var data map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&data)
	if _, ok := data["goroutines"]; !ok {
		t.Error("expected goroutines in response")
	}
}

func TestTimeTravelEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/timetravel?count=5", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCausalityGraphEndpoint(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/causality/graph", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRootCauseMissingID(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/causality/root-cause", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 400 {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestStatusEndpointFull(t *testing.T) {
	s, cleanup := setupFullServer(t)
	defer cleanup()
	req := httptest.NewRequest("GET", "/api/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["version"] != "0.3.0" {
		t.Errorf("expected version 0.3.0, got %v", resp["version"])
	}
	if _, ok := resp["uptime"]; !ok {
		t.Error("expected uptime in full status")
	}
	if _, ok := resp["audit_entries"]; !ok {
		t.Error("expected audit_entries in full status")
	}
}
