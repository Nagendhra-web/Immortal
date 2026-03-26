package report_test

import (
	"strings"
	"testing"

	"github.com/immortal-engine/immortal/internal/analytics/metrics"
	"github.com/immortal-engine/immortal/internal/analytics/report"
	"github.com/immortal-engine/immortal/internal/health"
)

func TestReportGeneration(t *testing.T) {
	agg := metrics.New(1000)
	reg := health.NewRegistry()

	reg.Register("api")
	reg.Update("api", health.StatusHealthy, "ok")

	agg.Record("cpu", 45.0)
	agg.Record("cpu", 50.0)
	agg.Record("cpu", 55.0)

	gen := report.NewGenerator(agg, reg)
	r := gen.Generate("Daily Report")

	if r.Title != "Daily Report" {
		t.Errorf("expected 'Daily Report', got '%s'", r.Title)
	}
	if len(r.Sections) < 2 {
		t.Errorf("expected at least 2 sections, got %d", len(r.Sections))
	}
}

func TestReportString(t *testing.T) {
	agg := metrics.New(1000)
	reg := health.NewRegistry()
	reg.Register("api")
	reg.Update("api", health.StatusHealthy, "running")

	gen := report.NewGenerator(agg, reg)
	r := gen.Generate("Test Report")

	output := r.String()
	if !strings.Contains(output, "Test Report") {
		t.Error("report should contain title")
	}
	if !strings.Contains(output, "Health") {
		t.Error("report should contain health section")
	}
}

func TestReportWithMultipleServices(t *testing.T) {
	agg := metrics.New(1000)
	reg := health.NewRegistry()

	reg.Register("api")
	reg.Register("db")
	reg.Register("cache")
	reg.Update("api", health.StatusHealthy, "ok")
	reg.Update("db", health.StatusDegraded, "slow")
	reg.Update("cache", health.StatusHealthy, "ok")

	gen := report.NewGenerator(agg, reg)
	r := gen.Generate("Multi-Service Report")

	output := r.String()
	if !strings.Contains(output, "api") {
		t.Error("should mention api service")
	}
	if !strings.Contains(output, "db") {
		t.Error("should mention db service")
	}
}
