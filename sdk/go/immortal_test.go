package immortal_test

import (
	"sync/atomic"
	"testing"
	"time"

	immortal "github.com/Nagendhra-web/Immortal/sdk/go"
	"github.com/Nagendhra-web/Immortal/internal/event"
	"github.com/Nagendhra-web/Immortal/internal/healing"
)

func TestNewApp(t *testing.T) {
	app := immortal.New(immortal.Config{Name: "test"})
	if app.Config().Name != "test" {
		t.Error("wrong name")
	}
	if app.IsRunning() {
		t.Error("should not be running")
	}
}

func TestStartStop(t *testing.T) {
	app := immortal.New(immortal.Config{})
	app.Start()
	if !app.IsRunning() {
		t.Error("should be running")
	}
	app.Stop()
	if app.IsRunning() {
		t.Error("should be stopped")
	}
}

func TestHealAndIngest(t *testing.T) {
	app := immortal.New(immortal.Config{})
	var healed atomic.Bool
	app.Heal(healing.Rule{
		Name:   "test",
		Match:  immortal.MatchSeverity(event.SeverityCritical),
		Action: func(e *event.Event) error { healed.Store(true); return nil },
	})
	app.Start()
	defer app.Stop()
	app.Ingest(event.New(event.TypeError, event.SeverityCritical, "crash"))
	time.Sleep(200 * time.Millisecond)
	if !healed.Load() {
		t.Error("should have healed")
	}
}

func TestMetrics(t *testing.T) {
	app := immortal.New(immortal.Config{Name: "test"})
	// Use varied baseline so StdDev > 0
	for i := 0; i < 100; i++ {
		app.RecordMetric("cpu", 43.0+float64(i%5))
	}
	if app.IsAnomaly("cpu", 44.0) {
		t.Error("44 should not be anomaly for baseline of 43-47")
	}
}

func TestHealthScore(t *testing.T) {
	app := immortal.New(immortal.Config{})
	for i := 0; i < 100; i++ {
		app.RecordMetric("cpu", 40.0+float64(i%10))
	}
	score := app.HealthScore(map[string]float64{"cpu": 45.0})
	if score < 0.5 {
		t.Errorf("expected good health score, got %f", score)
	}
}

func TestDefaultConfig(t *testing.T) {
	app := immortal.New(immortal.Config{})
	if app.Config().Name != "immortal-app" {
		t.Error("wrong default name")
	}
	if app.Config().Mode != "reactive" {
		t.Error("wrong default mode")
	}
}
