package rest_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/agentic"
	"github.com/immortal-engine/immortal/internal/api/rest"
	"github.com/immortal-engine/immortal/internal/audit"
	"github.com/immortal-engine/immortal/internal/causal"
	"github.com/immortal-engine/immortal/internal/causality"
	"github.com/immortal-engine/immortal/internal/dependency"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/export"
	"github.com/immortal-engine/immortal/internal/federated"
	"github.com/immortal-engine/immortal/internal/healing"
	"github.com/immortal-engine/immortal/internal/health"
	"github.com/immortal-engine/immortal/internal/pattern"
	"github.com/immortal-engine/immortal/internal/predict"
	"github.com/immortal-engine/immortal/internal/pqaudit"
	"github.com/immortal-engine/immortal/internal/selfmonitor"
	"github.com/immortal-engine/immortal/internal/sla"
	"github.com/immortal-engine/immortal/internal/storage"
	"github.com/immortal-engine/immortal/internal/timetravel"
	"github.com/immortal-engine/immortal/internal/twin"
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

// --- v0.4.0 helper ---

func newPQLedger(t *testing.T) *pqaudit.Ledger {
	t.Helper()
	signer, err := pqaudit.NewEd25519Signer("test-key")
	if err != nil {
		t.Fatal(err)
	}
	led, err := pqaudit.New(pqaudit.Config{Signer: signer, MaxEntries: 1000})
	if err != nil {
		t.Fatal(err)
	}
	return led
}

func setupV4Server(t *testing.T) (*rest.Server, func()) {
	t.Helper()
	dir, _ := os.MkdirTemp("", "immortal-v4-test-*")
	store, err := storage.New(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	registry := health.NewRegistry()
	healer := healing.NewHealer()

	led := newPQLedger(t)
	if _, err := led.Append("test-action", "tester", "target", "detail", true); err != nil {
		t.Fatal(err)
	}

	tw := twin.New(twin.Config{})
	tw.Observe(twin.State{Service: "svc-a", CPU: 30, Memory: 40, Replicas: 2, Healthy: true})
	tw.Observe(twin.State{Service: "svc-b", CPU: 60, Memory: 70, Replicas: 1, Healthy: false})

	ag := agentic.New(agentic.Config{MaxIterations: 8, Planner: &testPlanner{}})

	fedClient := federated.NewClientWithSeed("test-node", 42, 0)
	fedClient.Observe("cpu", 55.0)
	fedClient.Observe("mem", 70.0)

	// causalFn backed by a small in-memory dataset (40 rows, 2 metrics)
	causalFn := func(outcome string, vars []string) (*causal.RootCauseResult, error) {
		names := []string{"cpu", "mem"}
		ds := causal.NewDataset(names)
		for i := 0; i < 40; i++ {
			_ = ds.Add(map[string]float64{
				"cpu": float64(i) + 1.0,
				"mem": float64(i)*0.9 + 0.5,
			})
		}
		g, err := causal.Discover(ds, causal.DiscoverConfig{Alpha: 0.05, MaxCondSetSize: 2})
		if err != nil {
			return nil, err
		}
		rc, err := causal.RootCause(ds, g, outcome)
		if err != nil {
			return nil, err
		}
		return &rc, nil
	}

	cleanup := func() {
		store.Close()
		os.RemoveAll(dir)
	}

	s := rest.NewFull(rest.ServerConfig{
		Store:     store,
		Registry:  registry,
		Healer:    healer,
		PQLedger:  led,
		Twin:      tw,
		Agent:     ag,
		FedClient: fedClient,
		CausalFn:  causalFn,
	})
	return s, cleanup
}

// --- v0.4.0 audit tests ---

func TestAPI_Audit_Verify(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/api/v4/audit/verify", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if _, ok := resp["ok"]; !ok {
		t.Error("expected 'ok' field in response")
	}
	if _, ok := resp["count"]; !ok {
		t.Error("expected 'count' field in response")
	}
}

