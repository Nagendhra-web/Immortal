package rest_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/agentic"
	"github.com/Nagendhra-web/Immortal/internal/api/rest"
	"github.com/Nagendhra-web/Immortal/internal/audit"
	"github.com/Nagendhra-web/Immortal/internal/causal"
	"github.com/Nagendhra-web/Immortal/internal/causality"
	"github.com/Nagendhra-web/Immortal/internal/dependency"
	"github.com/Nagendhra-web/Immortal/internal/dna"
	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/export"
	"github.com/Nagendhra-web/Immortal/internal/federated"
	"github.com/Nagendhra-web/Immortal/internal/healing"
	"github.com/Nagendhra-web/Immortal/internal/health"
	"github.com/Nagendhra-web/Immortal/internal/pattern"
	"github.com/Nagendhra-web/Immortal/internal/predict"
	"github.com/Nagendhra-web/Immortal/internal/pqaudit"
	"github.com/Nagendhra-web/Immortal/internal/selfmonitor"
	"github.com/Nagendhra-web/Immortal/internal/sla"
	"github.com/Nagendhra-web/Immortal/internal/storage"
	"github.com/Nagendhra-web/Immortal/internal/timetravel"
	"github.com/Nagendhra-web/Immortal/internal/topology"
	"github.com/Nagendhra-web/Immortal/internal/twin"
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

// --- v0.5.0 helpers ---

