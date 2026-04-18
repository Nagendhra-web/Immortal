package otel

import (
	"testing"

	"github.com/immortal-engine/immortal/internal/event"
)

// ---------- helpers ----------

func collectSink() (Sink, func() []*event.Event) {
	var events []*event.Event
	sink := func(e *event.Event) { events = append(events, e) }
	return sink, func() []*event.Event { return events }
}

// ---------- traces ----------

func TestConvertTraces_OneSpanPerEvent(t *testing.T) {
	sink, collected := collectSink()
	err := convertTraces([]byte(tinyTracesJSON()), sink)
	if err != nil {
		t.Fatalf("convertTraces: %v", err)
	}
	got := collected()
	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].Type != event.TypeTrace {
		t.Errorf("expected TypeTrace, got %q", got[0].Type)
	}
}

func TestConvertTraces_StatusErrorMapsToSeverityError(t *testing.T) {
	body := `{
		"resourceSpans":[{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc"}}]},
			"scopeSpans":[{"spans":[{
				"traceId":"abc","spanId":"def","name":"op",
				"startTimeUnixNano":"1000","endTimeUnixNano":"2000",
				"status":{"code":2,"message":"ERROR"},
				"attributes":[]
			}]}]
		}]
	}`
	sink, collected := collectSink()
	if err := convertTraces([]byte(body), sink); err != nil {
		t.Fatal(err)
	}
	ev := collected()
	if len(ev) != 1 {
		t.Fatalf("want 1 event, got %d", len(ev))
	}
	if ev[0].Severity != event.SeverityError {
		t.Errorf("expected SeverityError, got %q", ev[0].Severity)
	}
}

func TestConvertTraces_HTTPAttributesPropagatedToMeta(t *testing.T) {
	body := `{
		"resourceSpans":[{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"web"}}]},
			"scopeSpans":[{"spans":[{
				"traceId":"t1","spanId":"s1","name":"GET /api",
				"startTimeUnixNano":"0","endTimeUnixNano":"0",
				"status":{"code":0},
				"attributes":[
					{"key":"http.method","value":{"stringValue":"GET"}},
					{"key":"http.status_code","value":{"intValue":"200"}},
					{"key":"http.url","value":{"stringValue":"https://example.com/api"}},
					{"key":"db.system","value":{"stringValue":"postgresql"}}
				]
			}]}]
		}]
	}`
	sink, collected := collectSink()
	if err := convertTraces([]byte(body), sink); err != nil {
		t.Fatal(err)
	}
	ev := collected()
	if len(ev) != 1 {
		t.Fatalf("want 1 event, got %d", len(ev))
	}
	meta := ev[0].Meta
	if meta["http.method"] != "GET" {
		t.Errorf("http.method: got %v", meta["http.method"])
	}
	if meta["http.url"] != "https://example.com/api" {
		t.Errorf("http.url: got %v", meta["http.url"])
	}
	if meta["db.system"] != "postgresql" {
		t.Errorf("db.system: got %v", meta["db.system"])
	}
}

// ---------- metrics ----------

func TestConvertMetrics_GaugeDataPoint_EmitsEvent(t *testing.T) {
	body := `{
		"resourceMetrics":[{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc"}}]},
			"scopeMetrics":[{"metrics":[{
				"name":"cpu.usage",
				"gauge":{"dataPoints":[{
					"attributes":[],
					"timeUnixNano":"1000000000",
					"asDouble":0.42
				}]}
			}]}]
		}]
	}`
	sink, collected := collectSink()
	if err := convertMetrics([]byte(body), sink); err != nil {
		t.Fatal(err)
	}
	ev := collected()
	if len(ev) != 1 {
		t.Fatalf("want 1 event, got %d", len(ev))
	}
	if ev[0].Type != event.TypeMetric {
		t.Errorf("want TypeMetric, got %q", ev[0].Type)
	}
	if ev[0].Message != "cpu.usage" {
		t.Errorf("want message 'cpu.usage', got %q", ev[0].Message)
	}
	if ev[0].Meta["value"] != 0.42 {
		t.Errorf("want value 0.42, got %v", ev[0].Meta["value"])
	}
}

func TestConvertMetrics_SumDataPoint_EmitsEvent(t *testing.T) {
	body := `{
		"resourceMetrics":[{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc"}}]},
			"scopeMetrics":[{"metrics":[{
				"name":"requests.total",
				"sum":{"dataPoints":[{
					"attributes":[{"key":"env","value":{"stringValue":"prod"}}],
					"timeUnixNano":"2000000000",
					"asDouble":1024.0
				}]}
			}]}]
		}]
	}`
	sink, collected := collectSink()
	if err := convertMetrics([]byte(body), sink); err != nil {
		t.Fatal(err)
	}
	ev := collected()
	if len(ev) != 1 {
		t.Fatalf("want 1 event, got %d", len(ev))
	}
	if ev[0].Meta["env"] != "prod" {
		t.Errorf("want env=prod, got %v", ev[0].Meta["env"])
	}
}

// ---------- logs ----------

func TestConvertLogs_SeverityNumberMapping_AllRanges(t *testing.T) {
	cases := []struct {
		num  int
		want event.Severity
	}{
		{1, event.SeverityInfo},    // TRACE
		{4, event.SeverityInfo},    // DEBUG
		{5, event.SeverityInfo},    // INFO
		{8, event.SeverityInfo},    // INFO4
		{9, event.SeverityWarning}, // WARN
		{12, event.SeverityWarning},
		{13, event.SeverityError},  // ERROR
		{16, event.SeverityError},
		{17, event.SeverityCritical}, // FATAL
		{20, event.SeverityCritical},
	}

	for _, tc := range cases {
		got := otlpSeverityToEvent(tc.num)
		if got != tc.want {
			t.Errorf("severityNumber=%d: want %q, got %q", tc.num, tc.want, got)
		}
	}
}

func TestConvertLogs_BodyStringValueExtracted(t *testing.T) {
	sink, collected := collectSink()
	if err := convertLogs([]byte(tinyLogsJSON()), sink); err != nil {
		t.Fatal(err)
	}
	ev := collected()
	if len(ev) != 1 {
		t.Fatalf("want 1 event, got %d", len(ev))
	}
	if ev[0].Message != "hello world" {
		t.Errorf("want 'hello world', got %q", ev[0].Message)
	}
}

// ---------- extractAttr ----------

func TestExtractAttr_String_Int_Double_Bool(t *testing.T) {
	// string
	if got := extractAttr(map[string]any{"stringValue": "foo"}); got != "foo" {
		t.Errorf("string: got %v", got)
	}
	// int (string-encoded as per OTLP JSON)
	if got := extractAttr(map[string]any{"intValue": "42"}); got != int64(42) {
		t.Errorf("int: got %v (%T)", got, got)
	}
	// int as float64 (JSON number)
	if got := extractAttr(map[string]any{"intValue": float64(7)}); got != int64(7) {
		t.Errorf("int as float64: got %v (%T)", got, got)
	}
	// double
	if got := extractAttr(map[string]any{"doubleValue": 3.14}); got != 3.14 {
		t.Errorf("double: got %v", got)
	}
	// bool
	if got := extractAttr(map[string]any{"boolValue": true}); got != true {
		t.Errorf("bool: got %v", got)
	}
}
