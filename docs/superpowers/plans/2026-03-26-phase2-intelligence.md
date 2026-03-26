# Phase 2: Immortal Intelligence — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the intelligence layer to Immortal — health fingerprinting (Healing DNA), predictive healing, causality tracking, time-travel debugging, simulation sandbox, and consensus verification. This transforms Immortal from a reactive healer into a predictive, intelligent engine.

**Architecture:** Healing DNA establishes per-app health baselines using rolling statistics. Predictive healer uses anomaly detection on metrics to predict failures before they happen. Causality Graph tracks event chains to find root causes. Time-Travel stores state snapshots for replay. Simulation Sandbox tests fixes in isolation. Consensus requires multiple detection methods to agree before acting.

**Tech Stack:** Go (all components), embedded SQLite (state), rolling statistics (anomaly detection), directed acyclic graph (causality)

---

## Task 1: Healing DNA — Health Baseline

**Files:**
- Create: `internal/dna/dna.go`
- Create: `internal/dna/dna_test.go`

- [ ] **Step 1: Write failing test for Healing DNA**

Create `internal/dna/dna_test.go`:

```go
package dna_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/dna"
)

func TestDNARecordAndBaseline(t *testing.T) {
	d := dna.New("api-server")

	// Feed it normal metrics
	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 45.0+float64(i%10))
		d.Record("memory_percent", 60.0+float64(i%5))
		d.Record("response_time_ms", 120.0+float64(i%20))
	}

	baseline := d.Baseline()
	if baseline == nil {
		t.Fatal("expected non-nil baseline")
	}

	cpuBaseline, ok := baseline["cpu_percent"]
	if !ok {
		t.Fatal("expected cpu_percent in baseline")
	}
	if cpuBaseline.Mean < 44 || cpuBaseline.Mean > 55 {
		t.Errorf("cpu mean out of range: %f", cpuBaseline.Mean)
	}
	if cpuBaseline.StdDev <= 0 {
		t.Error("expected positive std dev")
	}
}

func TestDNADetectsAnomaly(t *testing.T) {
	d := dna.New("api-server")

	// Establish normal baseline
	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 45.0+float64(i%10))
	}

	// Normal value — should NOT be anomaly
	if d.IsAnomaly("cpu_percent", 48.0) {
		t.Error("48.0 should not be anomaly for cpu baseline ~45-55")
	}

	// Extreme value — SHOULD be anomaly
	if !d.IsAnomaly("cpu_percent", 99.0) {
		t.Error("99.0 should be anomaly for cpu baseline ~45-55")
	}
}

func TestDNAHealthScore(t *testing.T) {
	d := dna.New("api-server")

	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 45.0)
		d.Record("memory_percent", 60.0)
	}

	// All normal — health should be high
	score := d.HealthScore(map[string]float64{
		"cpu_percent":    46.0,
		"memory_percent": 61.0,
	})
	if score < 0.8 {
		t.Errorf("expected high health score for normal values, got %f", score)
	}

	// All anomalous — health should be low
	score = d.HealthScore(map[string]float64{
		"cpu_percent":    99.0,
		"memory_percent": 99.0,
	})
	if score > 0.5 {
		t.Errorf("expected low health score for anomalous values, got %f", score)
	}
}

func TestDNADecay(t *testing.T) {
	d := dna.NewWithWindow("api-server", 10)

	// Fill window
	for i := 0; i < 10; i++ {
		d.Record("cpu", 50.0)
	}

	// Now feed higher values — baseline should shift
	for i := 0; i < 10; i++ {
		d.Record("cpu", 90.0)
	}

	baseline := d.Baseline()
	if baseline["cpu"].Mean < 70 {
		t.Errorf("expected baseline to shift toward 90, got mean %f", baseline["cpu"].Mean)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -v ./internal/dna/...
```

Expected: FAIL — package doesn't exist

- [ ] **Step 3: Implement Healing DNA**

Create `internal/dna/dna.go`:

```go
package dna

import (
	"math"
	"sync"
)

// MetricStats holds rolling statistics for a single metric.
type MetricStats struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Count  int     `json:"count"`
}

// DNA represents the health fingerprint of an application.
// It learns what "normal" looks like from observed metrics.
type DNA struct {
	mu       sync.RWMutex
	source   string
	window   int
	metrics  map[string]*rollingWindow
}

type rollingWindow struct {
	values []float64
	size   int
}

func newRollingWindow(size int) *rollingWindow {
	return &rollingWindow{
		values: make([]float64, 0, size),
		size:   size,
	}
}

func (rw *rollingWindow) Add(v float64) {
	if len(rw.values) >= rw.size {
		rw.values = rw.values[1:]
	}
	rw.values = append(rw.values, v)
}

func (rw *rollingWindow) Stats() MetricStats {
	n := len(rw.values)
	if n == 0 {
		return MetricStats{}
	}

	sum := 0.0
	min := rw.values[0]
	max := rw.values[0]
	for _, v := range rw.values {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	mean := sum / float64(n)

	variance := 0.0
	for _, v := range rw.values {
		diff := v - mean
		variance += diff * diff
	}
	if n > 1 {
		variance /= float64(n - 1)
	}

	return MetricStats{
		Mean:   mean,
		StdDev: math.Sqrt(variance),
		Min:    min,
		Max:    max,
		Count:  n,
	}
}

// New creates a new DNA with default window size of 1000.
func New(source string) *DNA {
	return NewWithWindow(source, 1000)
}

// NewWithWindow creates a new DNA with a custom rolling window size.
func NewWithWindow(source string, window int) *DNA {
	return &DNA{
		source:  source,
		window:  window,
		metrics: make(map[string]*rollingWindow),
	}
}

// Record adds a metric observation to the DNA.
func (d *DNA) Record(metric string, value float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	rw, ok := d.metrics[metric]
	if !ok {
		rw = newRollingWindow(d.window)
		d.metrics[metric] = rw
	}
	rw.Add(value)
}

// Baseline returns the current health baseline for all recorded metrics.
func (d *DNA) Baseline() map[string]MetricStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]MetricStats)
	for name, rw := range d.metrics {
		result[name] = rw.Stats()
	}
	return result
}

// IsAnomaly checks if a value is anomalous for the given metric.
// Uses 3-sigma rule: anomaly if value is more than 3 standard deviations from mean.
func (d *DNA) IsAnomaly(metric string, value float64) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rw, ok := d.metrics[metric]
	if !ok || len(rw.values) < 10 {
		return false // Not enough data to determine
	}

	stats := rw.Stats()
	if stats.StdDev == 0 {
		return value != stats.Mean
	}

	zScore := math.Abs(value-stats.Mean) / stats.StdDev
	return zScore > 3.0
}

// HealthScore computes an overall health score (0.0 to 1.0) given current metric values.
// 1.0 = perfectly healthy, 0.0 = completely unhealthy.
func (d *DNA) HealthScore(current map[string]float64) float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(current) == 0 {
		return 1.0
	}

	totalScore := 0.0
	count := 0

	for metric, value := range current {
		rw, ok := d.metrics[metric]
		if !ok || len(rw.values) < 10 {
			continue
		}

		stats := rw.Stats()
		if stats.StdDev == 0 {
			if value == stats.Mean {
				totalScore += 1.0
			}
			count++
			continue
		}

		zScore := math.Abs(value-stats.Mean) / stats.StdDev
		// Convert z-score to health: z=0 → 1.0, z=3 → 0.0
		score := math.Max(0, 1.0-zScore/3.0)
		totalScore += score
		count++
	}

	if count == 0 {
		return 1.0
	}
	return totalScore / float64(count)
}

// Source returns the source name.
func (d *DNA) Source() string {
	return d.source
}
```

- [ ] **Step 4: Run tests**

```bash
go test -v ./internal/dna/...
```

Expected: All 4 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/dna/
git commit -m "feat: add Healing DNA — health baseline fingerprinting with anomaly detection"
```

---

## Task 2: Predictive Healer — Anomaly-Based Prediction

**Files:**
- Create: `internal/brain/predictive.go`
- Create: `internal/brain/predictive_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/brain/predictive_test.go`:

```go
package brain_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/brain"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestPredictiveHealerDetectsTrend(t *testing.T) {
	d := dna.New("api-server")

	// Establish normal baseline
	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 45.0)
		d.Record("memory_percent", 60.0)
	}

	ph := brain.NewPredictiveHealer(d)

	// Feed gradually increasing CPU — should detect upward trend
	for i := 0; i < 20; i++ {
		e := event.New(event.TypeMetric, event.SeverityInfo, "cpu metric").
			WithSource("api-server").
			WithMeta("cpu_percent", 45.0+float64(i)*3)
		ph.Observe(e)
	}

	predictions := ph.Predict()
	if len(predictions) == 0 {
		t.Error("expected predictions for rising CPU trend")
	}

	foundCPU := false
	for _, p := range predictions {
		if p.Metric == "cpu_percent" {
			foundCPU = true
			if p.Direction != brain.TrendUp {
				t.Errorf("expected upward trend, got %s", p.Direction)
			}
		}
	}
	if !foundCPU {
		t.Error("expected cpu_percent prediction")
	}
}