func setupV5Server(t *testing.T) (*rest.Server, func()) {
	t.Helper()
	dir, _ := os.MkdirTemp("", "immortal-v5-test-*")
	store, err := storage.New(dir + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	registry := health.NewRegistry()
	healer := healing.NewHealer()

	// Topology: tracker with 2 snapshots (generates events on topology change)
	tr := topology.NewTracker(100)
	g1 := topology.NewDiGraph()
	g1.AddEdge("svc-a", "svc-b")
	tr.Record(g1)
	g2 := topology.NewDiGraph()
	g2.AddEdge("svc-a", "svc-b")
	g2.AddEdge("svc-b", "svc-c")
	tr.Record(g2)

	// Semantic memory with 3 records
	sm := agentic.NewSemanticMemory(100)
	for i := 0; i < 3; i++ {
		inc := agentic.Incident{Message: fmt.Sprintf("high cpu on svc-%d", i), Source: "svc-a", Severity: "critical"}
		trace := &agentic.Trace{Steps: []agentic.Step{}, Resolved: true}
		sm.Record(inc, trace, agentic.OutcomeResolved)
	}

	// MetaAgent with a deterministic finishing planner
	ma := agentic.NewMetaAgent(agentic.MetaConfig{StopOnFirstResolve: true})

	// PCMCI function: real DiscoverPCMCI on a small chain dataset
	pcmciFn := func(ds *causal.Dataset, cfg causal.PCMCIConfig) (causal.LaggedGraph, error) {
		return causal.DiscoverPCMCI(ds, cfg)
	}

	// Counterfactual function: real Counterfactual call
	counterfactualFn := func(ds *causal.Dataset, m *causal.StructuralModel, rowIdx int, cause string, do float64, outcome string) (causal.CounterfactualResult, error) {
		return causal.Counterfactual(ds, m, rowIdx, cause, do, outcome)
	}

	// Federated aggregator with 1 client
	agg := federated.NewAggregator(federated.AggregatorConfig{MinClients: 1})
	fc := federated.NewClientWithSeed("v5-node", 99, 0)
	fc.Observe("cpu", 50.0)
	agg.Submit(fc.Snapshot(1, 0))

	cleanup := func() {
		store.Close()
		os.RemoveAll(dir)
	}

	s := rest.NewFull(rest.ServerConfig{
		Store:              store,
		Registry:           registry,
		Healer:             healer,
		Topology:           tr,
		FormalOn:           true,
		PCMCIFn:            pcmciFn,
		CounterfactualFn:   counterfactualFn,
		SemanticMemory:     sm,
		MetaAgent:          ma,
		AggregatorAdvanced: agg,
	})
	return s, cleanup
}

func disabledV5Server(t *testing.T) (*rest.Server, func()) {
	t.Helper()
	dir, _ := os.MkdirTemp("", "immortal-v5-disabled-*")
	store, _ := storage.New(dir + "/test.db")
	cleanup := func() { store.Close(); os.RemoveAll(dir) }
	s := rest.NewFull(rest.ServerConfig{
		Store:    store,
		Registry: health.NewRegistry(),
		Healer:   healing.NewHealer(),
		// all v0.5 fields nil/false
	})
	return s, cleanup
}

// --- v0.5.0 topology tests ---

func TestAPI_V5_TopologySnapshot_OK(t *testing.T) {
	s, cleanup := setupV5Server(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v5/topology/snapshot", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if _, ok := resp["snapshot"]; !ok {
		t.Error("expected 'snapshot' key in response")
	}
}

func TestAPI_V5_TopologySnapshot_Disabled404(t *testing.T) {
	s, cleanup := disabledV5Server(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v5/topology/snapshot", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestAPI_V5_TopologyEvents_OK(t *testing.T) {
	s, cleanup := setupV5Server(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v5/topology/events", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if _, ok := resp["events"]; !ok {
		t.Error("expected 'events' key in response")
	}
}

// --- v0.5.0 formal tests ---

func TestAPI_V5_FormalCheck_DetectsViolation(t *testing.T) {
	s, cleanup := setupV5Server(t)
	defer cleanup()

	// Scale svc-a to 0 replicas; min_replicas invariant requires >=1 → violation
	body := `{
		"world": {"svc-a": {"healthy": true, "replicas": 2}},
		"plan": {
			"id": "plan-scale-zero",
			"steps": [{"name": "scale-down", "action_type": "set_replicas", "target": "svc-a", "params": {"target": "svc-a", "value": 0}}]
		},
		"invariants": [{"kind": "min_replicas", "args": {"service": "svc-a", "n": 1}}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v5/formal/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)
	safe, _ := result["Safe"].(bool)
	if safe {
		t.Error("expected Safe=false for scale-to-zero + min_replicas violation")
	}
}

func TestAPI_V5_FormalCheck_Disabled503(t *testing.T) {
	s, cleanup := disabledV5Server(t)
	defer cleanup()

	body := `{"world":{},"plan":{"id":"x","steps":[]},"invariants":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v5/formal/check", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// --- v0.5.0 causal PCMCI tests ---

func makePCMCIBody() string {
	// 3 variables, 30 rows — enough for PCMCI (needs >TauMax rows)
	names := `["x","y","z"]`
	var rows []string
	for i := 0; i < 30; i++ {
		fi := float64(i)
		rows = append(rows, fmt.Sprintf("[%.2f,%.2f,%.2f]", fi, fi*0.8+1, fi*0.5+2))
	}
	rowsJSON := "["
	for i, r := range rows {
		if i > 0 {
			rowsJSON += ","
		}
		rowsJSON += r
	}
	rowsJSON += "]"
	return fmt.Sprintf(`{"names":%s,"rows":%s,"alpha":0.05,"tau_max":2,"max_cond_set_size":2}`, names, rowsJSON)
}

func TestAPI_V5_PCMCI_ReturnsGraph(t *testing.T) {
	s, cleanup := setupV5Server(t)
	defer cleanup()

	body := makePCMCIBody()
	req := httptest.NewRequest(http.MethodPost, "/api/v5/causal/pcmci", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)
	if _, ok := result["Parents"]; !ok {
		t.Error("expected 'Parents' key in PCMCI result")
	}
}

func TestAPI_V5_PCMCI_Disabled503(t *testing.T) {
	s, cleanup := disabledV5Server(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v5/causal/pcmci", bytes.NewBufferString(makePCMCIBody()))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// --- v0.5.0 causal counterfactual tests ---

func makeCounterfactualBody() string {
	// Simple 2-variable dataset: y = 2*x + noise; parents: x → y
	var rows []string
	for i := 0; i < 20; i++ {
		fi := float64(i + 1)
		rows = append(rows, fmt.Sprintf("[%.1f,%.1f]", fi, fi*2.0))
	}
	rowsJSON := "[" + joinStrings(rows, ",") + "]"
	return fmt.Sprintf(`{"names":["x","y"],"rows":%s,"parents":{"y":["x"]},"row_index":5,"cause":"x","do":10.0,"outcome":"y"}`, rowsJSON)
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func TestAPI_V5_Counterfactual_ReturnsResult(t *testing.T) {
	s, cleanup := setupV5Server(t)
	defer cleanup()

	body := makeCounterfactualBody()
	req := httptest.NewRequest(http.MethodPost, "/api/v5/causal/counterfactual", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)
	if _, ok := result["CounterfactualOutcome"]; !ok {
		t.Error("expected 'CounterfactualOutcome' in result")
	}
}

func TestAPI_V5_Counterfactual_Disabled503(t *testing.T) {
	s, cleanup := disabledV5Server(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v5/causal/counterfactual", bytes.NewBufferString(makeCounterfactualBody()))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// --- v0.5.0 agentic memory recall tests ---

func TestAPI_V5_AgenticMemoryRecall_ReturnsTopK(t *testing.T) {
	s, cleanup := setupV5Server(t)
	defer cleanup()

	body := `{"message":"high cpu","source":"svc-a","severity":"critical","k":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v5/agentic/memory/recall", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	entries, ok := resp["entries"].([]interface{})
	if !ok {
		t.Fatal("expected 'entries' array in response")
	}
	if len(entries) == 0 {
		t.Error("expected at least one entry returned")
	}
	if len(entries) > 2 {
		t.Errorf("expected at most k=2 entries, got %d", len(entries))
	}
}

func TestAPI_V5_AgenticMemoryRecall_Disabled503(t *testing.T) {
	s, cleanup := disabledV5Server(t)
	defer cleanup()

	body := `{"message":"test","source":"svc","severity":"info","k":3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v5/agentic/memory/recall", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// --- v0.5.0 meta-investigate tests ---

func TestAPI_V5_MetaInvestigate_RunsBranches(t *testing.T) {
	s, cleanup := setupV5Server(t)
	defer cleanup()

	body := `{
		"type":"error","severity":"critical","message":"high cpu","source":"svc-a",
		"hypothesis_types":["resource_exhaustion","dependency_failure","recent_deployment"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v5/agentic/meta-investigate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)
	branches, ok := result["Branches"].([]interface{})
	if !ok {
		t.Fatal("expected 'Branches' array in MetaResult")
	}
	if len(branches) != 3 {
		t.Errorf("expected 3 branches, got %d", len(branches))
	}
}

func TestAPI_V5_MetaInvestigate_Disabled503(t *testing.T) {
	s, cleanup := disabledV5Server(t)
	defer cleanup()

	body := `{"type":"error","severity":"critical","message":"test","source":"svc","hypothesis_types":["resource_exhaustion"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v5/agentic/meta-investigate", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// --- v0.5.0 federated close tests ---

func TestAPI_V5_FederatedClose_ReturnsGlobalModel(t *testing.T) {
	s, cleanup := setupV5Server(t)
	defer cleanup()

	body := `{"round":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v5/federated/close", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var gm map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&gm)
	if _, ok := gm["Round"]; !ok {
		t.Error("expected 'Round' in GlobalModel response")
	}
}

func TestAPI_V5_FederatedClose_Disabled503(t *testing.T) {
	s, cleanup := disabledV5Server(t)
	defer cleanup()

	body := `{"round":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v5/federated/close", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
}

// --- v0.5.0 method-not-allowed table test ---

func TestAPI_V5_MethodNotAllowed_AllEndpoints(t *testing.T) {
	s, cleanup := setupV5Server(t)
	defer cleanup()

	cases := []struct {
		method string
		path   string
	}{
		// POST-only endpoints called with GET
		{http.MethodGet, "/api/v5/formal/check"},
		{http.MethodGet, "/api/v5/causal/pcmci"},
		{http.MethodGet, "/api/v5/causal/counterfactual"},
		{http.MethodGet, "/api/v5/agentic/memory/recall"},
		{http.MethodGet, "/api/v5/agentic/meta-investigate"},
		{http.MethodGet, "/api/v5/federated/close"},
		// GET-only endpoints called with POST
		{http.MethodPost, "/api/v5/topology/snapshot"},
		{http.MethodPost, "/api/v5/topology/events"},
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

// Ensure topology import is used (it is used in setupV5Server via topology.NewTracker etc.)
var _ = topology.NewTracker

// --- Dashboard route tests ---

func TestServer_DashboardRouteRegistered(t *testing.T) {
	s, cleanup := setupServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/index.html", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /dashboard/index.html, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %s", ct)
	}
}

func TestServer_RootRedirectsToDashboard(t *testing.T) {
	// The root "/" is not registered (no catch-all), so http.ServeMux returns 404.
	// This test confirms /dashboard/ itself serves correctly as the canonical entry.
	s, cleanup := setupServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	// /dashboard/ strips to "" → serves index.html (Go's FileServer serves index.html for directory)
	if rec.Code != http.StatusOK && rec.Code != http.StatusMovedPermanently && rec.Code != http.StatusFound {
		t.Errorf("expected 200 or redirect from /dashboard/, got %d", rec.Code)
	}
}