func TestAPI_Audit_Verify_Disabled_Returns200(t *testing.T) {
	// When ledger is nil, verify returns ok:true with zero issues
	dir, _ := os.MkdirTemp("", "immortal-v4-nodisabled-*")
	store, _ := storage.New(dir + "/test.db")
	defer func() { store.Close(); os.RemoveAll(dir) }()
	s := rest.NewFull(rest.ServerConfig{
		Store:    store,
		Registry: health.NewRegistry(),
		Healer:   healing.NewHealer(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v4/audit/verify", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["ok"] != true {
		t.Errorf("expected ok:true when ledger disabled, got %v", resp["ok"])
	}
}

func TestAPI_Audit_MerkleRoot(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/api/v4/audit/merkle-root", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	root, ok := resp["root"].(string)
	if !ok || root == "" {
		t.Error("expected non-empty 'root' hex string")
	}
}

func TestAPI_Audit_MerkleRoot_Disabled_404(t *testing.T) {
	dir, _ := os.MkdirTemp("", "immortal-v4-nomerkle-*")
	store, _ := storage.New(dir + "/test.db")
	defer func() { store.Close(); os.RemoveAll(dir) }()
	s := rest.NewFull(rest.ServerConfig{
		Store:    store,
		Registry: health.NewRegistry(),
		Healer:   healing.NewHealer(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v4/audit/merkle-root", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAPI_Audit_Entries(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/api/v4/audit/entries?limit=10", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	entries, ok := resp["entries"]
	if !ok {
		t.Fatal("expected 'entries' key in response")
	}
	list, ok := entries.([]interface{})
	if !ok || len(list) == 0 {
		t.Errorf("expected at least one entry, got %v", entries)
	}
}

// --- v0.4.0 twin tests ---

func TestAPI_Twin_States(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/api/v4/twin/states", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	states, ok := resp["states"]
	if !ok {
		t.Fatal("expected 'states' key")
	}
	m, ok := states.(map[string]interface{})
	if !ok || len(m) < 2 {
		t.Errorf("expected at least 2 services, got %v", states)
	}
}

func TestAPI_Twin_StateByService(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/api/v4/twin/state/svc-a", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var state map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&state)
	if state["Service"] != "svc-a" {
		t.Errorf("expected Service=svc-a, got %v", state["Service"])
	}
}

func TestAPI_Twin_StateByService_NotFound(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/api/v4/twin/state/nonexistent", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAPI_Twin_Simulate(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()

	body := `{"id":"plan-1","actions":[{"type":"restart","target":"svc-b","params":{}}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v4/twin/simulate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var sim map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&sim)
	if sim["PlanID"] != "plan-1" {
		t.Errorf("expected PlanID=plan-1, got %v", sim["PlanID"])
	}
}

func TestAPI_Twin_Simulate_Disabled_404(t *testing.T) {
	dir, _ := os.MkdirTemp("", "immortal-v4-notwin-*")
	store, _ := storage.New(dir + "/test.db")
	defer func() { store.Close(); os.RemoveAll(dir) }()
	s := rest.NewFull(rest.ServerConfig{
		Store:    store,
		Registry: health.NewRegistry(),
		Healer:   healing.NewHealer(),
	})
	body := `{"id":"plan-x","actions":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v4/twin/simulate", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// --- v0.4.0 agentic tests ---

func TestAPI_Agentic_Run(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()

	// heuristic planner: source non-empty, health checker returns "healthy" →
	// iter 0: check_health → "healthy"; iter 1: finish (already healthy) → Resolved=true
	body := fmt.Sprintf(`{"type":"error","severity":"critical","message":"high cpu","source":"svc-a","meta":{}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v4/agentic/run", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var trace map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&trace)
	resolved, _ := trace["Resolved"].(bool)
	if !resolved {
		t.Errorf("expected Resolved=true for critical event with healthy service, got trace=%v", trace)
	}
}

func TestAPI_Agentic_Run_Disabled_404(t *testing.T) {
	dir, _ := os.MkdirTemp("", "immortal-v4-noagent-*")
	store, _ := storage.New(dir + "/test.db")
	defer func() { store.Close(); os.RemoveAll(dir) }()
	s := rest.NewFull(rest.ServerConfig{
		Store:    store,
		Registry: health.NewRegistry(),
		Healer:   healing.NewHealer(),
	})
	body := `{"type":"error","severity":"critical","message":"test","source":"svc","meta":{}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v4/agentic/run", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// --- v0.4.0 causal tests ---

func TestAPI_Causal_RootCause(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()

	body := `{"outcome":"mem","variables":["cpu","mem"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v4/causal/root-cause", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)
	if result["Outcome"] != "mem" {
		t.Errorf("expected Outcome=mem, got %v", result["Outcome"])
	}
}

func TestAPI_Causal_RootCause_Disabled_503(t *testing.T) {
	dir, _ := os.MkdirTemp("", "immortal-v4-nocausal-*")
	store, _ := storage.New(dir + "/test.db")
	defer func() { store.Close(); os.RemoveAll(dir) }()
	s := rest.NewFull(rest.ServerConfig{
		Store:    store,
		Registry: health.NewRegistry(),
		Healer:   healing.NewHealer(),
	})
	body := `{"outcome":"cpu","variables":["cpu","mem"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v4/causal/root-cause", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// --- v0.4.0 federated tests ---

func TestAPI_Federated_Snapshot(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v4/federated/snapshot?round=1&epsilon=1.0", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var update map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&update)
	if update["ClientID"] == nil {
		t.Error("expected ClientID in federated snapshot response")
	}
}

func TestAPI_Federated_Snapshot_Disabled_404(t *testing.T) {
	dir, _ := os.MkdirTemp("", "immortal-v4-nofed-*")
	store, _ := storage.New(dir + "/test.db")
	defer func() { store.Close(); os.RemoveAll(dir) }()
	s := rest.NewFull(rest.ServerConfig{
		Store:    store,
		Registry: health.NewRegistry(),
		Healer:   healing.NewHealer(),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v4/federated/snapshot?round=0", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

// --- method-not-allowed for v0.4.0 endpoints ---

func TestAPI_V4_MethodNotAllowed(t *testing.T) {
	s, cleanup := setupV4Server(t)
	defer cleanup()

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v4/audit/verify"},
		{http.MethodPost, "/api/v4/audit/merkle-root"},
		{http.MethodPost, "/api/v4/audit/entries"},
		{http.MethodGet, "/api/v4/twin/simulate"},
		{http.MethodPost, "/api/v4/twin/states"},
		{http.MethodPost, "/api/v4/twin/state/svc-a"},
		{http.MethodGet, "/api/v4/agentic/run"},
		{http.MethodGet, "/api/v4/causal/root-cause"},
		{http.MethodPost, "/api/v4/federated/snapshot"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s %s", tc.method, tc.path), func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405, got %d", rec.Code)
			}
		})
	}
}

// testPlanner is a minimal deterministic Planner for the REST agentic tests:
// it always finishes immediately so the loop returns a Resolved trace.
type testPlanner struct{}

func (t *testPlanner) NextStep(ev *event.Event, history []agentic.Step) (string, map[string]any, string, error) {
	return "finish", map[string]any{"reason": "test planner resolved"}, "finished immediately", nil
}

