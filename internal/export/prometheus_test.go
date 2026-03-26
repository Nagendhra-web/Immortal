package export_test

import (
	"strings"
	"testing"

	"github.com/immortal-engine/immortal/internal/export"
)

func TestGauge(t *testing.T) {
	p := export.NewPrometheus()
	p.SetGauge("cpu_usage", 45.5)
	if p.Get("cpu_usage") != 45.5 {
		t.Error("wrong gauge value")
	}
	output := p.Export()
	if !strings.Contains(output, "cpu_usage 45.5") {
		t.Error("export missing gauge")
	}
	if !strings.Contains(output, "# TYPE cpu_usage gauge") {
		t.Error("missing type line")
	}
}

func TestCounter(t *testing.T) {
	p := export.NewPrometheus()
	p.IncCounter("requests_total")
	p.IncCounter("requests_total")
	p.IncCounter("requests_total")
	if p.Get("requests_total") != 3 {
		t.Errorf("expected 3, got %g", p.Get("requests_total"))
	}
}

func TestAddCounter(t *testing.T) {
	p := export.NewPrometheus()
	p.AddCounter("bytes_total", 1024)
	p.AddCounter("bytes_total", 2048)
	if p.Get("bytes_total") != 3072 {
		t.Error("wrong counter value")
	}
}

func TestExportFormat(t *testing.T) {
	p := export.NewPrometheus()
	p.SetGauge("mem", 60)
	p.IncCounter("errors")
	output := p.Export()
	if !strings.Contains(output, "# TYPE") {
		t.Error("missing TYPE declaration")
	}
}

func TestGetNonexistent(t *testing.T) {
	p := export.NewPrometheus()
	if p.Get("nope") != 0 {
		t.Error("nonexistent should return 0")
	}
}
