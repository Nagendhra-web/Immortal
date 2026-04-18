package otel

import (
	"encoding/json"
	"log"
)

// TopologySinkFunc adapts a plain function into a TopologySink.
type TopologySinkFunc func(from, to string)

func (f TopologySinkFunc) AddEdge(from, to string) { f(from, to) }

// topoSpan contains only the fields needed for topology discovery.
type topoSpan struct {
	SpanID       string `json:"spanId"`
	ParentSpanID string `json:"parentSpanId"`
}

// topoScopeSpans holds spans from a single instrumentation scope.
type topoScopeSpans struct {
	Spans []topoSpan `json:"spans"`
}

// topoResourceSpans pairs a resource (for service.name) with its scope spans.
type topoResourceSpans struct {
	Resource   otlpResource     `json:"resource"`
	ScopeSpans []topoScopeSpans `json:"scopeSpans"`
}

// topoPayload is a minimal unmarshal target for OTLP trace JSON.
type topoPayload struct {
	ResourceSpans []topoResourceSpans `json:"resourceSpans"`
}

// buildSpanIndex returns a map from spanID → serviceName for every span in the payload.
func buildSpanIndex(payload *topoPayload) map[string]string {
	index := make(map[string]string)
	for _, rs := range payload.ResourceSpans {
		svc := serviceName(rs.Resource)
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				if span.SpanID != "" {
					index[span.SpanID] = svc
				}
			}
		}
	}
	return index
}

// emitTopology parses raw OTLP JSON trace data, builds a span index, then walks
// every span. For each span whose ParentSpanID appears in the index it emits one
// AddEdge(parentService, currentService) call via the sink. Edges are deduped
// within a single request. Unknown parents are silently skipped (one log line).
func emitTopology(data []byte, sink TopologySink) {
	var payload topoPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Printf("otel/topology: failed to parse traces for topology: %v", err)
		return
	}

	index := buildSpanIndex(&payload)

	type edge struct{ from, to string }
	seen := make(map[edge]struct{})

	for _, rs := range payload.ResourceSpans {
		svc := serviceName(rs.Resource)
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				if span.ParentSpanID == "" {
					// root span — no parent, skip
					continue
				}
				parentSvc, ok := index[span.ParentSpanID]
				if !ok {
					log.Printf("otel/topology: parent span %q not found in request index (skipped)", span.ParentSpanID)
					continue
				}
				e := edge{from: parentSvc, to: svc}
				if _, dup := seen[e]; dup {
					continue
				}
				seen[e] = struct{}{}
				sink.AddEdge(parentSvc, svc)
			}
		}
	}
}
