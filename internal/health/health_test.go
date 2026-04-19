package health_test

import (
	"testing"

	"github.com/Nagendhra-web/Immortal/internal/health"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := health.NewRegistry()
	r.Register("api-server")

	svc := r.Get("api-server")
	if svc == nil {
		t.Fatal("expected service")
	}
	if svc.Status != health.StatusUnknown {
		t.Errorf("expected unknown status, got %s", svc.Status)
	}
}

func TestRegistryUpdate(t *testing.T) {
	r := health.NewRegistry()
	r.Register("api")
	r.Update("api", health.StatusHealthy, "all good")

	svc := r.Get("api")
	if svc.Status != health.StatusHealthy {
		t.Errorf("expected healthy, got %s", svc.Status)
	}
	if svc.Checks != 1 {
		t.Errorf("expected 1 check, got %d", svc.Checks)
	}
}

func TestOverallStatus(t *testing.T) {
	r := health.NewRegistry()
	r.Register("api")
	r.Register("db")

	r.Update("api", health.StatusHealthy, "ok")
	r.Update("db", health.StatusHealthy, "ok")
	if r.OverallStatus() != health.StatusHealthy {
		t.Error("expected overall healthy")
	}

	r.Update("db", health.StatusUnhealthy, "down")
	if r.OverallStatus() != health.StatusUnhealthy {
		t.Error("expected overall unhealthy")
	}
}

func TestUptimePercent(t *testing.T) {
	r := health.NewRegistry()
	r.Register("api")

	r.Update("api", health.StatusHealthy, "ok")
	r.Update("api", health.StatusHealthy, "ok")
	r.Update("api", health.StatusUnhealthy, "down")
	r.Update("api", health.StatusHealthy, "ok")

	uptime := r.UptimePercent("api")
	if uptime != 75.0 {
		t.Errorf("expected 75%% uptime, got %.1f%%", uptime)
	}
}

func TestGetNonexistent(t *testing.T) {
	r := health.NewRegistry()
	if r.Get("fake") != nil {
		t.Error("expected nil for nonexistent service")
	}
}

func TestAllServices(t *testing.T) {
	r := health.NewRegistry()
	r.Register("a")
	r.Register("b")
	r.Register("c")

	all := r.All()
	if len(all) != 3 {
		t.Errorf("expected 3 services, got %d", len(all))
	}
}