func TestPredictiveHealerStableNoAlert(t *testing.T) {
	d := dna.New("api-server")

	for i := 0; i < 100; i++ {
		d.Record("cpu_percent", 45.0)
	}

	ph := brain.NewPredictiveHealer(d)

	// Feed stable values — should NOT predict failure
	for i := 0; i < 20; i++ {
		e := event.New(event.TypeMetric, event.SeverityInfo, "cpu metric").
			WithSource("api-server").
			WithMeta("cpu_percent", 45.0+float64(i%3))
		ph.Observe(e)
	}

	predictions := ph.Predict()
	for _, p := range predictions {
		if p.Metric == "cpu_percent" && p.Risk > 0.5 {
			t.Errorf("stable metrics should have low risk, got %f", p.Risk)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -v ./internal/brain/...
```

- [ ] **Step 3: Implement predictive healer**

Create `internal/brain/predictive.go`:

```go
package brain

import (
	"sync"

	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
)

// TrendDirection indicates the direction of a metric trend.
type TrendDirection string

const (
	TrendUp     TrendDirection = "up"
	TrendDown   TrendDirection = "down"
	TrendStable TrendDirection = "stable"
)

// Prediction represents a predicted future state.
type Prediction struct {
	Metric    string         `json:"metric"`
	Direction TrendDirection `json:"direction"`
	Risk      float64        `json:"risk"` // 0.0 = no risk, 1.0 = certain failure
	Message   string         `json:"message"`
}

// PredictiveHealer observes metric events and predicts failures using trend analysis.
type PredictiveHealer struct {
	mu      sync.RWMutex
	dna     *dna.DNA
	history map[string][]float64 // recent values per metric
	window  int
}

// NewPredictiveHealer creates a predictive healer backed by a DNA baseline.
func NewPredictiveHealer(d *dna.DNA) *PredictiveHealer {
	return &PredictiveHealer{
		dna:     d,
		history: make(map[string][]float64),
		window:  20,
	}
}

// Observe records a metric event for trend analysis.
func (ph *PredictiveHealer) Observe(e *event.Event) {
	if e.Type != event.TypeMetric {
		return
	}

	ph.mu.Lock()
	defer ph.mu.Unlock()

	for key, val := range e.Meta {
		fval, ok := toFloat64(val)
		if !ok {
			continue
		}

		history := ph.history[key]
		if len(history) >= ph.window {
			history = history[1:]
		}
		ph.history[key] = append(history, fval)
	}
}

// Predict analyzes recent trends and returns predictions.
func (ph *PredictiveHealer) Predict() []Prediction {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	var predictions []Prediction

	for metric, values := range ph.history {
		if len(values) < 5 {
			continue
		}

		direction, slope := detectTrend(values)
		risk := ph.assessRisk(metric, values, slope)

		if risk > 0.3 {
			predictions = append(predictions, Prediction{
				Metric:    metric,
				Direction: direction,
				Risk:      risk,
				Message:   formatPrediction(metric, direction, risk),
			})
		}
	}

	return predictions
}

func detectTrend(values []float64) (TrendDirection, float64) {
	n := len(values)
	if n < 3 {
		return TrendStable, 0
	}

	// Simple linear regression slope
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, v := range values {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}

	nf := float64(n)
	slope := (nf*sumXY - sumX*sumY) / (nf*sumX2 - sumX*sumX)

	// Normalize slope relative to mean
	mean := sumY / nf
	if mean == 0 {
		mean = 1
	}
	normalizedSlope := slope / mean

	if normalizedSlope > 0.02 {
		return TrendUp, normalizedSlope
	} else if normalizedSlope < -0.02 {
		return TrendDown, normalizedSlope
	}
	return TrendStable, normalizedSlope
}

func (ph *PredictiveHealer) assessRisk(metric string, values []float64, slope float64) float64 {
	if len(values) == 0 {
		return 0
	}

	latest := values[len(values)-1]
	isAnomaly := ph.dna.IsAnomaly(metric, latest)

	risk := 0.0

	// Rising trend increases risk
	if slope > 0 {
		risk += slope * 5 // Scale slope to risk contribution
		if risk > 0.5 {
			risk = 0.5
		}
	}

	// Already anomalous adds significant risk
	if isAnomaly {
		risk += 0.5
	}

	if risk > 1.0 {
		risk = 1.0
	}
	return risk
}

func formatPrediction(metric string, direction TrendDirection, risk float64) string {
	severity := "low"
	if risk > 0.7 {
		severity = "high"
	} else if risk > 0.4 {
		severity = "medium"
	}

	switch direction {
	case TrendUp:
		return metric + " is trending upward — " + severity + " risk of threshold breach"
	case TrendDown:
		return metric + " is trending downward — " + severity + " risk of degradation"
	default:
		return metric + " — " + severity + " risk detected"
	}
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test -v ./internal/brain/...
```

Expected: All tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/brain/
git commit -m "feat: add predictive healer with trend analysis and risk scoring"
```

---

## Task 3: Causality Graph — Root Cause Tracking

**Files:**
- Create: `internal/causality/graph.go`
- Create: `internal/causality/graph_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/causality/graph_test.go`:

```go
package causality_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/causality"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestGraphAddAndTraceChain(t *testing.T) {
	g := causality.New()

	// Simulate: bad deploy → slow DB → API timeout
	e1 := event.New(event.TypeError, event.SeverityWarning, "deployment started").
		WithSource("deploy-service")
	time.Sleep(10 * time.Millisecond)

	e2 := event.New(event.TypeError, event.SeverityError, "database query slow").
		WithSource("postgres")
	time.Sleep(10 * time.Millisecond)

	e3 := event.New(event.TypeError, event.SeverityCritical, "API timeout").
		WithSource("api-server")

	g.Add(e1)
	g.Add(e2)
	g.Add(e3)

	// Link them: e1 caused e2, e2 caused e3
	g.Link(e1.ID, e2.ID)
	g.Link(e2.ID, e3.ID)

	// Trace root cause from e3
	chain := g.RootCause(e3.ID)
	if len(chain) < 3 {
		t.Fatalf("expected chain of 3, got %d", len(chain))
	}
	if chain[0].ID != e1.ID {
		t.Errorf("expected root cause to be e1, got %s", chain[0].Source)
	}
}

func TestGraphAutoCorrelate(t *testing.T) {
	g := causality.NewWithWindow(2 * time.Second)

	// Events close in time from related sources
	e1 := event.New(event.TypeError, event.SeverityError, "disk full").
		WithSource("storage")
	e2 := event.New(event.TypeError, event.SeverityCritical, "write failed").
		WithSource("database")

	g.Add(e1)
	g.Add(e2)

	// Auto-correlate by time proximity
	g.AutoCorrelate()

	chain := g.RootCause(e2.ID)
	if len(chain) < 2 {
		t.Errorf("expected auto-correlated chain of at least 2, got %d", len(chain))
	}
}

func TestGraphImpactAnalysis(t *testing.T) {
	g := causality.New()

	root := event.New(event.TypeError, event.SeverityError, "root failure").WithSource("core")
	child1 := event.New(event.TypeError, event.SeverityError, "child 1").WithSource("svc-a")
	child2 := event.New(event.TypeError, event.SeverityError, "child 2").WithSource("svc-b")

	g.Add(root)
	g.Add(child1)
	g.Add(child2)
	g.Link(root.ID, child1.ID)
	g.Link(root.ID, child2.ID)

	impact := g.Impact(root.ID)
	if len(impact) != 2 {
		t.Errorf("expected 2 impacted events, got %d", len(impact))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Implement causality graph**

Create `internal/causality/graph.go`:

```go
package causality

import (
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

// Graph tracks causal relationships between events.
type Graph struct {
	mu       sync.RWMutex
	events   map[string]*event.Event
	parents  map[string]string   // child → parent
	children map[string][]string // parent → children
	order    []string            // insertion order
	window   time.Duration       // time window for auto-correlation
}

// New creates a new causality graph with default 5-second correlation window.
func New() *Graph {
	return NewWithWindow(5 * time.Second)
}

// NewWithWindow creates a graph with a custom time window.
func NewWithWindow(window time.Duration) *Graph {
	return &Graph{
		events:   make(map[string]*event.Event),
		parents:  make(map[string]string),
		children: make(map[string][]string),
		window:   window,
	}
}

// Add records an event in the graph.
func (g *Graph) Add(e *event.Event) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.events[e.ID] = e
	g.order = append(g.order, e.ID)
}

// Link establishes a causal relationship: cause → effect.
func (g *Graph) Link(causeID, effectID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.parents[effectID] = causeID
	g.children[causeID] = append(g.children[causeID], effectID)
}

// RootCause traces the causal chain from an event back to its root cause.
// Returns events from root to the given event.
func (g *Graph) RootCause(eventID string) []*event.Event {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var chain []*event.Event
	current := eventID
	visited := make(map[string]bool)

	for {
		if visited[current] {
			break // Prevent cycles
		}
		visited[current] = true

		e, ok := g.events[current]
		if !ok {
			break
		}
		chain = append([]*event.Event{e}, chain...)

		parent, ok := g.parents[current]
		if !ok {
			break
		}
		current = parent
	}

	return chain
}

// Impact returns all events caused (directly or indirectly) by the given event.
func (g *Graph) Impact(eventID string) []*event.Event {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*event.Event
	visited := make(map[string]bool)

	var walk func(id string)
	walk = func(id string) {
		for _, childID := range g.children[id] {
			if visited[childID] {
				continue
			}
			visited[childID] = true
			if e, ok := g.events[childID]; ok {
				result = append(result, e)
			}
			walk(childID)
		}
	}

	walk(eventID)
	return result
}

// AutoCorrelate links events that are close in time and likely related.
// Events within the time window are linked by temporal proximity.
func (g *Graph) AutoCorrelate() {
	g.mu.Lock()
	defer g.mu.Unlock()

	for i := 0; i < len(g.order)-1; i++ {
		e1, ok1 := g.events[g.order[i]]
		e2, ok2 := g.events[g.order[i+1]]
		if !ok1 || !ok2 {
			continue
		}

		timeDiff := e2.Timestamp.Sub(e1.Timestamp)
		if timeDiff >= 0 && timeDiff <= g.window {
			// Link if not already linked
			if _, exists := g.parents[e2.ID]; !exists {
				g.parents[e2.ID] = e1.ID
				g.children[e1.ID] = append(g.children[e1.ID], e2.ID)
			}
		}
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test -v ./internal/causality/...
```

Expected: All 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/causality/
git commit -m "feat: add causality graph for root cause analysis and impact tracking"
```

---

## Task 4: Time-Travel Debugger — State Snapshots

**Files:**
- Create: `internal/timetravel/recorder.go`
- Create: `internal/timetravel/recorder_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/timetravel/recorder_test.go`:

```go
package timetravel_test

import (
	"testing"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/timetravel"
)

func TestRecorderCaptureAndReplay(t *testing.T) {
	r := timetravel.New(100)

	// Record events over time
	e1 := event.New(event.TypeMetric, event.SeverityInfo, "cpu: 45%")
	time.Sleep(10 * time.Millisecond)
	e2 := event.New(event.TypeMetric, event.SeverityInfo, "cpu: 50%")
	time.Sleep(10 * time.Millisecond)
	e3 := event.New(event.TypeError, event.SeverityCritical, "crash!")

	r.Record(e1)
	r.Record(e2)
	r.Record(e3)

	// Replay all
	events := r.Replay(time.Time{}, time.Now().Add(time.Second))
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestRecorderReplayTimeRange(t *testing.T) {
	r := timetravel.New(100)

	before := time.Now()
	time.Sleep(20 * time.Millisecond)

	e1 := event.New(event.TypeMetric, event.SeverityInfo, "in range")
	r.Record(e1)

	time.Sleep(20 * time.Millisecond)
	after := time.Now()
	time.Sleep(20 * time.Millisecond)

	e2 := event.New(event.TypeMetric, event.SeverityInfo, "out of range")
	r.Record(e2)

	events := r.Replay(before, after)
	if len(events) != 1 {
		t.Errorf("expected 1 event in range, got %d", len(events))
	}
}

func TestRecorderRewindToBeforeFailure(t *testing.T) {
	r := timetravel.New(100)

	// Normal period
	for i := 0; i < 5; i++ {
		r.Record(event.New(event.TypeMetric, event.SeverityInfo, "normal"))
		time.Sleep(5 * time.Millisecond)
	}

	failTime := time.Now()
	time.Sleep(5 * time.Millisecond)

	// Failure
	r.Record(event.New(event.TypeError, event.SeverityCritical, "CRASH"))

	// Rewind to before failure
	events := r.RewindBefore(failTime, 3)
	if len(events) == 0 {
		t.Error("expected events before failure")
	}
	for _, e := range events {
		if e.Severity == event.SeverityCritical {
			t.Error("should not include the crash event")
		}
	}
}

func TestRecorderSnapshot(t *testing.T) {
	r := timetravel.New(100)

	r.TakeSnapshot("deploy-v1", map[string]interface{}{
		"version": "1.0.0",
		"cpu":     45.0,
		"memory":  60.0,
	})

	r.TakeSnapshot("deploy-v2", map[string]interface{}{
		"version": "2.0.0",
		"cpu":     95.0,
		"memory":  88.0,
	})

	snapshots := r.Snapshots()
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}

	diff := r.DiffSnapshots("deploy-v1", "deploy-v2")
	if len(diff) == 0 {
		t.Error("expected differences between snapshots")
	}
}
```

- [ ] **Step 2: Implement time-travel recorder**

Create `internal/timetravel/recorder.go`:

```go
package timetravel

import (
	"fmt"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

// Snapshot captures system state at a point in time.
type Snapshot struct {
	Name      string                 `json:"name"`
	Timestamp time.Time              `json:"timestamp"`
	State     map[string]interface{} `json:"state"`
}

// Diff represents a difference between two snapshots.
type Diff struct {
	Key      string      `json:"key"`
	Before   interface{} `json:"before"`
	After    interface{} `json:"after"`
}

// Recorder captures events and snapshots for time-travel debugging.
type Recorder struct {
	mu        sync.RWMutex
	events    []*event.Event
	snapshots map[string]*Snapshot
	maxEvents int
}

// New creates a new time-travel recorder with a max event buffer.
func New(maxEvents int) *Recorder {
	return &Recorder{
		events:    make([]*event.Event, 0, maxEvents),
		snapshots: make(map[string]*Snapshot),
		maxEvents: maxEvents,
	}
}

// Record adds an event to the timeline.
func (r *Recorder) Record(e *event.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.events) >= r.maxEvents {
		r.events = r.events[1:]
	}
	r.events = append(r.events, e)
}

// Replay returns all events within a time range, ordered chronologically.
func (r *Recorder) Replay(from, to time.Time) []*event.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*event.Event
	for _, e := range r.events {
		if (from.IsZero() || !e.Timestamp.Before(from)) &&
			(to.IsZero() || !e.Timestamp.After(to)) {
			result = append(result, e)
		}
	}
	return result
}

// RewindBefore returns the last N events before a given timestamp.
func (r *Recorder) RewindBefore(before time.Time, count int) []*event.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var candidates []*event.Event
	for _, e := range r.events {
		if e.Timestamp.Before(before) {
			candidates = append(candidates, e)
		}
	}

	if len(candidates) <= count {
		return candidates
	}
	return candidates[len(candidates)-count:]
}

// TakeSnapshot captures the current state under a name.
func (r *Recorder) TakeSnapshot(name string, state map[string]interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.snapshots[name] = &Snapshot{
		Name:      name,
		Timestamp: time.Now(),
		State:     state,
	}
}

// Snapshots returns all snapshots.
func (r *Recorder) Snapshots() []*Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Snapshot
	for _, s := range r.snapshots {
		result = append(result, s)
	}
	return result
}

// DiffSnapshots compares two named snapshots and returns differences.
func (r *Recorder) DiffSnapshots(name1, name2 string) []Diff {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s1, ok1 := r.snapshots[name1]
	s2, ok2 := r.snapshots[name2]
	if !ok1 || !ok2 {
		return nil
	}

	var diffs []Diff

	// Check all keys in s1
	for k, v1 := range s1.State {
		v2, exists := s2.State[k]
		if !exists {
			diffs = append(diffs, Diff{Key: k, Before: v1, After: nil})
		} else if fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			diffs = append(diffs, Diff{Key: k, Before: v1, After: v2})
		}
	}

	// Check keys only in s2
	for k, v2 := range s2.State {
		if _, exists := s1.State[k]; !exists {
			diffs = append(diffs, Diff{Key: k, Before: nil, After: v2})
		}
	}

	return diffs
}

// EventCount returns the number of recorded events.
func (r *Recorder) EventCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.events)
}
```

- [ ] **Step 3: Run tests**

```bash
go test -v ./internal/timetravel/...
```

Expected: All 4 tests PASS

---

## Task 5: Simulation Sandbox — Test Fixes Before Applying

**Files:**
- Create: `internal/sandbox/sandbox.go`
- Create: `internal/sandbox/sandbox_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/sandbox/sandbox_test.go`:

```go
package sandbox_test

import (
	"errors"
	"testing"

	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/sandbox"
)

func TestSandboxRunSuccess(t *testing.T) {
	sb := sandbox.New()

	result := sb.Test(
		event.New(event.TypeError, event.SeverityCritical, "crash"),
		func(e *event.Event) error {
			// Simulated fix succeeds
			return nil
		},
	)

	if !result.Safe {
		t.Error("expected safe result for successful fix")
	}
	if result.Error != "" {
		t.Errorf("expected no error, got: %s", result.Error)
	}
}

func TestSandboxRunFailure(t *testing.T) {
	sb := sandbox.New()

	result := sb.Test(
		event.New(event.TypeError, event.SeverityCritical, "crash"),
		func(e *event.Event) error {
			return errors.New("fix made things worse")
		},
	)

	if result.Safe {
		t.Error("expected unsafe result for failing fix")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestSandboxRunPanic(t *testing.T) {
	sb := sandbox.New()

	result := sb.Test(
		event.New(event.TypeError, event.SeverityCritical, "crash"),
		func(e *event.Event) error {
			panic("something terrible")
		},
	)

	if result.Safe {
		t.Error("expected unsafe result for panicking fix")
	}
}

func TestSandboxHistory(t *testing.T) {
	sb := sandbox.New()

	sb.Test(event.New(event.TypeError, event.SeverityCritical, "fix1"),
		func(e *event.Event) error { return nil })
	sb.Test(event.New(event.TypeError, event.SeverityCritical, "fix2"),
		func(e *event.Event) error { return errors.New("fail") })

	history := sb.History()
	if len(history) != 2 {
		t.Fatalf("expected 2 results, got %d", len(history))
	}
	if !history[0].Safe {
		t.Error("first test should be safe")
	}
	if history[1].Safe {
		t.Error("second test should be unsafe")
	}
}
```

- [ ] **Step 2: Implement sandbox**

Create `internal/sandbox/sandbox.go`:

```go
package sandbox

import (
	"fmt"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

// Result captures the outcome of a sandbox test.
type Result struct {
	EventID   string        `json:"event_id"`
	Safe      bool          `json:"safe"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// ActionFunc is a healing action to test.
type ActionFunc func(e *event.Event) error

// Sandbox tests healing actions in isolation before applying to production.
type Sandbox struct {
	mu      sync.Mutex
	history []Result
}

// New creates a new simulation sandbox.
func New() *Sandbox {
	return &Sandbox{}
}

// Test runs a healing action in the sandbox and reports whether it's safe.
// Catches panics and errors.
func (s *Sandbox) Test(e *event.Event, action ActionFunc) Result {
	start := time.Now()

	result := Result{
		EventID:   e.ID,
		Timestamp: start,
	}

	// Run in protected context
	func() {
		defer func() {
			if r := recover(); r != nil {
				result.Safe = false
				result.Error = fmt.Sprintf("panic: %v", r)
			}
		}()

		err := action(e)
		if err != nil {
			result.Safe = false
			result.Error = err.Error()
		} else {
			result.Safe = true
		}
	}()

	result.Duration = time.Since(start)

	s.mu.Lock()
	s.history = append(s.history, result)
	s.mu.Unlock()

	return result
}

// History returns all sandbox test results.
func (s *Sandbox) History() []Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Result, len(s.history))
	copy(out, s.history)
	return out
}

// SuccessRate returns the percentage of sandbox tests that were safe.
func (s *Sandbox) SuccessRate() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.history) == 0 {
		return 1.0
	}

	safe := 0
	for _, r := range s.history {
		if r.Safe {
			safe++
		}
	}
	return float64(safe) / float64(len(s.history))
}
```

- [ ] **Step 3: Run tests**

```bash
go test -v ./internal/sandbox/...
```

Expected: All 4 tests PASS

---

## Task 6: Consensus Healing — Multi-Verification

**Files:**
- Create: `internal/consensus/consensus.go`
- Create: `internal/consensus/consensus_test.go`

- [ ] **Step 1: Write failing test**

Create `internal/consensus/consensus_test.go`:

```go
package consensus_test

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/consensus"
	"github.com/immortal-engine/immortal/internal/event"
)

func TestConsensusAllAgree(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 3})

	c.AddVerifier("rule-match", func(e *event.Event) bool { return true })
	c.AddVerifier("anomaly-detect", func(e *event.Event) bool { return true })
	c.AddVerifier("trend-analysis", func(e *event.Event) bool { return true })

	e := event.New(event.TypeError, event.SeverityCritical, "crash")
	result := c.Evaluate(e)

	if !result.Approved {
		t.Error("expected approved when all verifiers agree")
	}
	if result.Votes != 3 {
		t.Errorf("expected 3 votes, got %d", result.Votes)
	}
}

