package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/middleware"
)

func TestChain(t *testing.T) {
	var order []string
	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1")
			next.ServeHTTP(w, r)
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2")
			next.ServeHTTP(w, r)
		})
	}
	chain := middleware.Chain(m1, m2)
	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	if len(order) != 3 || order[0] != "m1" || order[1] != "m2" || order[2] != "handler" {
		t.Errorf("wrong order: %v", order)
	}
}

func TestRecovery(t *testing.T) {
	handler := middleware.Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 500 {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	handler := middleware.CORS("*")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := middleware.CORS("*")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("OPTIONS", "/", nil))
	if rec.Code != 204 {
		t.Errorf("expected 204 for preflight, got %d", rec.Code)
	}
}

func TestAPIKey(t *testing.T) {
	handler := middleware.APIKey("secret123")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	// No key
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 401 {
		t.Errorf("expected 401 without key, got %d", rec.Code)
	}

	// With key in header
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "secret123")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("expected 200 with key, got %d", rec.Code)
	}

	// With key in query
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/?api_key=secret123", nil))
	if rec.Code != 200 {
		t.Errorf("expected 200 with query key, got %d", rec.Code)
	}
}
