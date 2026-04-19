package otel

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

// ---------- OTLP JSON structures ----------

type tracesPayload struct {
	ResourceSpans []resourceSpans `json:"resourceSpans"`
}

type resourceSpans struct {
	Resource   otlpResource  `json:"resource"`
	ScopeSpans []scopeSpans  `json:"scopeSpans"`
}

type scopeSpans struct {
	Spans []otlpSpan `json:"spans"`
}

type otlpSpan struct {
	TraceID           string          `json:"traceId"`
	SpanID            string          `json:"spanId"`
	Name              string          `json:"name"`
	StartTimeUnixNano string          `json:"startTimeUnixNano"`
	EndTimeUnixNano   string          `json:"endTimeUnixNano"`
	Status            otlpSpanStatus  `json:"status"`
	Attributes        []otlpKeyValue  `json:"attributes"`
}

type otlpSpanStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ---------- metrics ----------

type metricsPayload struct {
	ResourceMetrics []resourceMetrics `json:"resourceMetrics"`
}

type resourceMetrics struct {
	Resource      otlpResource   `json:"resource"`
	ScopeMetrics  []scopeMetrics `json:"scopeMetrics"`
}

type scopeMetrics struct {
	Metrics []otlpMetric `json:"metrics"`
}

type otlpMetric struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Unit        string          `json:"unit"`
	Gauge       *otlpGauge      `json:"gauge,omitempty"`
	Sum         *otlpSum        `json:"sum,omitempty"`
	Histogram   *otlpHistogram  `json:"histogram,omitempty"`
}

type otlpGauge struct {
	DataPoints []otlpNumberDataPoint `json:"dataPoints"`
}

type otlpSum struct {
	DataPoints []otlpNumberDataPoint `json:"dataPoints"`
}

type otlpHistogram struct {
	DataPoints []otlpHistogramDataPoint `json:"dataPoints"`
}

type otlpNumberDataPoint struct {
	Attributes        []otlpKeyValue  `json:"attributes"`
	TimeUnixNano      string          `json:"timeUnixNano"`
	AsDouble          *float64        `json:"asDouble,omitempty"`
	AsInt             *int64          `json:"asInt,omitempty"`
}

type otlpHistogramDataPoint struct {
	Attributes  []otlpKeyValue `json:"attributes"`
	TimeUnixNano string        `json:"timeUnixNano"`
	Count       uint64         `json:"count"`
	Sum         *float64       `json:"sum,omitempty"`
	Min         *float64       `json:"min,omitempty"`
	Max         *float64       `json:"max,omitempty"`
}

// ---------- logs ----------

type logsPayload struct {
	ResourceLogs []resourceLogs `json:"resourceLogs"`
}

type resourceLogs struct {
	Resource   otlpResource `json:"resource"`
	ScopeLogs  []scopeLogs  `json:"scopeLogs"`
}

type scopeLogs struct {
	LogRecords []otlpLogRecord `json:"logRecords"`
}

type otlpLogRecord struct {
	TimeUnixNano         string         `json:"timeUnixNano"`
	ObservedTimeUnixNano string         `json:"observedTimeUnixNano"`
	SeverityNumber       int            `json:"severityNumber"`
	SeverityText         string         `json:"severityText"`
	Body                 otlpAnyValue   `json:"body"`
	Attributes           []otlpKeyValue `json:"attributes"`
}

// ---------- shared ----------

type otlpResource struct {
	Attributes []otlpKeyValue `json:"attributes"`
}

type otlpKeyValue struct {
	Key   string         `json:"key"`
	Value map[string]any `json:"value"`
}

// otlpAnyValue is the top-level body of a log record (stringValue, intValue, etc.)
type otlpAnyValue struct {
	StringValue *string        `json:"stringValue,omitempty"`
	IntValue    *string        `json:"intValue,omitempty"`   // OTLP JSON encodes int64 as string
	DoubleValue *float64       `json:"doubleValue,omitempty"`
	BoolValue   *bool          `json:"boolValue,omitempty"`
	ArrayValue  *otlpArrayValue `json:"arrayValue,omitempty"`
	KvlistValue *otlpKvList    `json:"kvlistValue,omitempty"`
}

