// Package otel provides an OTLP/HTTP JSON receiver that converts incoming
// OpenTelemetry signals into Immortal events and forwards them via a Sink.
//
// Supported endpoints (POST only, JSON body):
//
//	POST /v1/traces
//	POST /v1/metrics
//	POST /v1/logs
//
// Successful requests return HTTP 200 with body "{}".
// The receiver does NOT support Protobuf encoding or exponential histograms.
package otel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/immortal-engine/immortal/internal/event"
)

const defaultMaxBodyMB = 4

// Sink is the function the OTel receiver calls for every converted event.
// Typically wired to engine.Ingest.
type Sink func(*event.Event)

// TopologySink receives parent-service → child-service relationships
// extracted from incoming OTLP spans. Used for automatic topology discovery.
type TopologySink interface {
	AddEdge(from, to string)
}

// Config holds Receiver configuration.
type Config struct {
	Sink         Sink
	MaxBodyMB    int          // default 4
	TopologySink TopologySink // optional; if non-nil, invoked for every span pair
}

// Stats holds cumulative counters for a Receiver.
type Stats struct {
	RequestsTraces  uint64
	RequestsMetrics uint64
	RequestsLogs    uint64
	EventsEmitted   uint64
	ParseErrors     uint64
}

// Receiver implements the OTLP/HTTP JSON endpoints.
type Receiver struct {
	sink         Sink
	topologySink TopologySink
	maxBytes     int64
	reqTraces    atomic.Uint64
	reqMetrics   atomic.Uint64
	reqLogs      atomic.Uint64
	evEmitted    atomic.Uint64
	parseErrs    atomic.Uint64
}

// New creates a Receiver. Use Handler() to mount it on an existing mux,
// or call Listen for a standalone server.
func New(cfg Config) *Receiver {
	maxMB := cfg.MaxBodyMB
	if maxMB <= 0 {
		maxMB = defaultMaxBodyMB
	}
	sink := cfg.Sink
	if sink == nil {
		sink = func(*event.Event) {}
	}
	return &Receiver{
		sink:         sink,
		topologySink: cfg.TopologySink,
		maxBytes:     int64(maxMB) * 1024 * 1024,
	}
}

// Stats returns a snapshot of the receiver's counters.
func (r *Receiver) Stats() Stats {
	return Stats{
		RequestsTraces:  r.reqTraces.Load(),
		RequestsMetrics: r.reqMetrics.Load(),
		RequestsLogs:    r.reqLogs.Load(),
		EventsEmitted:   r.evEmitted.Load(),
		ParseErrors:     r.parseErrs.Load(),
	}
}

// Handler returns an http.Handler implementing /v1/traces, /v1/metrics, /v1/logs.
func (r *Receiver) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", r.handleTraces)
	mux.HandleFunc("/v1/metrics", r.handleMetrics)
	mux.HandleFunc("/v1/logs", r.handleLogs)
	return mux
}

// Listen starts a standalone HTTP server on addr (e.g. ":4318") and blocks
// until ctx is cancelled.
func (r *Receiver) Listen(ctx context.Context, addr string) error {
	srv := &http.Server{
		Addr:    addr,
		Handler: r.Handler(),
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		return srv.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

// ---------- endpoint handlers ----------

func (r *Receiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	r.reqTraces.Add(1)
	data, ok := r.readBody(w, req)
	if !ok {
		return
	}
	countingSink := r.countingSink()
	if err := convertTraces(data, countingSink); err != nil {
		r.parseErrs.Add(1)
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.topologySink != nil {
		emitTopology(data, r.topologySink)
	}
	writeOK(w)
}

func (r *Receiver) handleMetrics(w http.ResponseWriter, req *http.Request) {
	r.reqMetrics.Add(1)
	data, ok := r.readBody(w, req)
	if !ok {
		return
	}
	countingSink := r.countingSink()
	if err := convertMetrics(data, countingSink); err != nil {
		r.parseErrs.Add(1)
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeOK(w)
}

func (r *Receiver) handleLogs(w http.ResponseWriter, req *http.Request) {
	r.reqLogs.Add(1)
	data, ok := r.readBody(w, req)
	if !ok {
		return
	}
	countingSink := r.countingSink()
	if err := convertLogs(data, countingSink); err != nil {
		r.parseErrs.Add(1)
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeOK(w)
}

// readBody reads the request body, enforcing method and size limits.
// Returns (body, true) on success; writes the error response and returns (nil, false) on failure.
func (r *Receiver) readBody(w http.ResponseWriter, req *http.Request) ([]byte, bool) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return nil, false
	}
	limited := io.LimitReader(req.Body, r.maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return nil, false
	}
	if int64(len(data)) > r.maxBytes {
		http.Error(w, fmt.Sprintf("request body exceeds %d MB limit", r.maxBytes/1024/1024), http.StatusRequestEntityTooLarge)
		return nil, false
	}
	return data, true
}

// countingSink wraps the configured sink and increments the evEmitted counter.
func (r *Receiver) countingSink() Sink {
	return func(e *event.Event) {
		r.evEmitted.Add(1)
		r.sink(e)
	}
}

func writeOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("{}")) //nolint:errcheck
}
