package otel

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/immortal-engine/immortal/internal/event"
)

// ---------- test body helpers ----------

func tinyTracesJSON() string {
	return `{
		"resourceSpans":[{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-svc"}}]},
			"scopeSpans":[{
				"spans":[{
					"traceId":"aabbccddeeff0011aabbccddeeff0011",
					"spanId":"aabbccddeeff0011",
					"name":"test-span",
					"startTimeUnixNano":"1000000000",
					"endTimeUnixNano":"2000000000",
					"status":{"code":0},
					"attributes":[]
				}]
			}]
		}]
	}`
}

func tinyMetricsJSON() string {
	return `{
		"resourceMetrics":[{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-svc"}}]},
			"scopeMetrics":[{
				"metrics":[{
					"name":"test.gauge",
					"gauge":{"dataPoints":[{
						"attributes":[],
						"timeUnixNano":"1000000000",
						"asDouble":1.23
					}]}
				}]
			}]
		}]
	}`
}

func tinyLogsJSON() string {
	return `{
		"resourceLogs":[{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-svc"}}]},
			"scopeLogs":[{
				"logRecords":[{
					"timeUnixNano":"1000000000",
					"severityNumber":9,
					"body":{"stringValue":"hello world"},
					"attributes":[]
				}]
			}]
		}]
	}`
}

// ---------- helpers ----------

func newTestReceiver(sink Sink) (*Receiver, http.Handler) {
	r := New(Config{Sink: sink, MaxBodyMB: 1})
	return r, r.Handler()
}

func postJSON(h http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// ---------- endpoint 200 tests ----------

func TestReceiver_TracesEndpoint_200OnValidJSON(t *testing.T) {
	var got []*event.Event
	_, h := newTestReceiver(func(e *event.Event) { got = append(got, e) })

	w := postJSON(h, "/v1/traces", tinyTracesJSON())
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(got) != 1 {
		t.Errorf("want 1 event from sink, got %d", len(got))
	}
}

func TestReceiver_MetricsEndpoint_200(t *testing.T) {
	var got []*event.Event
	_, h := newTestReceiver(func(e *event.Event) { got = append(got, e) })

	w := postJSON(h, "/v1/metrics", tinyMetricsJSON())
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(got) != 1 {
		t.Errorf("want 1 event from sink, got %d", len(got))
	}
}

func TestReceiver_LogsEndpoint_200(t *testing.T) {
	var got []*event.Event
	_, h := newTestReceiver(func(e *event.Event) { got = append(got, e) })

	w := postJSON(h, "/v1/logs", tinyLogsJSON())
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(got) != 1 {
		t.Errorf("want 1 event from sink, got %d", len(got))
	}
}

// ---------- method / size / parse tests ----------

func TestReceiver_GETReturns405(t *testing.T) {
	_, h := newTestReceiver(nil)
	for _, path := range []string{"/v1/traces", "/v1/metrics", "/v1/logs"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s GET: want 405, got %d", path, w.Code)
		}
	}
}

func TestReceiver_BodyTooLarge_413(t *testing.T) {
	r := New(Config{MaxBodyMB: 1})
	h := r.Handler()

	big := bytes.Repeat([]byte("x"), 1*1024*1024+1)
	req := httptest.NewRequest(http.MethodPost, "/v1/traces", bytes.NewReader(big))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("want 413, got %d", w.Code)
	}
}

func TestReceiver_InvalidJSON_400_StatsRecorded(t *testing.T) {
	var sinkCalled bool
	r, h := newTestReceiver(func(*event.Event) { sinkCalled = true })

	req := httptest.NewRequest(http.MethodPost, "/v1/traces", strings.NewReader("not-json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
	if sinkCalled {
		t.Error("sink should not be called on parse error")
	}
	s := r.Stats()
	if s.ParseErrors != 1 {
		t.Errorf("want ParseErrors=1, got %d", s.ParseErrors)
	}
	if s.EventsEmitted != 0 {
		t.Errorf("want EventsEmitted=0, got %d", s.EventsEmitted)
	}
}

func TestReceiver_StatsCounters_Increment(t *testing.T) {
	r := New(Config{MaxBodyMB: 4})
	h := r.Handler()

	postJSON(h, "/v1/traces", tinyTracesJSON())
	postJSON(h, "/v1/metrics", tinyMetricsJSON())
	postJSON(h, "/v1/logs", tinyLogsJSON())

	s := r.Stats()
	if s.RequestsTraces != 1 {
		t.Errorf("RequestsTraces: want 1, got %d", s.RequestsTraces)
	}
	if s.RequestsMetrics != 1 {
		t.Errorf("RequestsMetrics: want 1, got %d", s.RequestsMetrics)
	}
	if s.RequestsLogs != 1 {
		t.Errorf("RequestsLogs: want 1, got %d", s.RequestsLogs)
	}
	if s.EventsEmitted != 3 {
		t.Errorf("EventsEmitted: want 3, got %d", s.EventsEmitted)
	}
	if s.ParseErrors != 0 {
		t.Errorf("ParseErrors: want 0, got %d", s.ParseErrors)
	}
}