type otlpArrayValue struct {
	Values []otlpAnyValue `json:"values"`
}

type otlpKvList struct {
	Values []otlpKeyValue `json:"values"`
}

// ---------- helpers ----------

// extractAttr returns a typed Go value from an OTLP attribute value map.
// The map has one of: stringValue, intValue, doubleValue, boolValue, arrayValue, kvlistValue.
func extractAttr(value map[string]any) any {
	if v, ok := value["stringValue"]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	if v, ok := value["intValue"]; ok {
		// OTLP JSON encodes int64 as string
		switch t := v.(type) {
		case string:
			var i int64
			fmt.Sscanf(t, "%d", &i)
			return i
		case float64:
			return int64(t)
		}
	}
	if v, ok := value["doubleValue"]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	if v, ok := value["boolValue"]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return nil
}

// attrsToMap converts a slice of OTLP KeyValue pairs to a Go map.
func attrsToMap(attrs []otlpKeyValue) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, kv := range attrs {
		m[kv.Key] = extractAttr(kv.Value)
	}
	return m
}

// serviceName extracts the "service.name" attribute from a resource, defaulting to "unknown".
func serviceName(r otlpResource) string {
	for _, kv := range r.Attributes {
		if kv.Key == "service.name" {
			if s, ok := extractAttr(kv.Value).(string); ok && s != "" {
				return s
			}
		}
	}
	return "unknown"
}

// parseNano parses a Unix nanosecond timestamp string to time.Time.
func parseNano(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	var ns int64
	fmt.Sscanf(s, "%d", &ns)
	return time.Unix(0, ns)
}

// ---------- converters ----------

// convertTraces decodes an OTLP/HTTP JSON traces body and calls sink for each span.
func convertTraces(data []byte, sink Sink) error {
	var payload tracesPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	spanAttrsOfInterest := map[string]bool{
		"http.status_code": true,
		"http.method":      true,
		"http.url":         true,
		"db.system":        true,
	}
	for _, rs := range payload.ResourceSpans {
		svc := serviceName(rs.Resource)
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				sev := event.SeverityInfo
				if span.Status.Code == 2 {
					sev = event.SeverityError
				}
				e := event.New(event.TypeTrace, sev, span.Name)
				e.WithSource(svc)

				// timing
				start := parseNano(span.StartTimeUnixNano)
				end := parseNano(span.EndTimeUnixNano)
				if !start.IsZero() {
					e.Timestamp = start
				}
				var durNs int64
				if !start.IsZero() && !end.IsZero() {
					durNs = end.Sub(start).Nanoseconds()
				}

				e.WithMeta("trace_id", span.TraceID)
				e.WithMeta("span_id", span.SpanID)
				e.WithMeta("duration_ns", durNs)

				for _, kv := range span.Attributes {
					if spanAttrsOfInterest[kv.Key] {
						e.WithMeta(kv.Key, extractAttr(kv.Value))
					}
				}

				sink(e)
			}
		}
	}
	return nil
}

// convertMetrics decodes an OTLP/HTTP JSON metrics body and calls sink for each data point.
func convertMetrics(data []byte, sink Sink) error {
	var payload metricsPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	for _, rm := range payload.ResourceMetrics {
		svc := serviceName(rm.Resource)
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				switch {
				case m.Gauge != nil:
					for _, dp := range m.Gauge.DataPoints {
						sink(numberDataPointEvent(svc, m.Name, dp))
					}
				case m.Sum != nil:
					for _, dp := range m.Sum.DataPoints {
						sink(numberDataPointEvent(svc, m.Name, dp))
					}
				case m.Histogram != nil:
					for _, dp := range m.Histogram.DataPoints {
						sink(histogramDataPointEvent(svc, m.Name, dp))
					}
				}
			}
		}
	}
	return nil
}