func TestConsensusDisagree(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 3})

	c.AddVerifier("rule-match", func(e *event.Event) bool { return true })
	c.AddVerifier("anomaly-detect", func(e *event.Event) bool { return false })
	c.AddVerifier("trend-analysis", func(e *event.Event) bool { return true })

	e := event.New(event.TypeError, event.SeverityCritical, "crash")
	result := c.Evaluate(e)

	if result.Approved {
		t.Error("expected NOT approved when only 2/3 agree and min is 3")
	}
	if result.Votes != 2 {
		t.Errorf("expected 2 votes, got %d", result.Votes)
	}
}

func TestConsensusMajority(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 2})

	c.AddVerifier("v1", func(e *event.Event) bool { return true })
	c.AddVerifier("v2", func(e *event.Event) bool { return true })
	c.AddVerifier("v3", func(e *event.Event) bool { return false })

	e := event.New(event.TypeError, event.SeverityError, "error")
	result := c.Evaluate(e)

	if !result.Approved {
		t.Error("expected approved with 2/3 when min is 2")
	}
}

func TestConsensusNoVerifiers(t *testing.T) {
	c := consensus.New(consensus.Config{MinAgreement: 1})

	e := event.New(event.TypeError, event.SeverityError, "error")
	result := c.Evaluate(e)

	if result.Approved {
		t.Error("expected NOT approved with no verifiers")
	}
}
```

- [ ] **Step 2: Implement consensus engine**

Create `internal/consensus/consensus.go`:

```go
package consensus

