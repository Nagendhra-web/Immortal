package landing_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/immortal-engine/immortal/internal/web/landing"
)

func TestLanding_ServesIndex(t *testing.T) {
	h := landing.Handler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("want text/html, got %s", ct)
	}
	if !strings.Contains(rec.Body.String(), "Immortal") {
		t.Fatalf("body missing 'Immortal'")
	}
}

func TestLanding_ServesJS(t *testing.T) {
	h := landing.Handler()
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Fatalf("want application/javascript, got %s", ct)
	}
}

func TestLanding_ServesCSS(t *testing.T) {
	h := landing.Handler()
	req := httptest.NewRequest(http.MethodGet, "/app.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Fatalf("want text/css, got %s", ct)
	}
}