func numberDataPointEvent(svc, metricName string, dp otlpNumberDataPoint) *event.Event {
	e := event.New(event.TypeMetric, event.SeverityInfo, metricName)
	e.WithSource(svc)
	if !parseNano(dp.TimeUnixNano).IsZero() {
		e.Timestamp = parseNano(dp.TimeUnixNano)
	}
	e.WithMeta("metric.name", metricName)
	if dp.AsDouble != nil {
		e.WithMeta("value", *dp.AsDouble)
	} else if dp.AsInt != nil {
		e.WithMeta("value", *dp.AsInt)
	}
	for k, v := range attrsToMap(dp.Attributes) {
		e.WithMeta(k, v)
	}
	return e
}

func histogramDataPointEvent(svc, metricName string, dp otlpHistogramDataPoint) *event.Event {
	e := event.New(event.TypeMetric, event.SeverityInfo, metricName)
	e.WithSource(svc)
	if !parseNano(dp.TimeUnixNano).IsZero() {
		e.Timestamp = parseNano(dp.TimeUnixNano)
	}
	e.WithMeta("metric.name", metricName)
	e.WithMeta("count", dp.Count)
	if dp.Sum != nil {
		e.WithMeta("sum", *dp.Sum)
	}
	if dp.Min != nil {
		e.WithMeta("min", *dp.Min)
	}
	if dp.Max != nil {
		e.WithMeta("max", *dp.Max)
	}
	for k, v := range attrsToMap(dp.Attributes) {
		e.WithMeta(k, v)
	}
	return e
}

// convertLogs decodes an OTLP/HTTP JSON logs body and calls sink for each log record.
func convertLogs(data []byte, sink Sink) error {
	var payload logsPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	for _, rl := range payload.ResourceLogs {
		svc := serviceName(rl.Resource)
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				sev := otlpSeverityToEvent(lr.SeverityNumber)
				msg := extractLogBody(lr.Body)
				e := event.New(event.TypeLog, sev, msg)
				e.WithSource(svc)
				ts := parseNano(lr.TimeUnixNano)
				if ts.IsZero() {
					ts = parseNano(lr.ObservedTimeUnixNano)
				}
				if !ts.IsZero() {
					e.Timestamp = ts
				}
				for k, v := range attrsToMap(lr.Attributes) {
					e.WithMeta(k, v)
				}
				sink(e)
			}
		}
	}
	return nil
}

// otlpSeverityToEvent maps OTLP severityNumber to event.Severity.
//   1-4  (TRACE/DEBUG) → SeverityInfo
//   5-8  (INFO)        → SeverityInfo
//   9-12 (WARN)        → SeverityWarning
//  13-16 (ERROR)       → SeverityError
//  17-20 (FATAL)       → SeverityCritical
func otlpSeverityToEvent(n int) event.Severity {
	switch {
	case n >= 1 && n <= 8:
		return event.SeverityInfo
	case n >= 9 && n <= 12:
		return event.SeverityWarning
	case n >= 13 && n <= 16:
		return event.SeverityError
	case n >= 17 && n <= 20:
		return event.SeverityCritical
	default:
		return event.SeverityInfo
	}
}

// extractLogBody returns a string representation of an OTLP AnyValue log body.
func extractLogBody(body otlpAnyValue) string {
	if body.StringValue != nil {
		return *body.StringValue
	}
	if body.ArrayValue != nil {
		parts := make([]string, 0, len(body.ArrayValue.Values))
		for _, v := range body.ArrayValue.Values {
			parts = append(parts, extractLogBody(v))
		}
		return strings.Join(parts, " ")
	}
	if body.KvlistValue != nil {
		b, err := json.Marshal(body.KvlistValue.Values)
		if err == nil {
			return string(b)
		}
	}
	if body.IntValue != nil {
		return *body.IntValue
	}
	if body.DoubleValue != nil {
		return fmt.Sprintf("%g", *body.DoubleValue)
	}
	if body.BoolValue != nil {
		if *body.BoolValue {
			return "true"
		}
		return "false"
	}
	return ""
}
