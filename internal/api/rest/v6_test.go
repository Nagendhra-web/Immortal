package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/evolve"
	"github.com/Nagendhra-web/Immortal/internal/intent"
	"github.com/Nagendhra-web/Immortal/internal/narrator"
)

type nopMetrics struct{}

func (nopMetrics) Value(string, string) (float64, bool) { return 0, false }

type fakeMetrics map[string]float64

func (f fakeMetrics) Value(service, metric string) (float64, bool) {
	v, ok := f[service+"::"+metric]
	return v, ok
}

// newV6Server builds a Server with only the v0.6 dependencies wired.
// Everything else is nil so unrelated endpoints return 503/empty.
func newV6Server(t *testing.T, metrics intent.MetricProvider) *Server {
	t.Helper()
	return NewFull(ServerConfig{
		Intent:   intent.New(metrics),
		Narrator: narrator.New(),
		Evolve:   evolve.New(),
	})
}

func TestV6Intent_EvaluatorUnset_Returns503(t *testing.T) {
	srv := NewFull(ServerConfig{})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v6/intent", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when evaluator nil, got %d", rec.Code)
	}
}

func TestV6Intent_AddAndList(t *testing.T) {
	srv := newV6Server(t, nopMetrics{})
	// POST a Contract preset serialized to JSON.
	it := intent.ProtectCheckout("checkout")
	body, _ := json.Marshal(it)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v6/intent", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v6/intent returned %d: %s", rec.Code, rec.Body.String())
	}
	// GET should return it with empty Statuses (no metrics).
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v6/intent", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v6/intent returned %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "protect-checkout") {
		t.Errorf("GET response should contain the intent name; got %s", rec.Body.String())
	}
}

func TestV6Intent_Suggest_AppearsWhenViolated(t *testing.T) {
	m := fakeMetrics{"checkout::latency_p99": 300} // over 200 target
	srv := newV6Server(t, m)
	srv.intentEval.AddIntent(intent.ProtectCheckout("checkout"))

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v6/intent/suggest", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("returned %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "scale:") && !strings.Contains(body, "shed_load") {
		t.Errorf("suggestion payload should include a healing action for violated latency; got %s", body)
	}
}

func TestV6Narrator_Explain_Incident(t *testing.T) {
	srv := newV6Server(t, nopMetrics{})
	payload := `{
		"incident": {
			"id": "inc-1",
			"event": {"service": "checkout", "kind": "latency_spike", "metric": "p99", "baseline": 80, "peak": 310, "unit": "ms"},
			"root":  {"driver": "retry storm", "score": 0.9}
		}
	}`
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v6/narrator/explain", strings.NewReader(payload)))
	if rec.Code != http.StatusOK {
		t.Fatalf("returned %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "markdown") {
		t.Errorf("response must include markdown field; got %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Latency spike") {
		t.Errorf("response must describe the incident; got %s", rec.Body.String())
	}
}

func TestV6Narrator_Explain_Verdict(t *testing.T) {
	srv := newV6Server(t, nopMetrics{})
	payload := `{
		"verdict": {
			"Cause":      "Retry storm.",
			"Evidence":   ["retry rate 0.4"],
			"Action":     ["throttled api"],
			"Outcome":    "p99 back to 95ms",
			"Confidence": 0.87
		}
	}`
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v6/narrator/explain", strings.NewReader(payload)))
	if rec.Code != http.StatusOK {
		t.Fatalf("returned %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"brief", "render", "markdown", "87%"} {
		if !strings.Contains(body, want) {
			t.Errorf("verdict response missing %q; body: %s", want, body)
		}
	}
}

func TestV6Evolve_Suggest(t *testing.T) {
	srv := newV6Server(t, nopMetrics{})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v6/evolve/suggest", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("returned %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "suggestions") {
		t.Errorf("response must contain suggestions array; got %s", body)
	}
	// The default demo bag includes retry=0.42 -> should suggest add-retry-budget.
	if !strings.Contains(body, "add-retry-budget") && !strings.Contains(body, "add-cache") {
		t.Errorf("default signals should trigger at least one known suggestion; got %s", body)
	}
}

func TestV6Intent_DeleteByName(t *testing.T) {
	srv := newV6Server(t, nopMetrics{})
	srv.intentEval.AddIntent(intent.Intent{Name: "demo", Goals: []intent.Goal{{Kind: intent.LatencyUnder, Target: 200}}})
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v6/intent/demo", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE returned %d: %s", rec.Code, rec.Body.String())
	}
	if n := len(srv.intentEval.List()); n != 0 {
		t.Errorf("intent should be removed; %d remaining", n)
	}
}