import (
	"sync"

	"github.com/immortal-engine/immortal/internal/event"
)

// VerifyFunc determines if a verifier agrees that action should be taken.
type VerifyFunc func(e *event.Event) bool

// Config controls consensus behavior.
type Config struct {
	MinAgreement int // Minimum number of verifiers that must agree
}

// Result captures the outcome of a consensus evaluation.
type Result struct {
	Approved  bool     `json:"approved"`
	Votes     int      `json:"votes"`
	Total     int      `json:"total"`
	Voters    []string `json:"voters"`    // Names of verifiers that agreed
	Dissenters []string `json:"dissenters"` // Names that disagreed
}

// Engine runs multiple verification strategies and requires consensus.
type Engine struct {
	mu        sync.RWMutex
	config    Config
	verifiers map[string]VerifyFunc
	order     []string
}

// New creates a new consensus engine.
func New(config Config) *Engine {
	if config.MinAgreement < 1 {
		config.MinAgreement = 1
	}
	return &Engine{
		config:    config,
		verifiers: make(map[string]VerifyFunc),
	}
}

// AddVerifier registers a named verification function.
func (e *Engine) AddVerifier(name string, fn VerifyFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.verifiers[name] = fn
	e.order = append(e.order, name)
}

// Evaluate runs all verifiers and determines if consensus is reached.
func (e *Engine) Evaluate(ev *event.Event) Result {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := Result{
		Total: len(e.verifiers),
	}

	for _, name := range e.order {
		fn := e.verifiers[name]
		if fn(ev) {
			result.Votes++
			result.Voters = append(result.Voters, name)
		} else {
			result.Dissenters = append(result.Dissenters, name)
		}
	}

	result.Approved = result.Votes >= e.config.MinAgreement
	return result
}

// VerifierCount returns the number of registered verifiers.
func (e *Engine) VerifierCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.verifiers)
}
```

- [ ] **Step 3: Run tests**

```bash
go test -v ./internal/consensus/...
```

Expected: All 4 tests PASS

---

## Self-Review

**Spec coverage:**
- Predictive healing with built-in ML ✅ (Task 2 — trend analysis, risk scoring)
- Healing DNA (health fingerprinting) ✅ (Task 1 — rolling stats, anomaly detection, health score)
- Causality Graph ✅ (Task 3 — root cause tracing, impact analysis, auto-correlation)
- Time-Travel Debugger ✅ (Task 4 — event recording, replay, snapshots, diff)
- Simulation Sandbox ✅ (Task 5 — safe action testing, panic recovery, history)
- Consensus Healing ✅ (Task 6 — multi-verifier agreement, majority voting)

**Placeholder scan:** No TBDs, TODOs, or vague instructions. All code complete.

**Type consistency:** All packages use internal/event.Event consistently. DNA integrates with PredictiveHealer. All new packages are independent and testable.
