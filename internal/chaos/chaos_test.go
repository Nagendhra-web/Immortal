package chaos_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/chaos"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestNewEngine(t *testing.T) {
	e := chaos.New(nil)
	if e == nil {
		t.Fatal("expected engine")
	}
}

func TestInjectHTTPError(t *testing.T) {
	var received *event.Event
	e := chaos.New(func(ev *event.Event) { received = ev })

	f := e.InjectHTTPError("api-server", 500)
	if f == nil {
		t.Fatal("expected fault")
	}
	if f.Type != "http_error" {
		t.Errorf("expected http_error, got %s", f.Type)
	}
	if f.Target != "api-server" {
		t.Errorf("expected api-server, got %s", f.Target)
	}
	if received == nil {
		t.Fatal("callback not called")
	}
}

func TestInjectProcessCrash(t *testing.T) {
	called := false
	e := chaos.New(func(ev *event.Event) { called = true })

	f := e.InjectProcessCrash("nginx")
	if f.Type != "process_crash" {
		t.Errorf("expected process_crash, got %s", f.Type)
	}
	if !called {
		t.Error("callback not called")
	}
}

func TestInjectCPUSpike(t *testing.T) {
	e := chaos.New(func(ev *event.Event) {})
	f := e.InjectCPUSpike(95.0)
	if f.Type != "cpu_spike" {
		t.Errorf("expected cpu_spike, got %s", f.Type)
	}
}

func TestInjectCustom(t *testing.T) {
	e := chaos.New(func(ev *event.Event) {})
	f := e.InjectCustom("disk_full", "storage", "disk full chaos", event.SeverityCritical)
	if f.Type != "disk_full" {
		t.Errorf("expected disk_full, got %s", f.Type)
	}
	if f.Target != "storage" {
		t.Errorf("expected storage, got %s", f.Target)
	}
}

func TestRecordResult(t *testing.T) {
	e := chaos.New(func(ev *event.Event) {})
	f := e.InjectHTTPError("api", 500)
	e.RecordResult(f.ID, true, 100*time.Millisecond, true, 200*time.Millisecond)

	results := e.Results()
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Detected {
		t.Error("expected detected")
	}
	if !results[0].Healed {
		t.Error("expected healed")
	}
}

func TestScore(t *testing.T) {
	e := chaos.New(func(ev *event.Event) {})

	// 5 faults: detect 4, heal 3
	for i := 0; i < 5; i++ {
		f := e.InjectHTTPError("api", 500)
		detected := i < 4
		healed := i < 3
		e.RecordResult(f.ID, detected, time.Millisecond, healed, time.Millisecond)
	}

	score := e.Score()
	// (4*0.4 + 3*0.6) / 5 = (1.6 + 1.8) / 5 = 0.68
	if score < 0.67 || score > 0.69 {
		t.Errorf("expected score ~0.68, got %.2f", score)
	}
}

func TestActiveFaults(t *testing.T) {
	e := chaos.New(func(ev *event.Event) {})
	f1 := e.InjectHTTPError("api", 500)
	e.InjectProcessCrash("nginx")

	active := e.ActiveFaults()
	if len(active) != 2 {
		t.Fatalf("expected 2 active, got %d", len(active))
	}

	e.ClearFault(f1.ID)
	active = e.ActiveFaults()
	if len(active) != 1 {
		t.Fatalf("expected 1 active, got %d", len(active))
	}
}

func TestReport(t *testing.T) {
	e := chaos.New(func(ev *event.Event) {})
	f := e.InjectHTTPError("api", 500)
	e.RecordResult(f.ID, true, time.Millisecond, true, time.Millisecond)

	report := e.Report()
	if report["total_faults"].(int) != 1 {
		t.Error("expected 1 total fault")
	}
	if report["detected"].(int) != 1 {
		t.Error("expected 1 detected")
	}
}

func TestReset(t *testing.T) {
	e := chaos.New(func(ev *event.Event) {})
	e.InjectHTTPError("api", 500)
	e.Reset()

	if len(e.ActiveFaults()) != 0 {
		t.Error("expected 0 faults after reset")
	}
	if len(e.Results()) != 0 {
		t.Error("expected 0 results after reset")
	}
}
