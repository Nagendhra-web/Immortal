package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/api/dashboard"
)

func TestDashboardHandler_ServesIndexHTML(t *testing.T) {
	h := dashboard.Handler()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/index.html", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html content-type, got %s", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Immortal") {
		t.Errorf("expected body to contain 'Immortal', got: %s", body[:min(200, len(body))])
	}
}

func TestDashboardHandler_ServesCSS(t *testing.T) {
	h := dashboard.Handler()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/app.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/css") {
		t.Errorf("expected text/css content-type, got %s", ct)
	}
}

func TestDashboardHandler_ServesJS(t *testing.T) {
	h := dashboard.Handler()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/app.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") {
		t.Errorf("expected javascript content-type, got %s", ct)
	}
}

func TestDashboardHandler_404OnUnknownPath(t *testing.T) {
	h := dashboard.Handler()
	req := httptest.NewRequest(http.MethodGet, "/dashboard/does-not-exist.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
