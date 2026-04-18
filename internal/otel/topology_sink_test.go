package otel

import (
	"testing"
)

// ---------- TestTopologySinkFunc_AdaptsClosure ----------

func TestTopologySinkFunc_AdaptsClosure(t *testing.T) {
	var calls []string
	fn := TopologySinkFunc(func(from, to string) {
		calls = append(calls, from+"->"+to)
	})
	fn.AddEdge("a", "b")
	fn.AddEdge("c", "d")
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(calls))
	}
	if calls[0] != "a->b" || calls[1] != "c->d" {
		t.Errorf("unexpected calls: %v", calls)
	}
}

// ---------- TestBuildSpanIndex_MapsSpanIDsToServiceNames ----------

func TestBuildSpanIndex_MapsSpanIDsToServiceNames(t *testing.T) {
	payload := &topoPayload{
		ResourceSpans: []topoResourceSpans{
			{
				Resource: otlpResource{Attributes: []otlpKeyValue{
					{Key: "service.name", Value: map[string]any{"stringValue": "svc-a"}},
				}},
				ScopeSpans: []topoScopeSpans{{Spans: []topoSpan{
					{SpanID: "span-a1", ParentSpanID: ""},
					{SpanID: "span-a2", ParentSpanID: "span-a1"},
				}}},
			},
			{
				Resource: otlpResource{Attributes: []otlpKeyValue{
					{Key: "service.name", Value: map[string]any{"stringValue": "svc-b"}},
				}},
				ScopeSpans: []topoScopeSpans{{Spans: []topoSpan{
					{SpanID: "span-b1", ParentSpanID: ""},
					{SpanID: "span-b2", ParentSpanID: "span-b1"},
				}}},
			},
		},
	}

	index := buildSpanIndex(payload)

	want := map[string]string{
		"span-a1": "svc-a",
		"span-a2": "svc-a",
		"span-b1": "svc-b",
		"span-b2": "svc-b",
	}
	for spanID, wantSvc := range want {
		if got := index[spanID]; got != wantSvc {
			t.Errorf("index[%q] = %q, want %q", spanID, got, wantSvc)
		}
	}
	if len(index) != len(want) {
		t.Errorf("index length: want %d, got %d", len(want), len(index))
	}
}

// ---------- TestEmitTopology_ParentChildEdge ----------

// Resource A has span S1 (child); Resource B has span P1 (parent).
// S1.parentSpanID = P1.spanID → expect edge (svc-b → svc-a).
func TestEmitTopology_ParentChildEdge(t *testing.T) {
	json := `{
		"resourceSpans":[
			{
				"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc-a"}}]},
				"scopeSpans":[{"spans":[
					{"spanId":"span-child","parentSpanId":"span-parent","name":"child-op"}
				]}]
			},
			{
				"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc-b"}}]},
				"scopeSpans":[{"spans":[
					{"spanId":"span-parent","parentSpanId":"","name":"parent-op"}
				]}]
			}
		]
	}`

	type edge struct{ from, to string }
	var edges []edge
	sink := TopologySinkFunc(func(from, to string) {
		edges = append(edges, edge{from, to})
	})

	emitTopology([]byte(json), sink)

	if len(edges) != 1 {
		t.Fatalf("want 1 edge, got %d: %v", len(edges), edges)
	}
	if edges[0].from != "svc-b" || edges[0].to != "svc-a" {
		t.Errorf("want svc-b->svc-a, got %s->%s", edges[0].from, edges[0].to)
	}
}

// ---------- TestEmitTopology_DedupesWithinRequest ----------

func TestEmitTopology_DedupesWithinRequest(t *testing.T) {
	// 5 child spans all pointing to the same parent span in the same service pair.
	json := `{
		"resourceSpans":[
			{
				"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc-child"}}]},
				"scopeSpans":[{"spans":[
					{"spanId":"c1","parentSpanId":"p1"},
					{"spanId":"c2","parentSpanId":"p1"},
					{"spanId":"c3","parentSpanId":"p1"},
					{"spanId":"c4","parentSpanId":"p1"},
					{"spanId":"c5","parentSpanId":"p1"}
				]}]
			},
			{
				"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc-parent"}}]},
				"scopeSpans":[{"spans":[
					{"spanId":"p1","parentSpanId":""}
				]}]
			}
		]
	}`

	callCount := 0
	sink := TopologySinkFunc(func(from, to string) { callCount++ })

	emitTopology([]byte(json), sink)

	if callCount != 1 {
		t.Errorf("want deduped to 1 call, got %d", callCount)
	}
}

// ---------- TestEmitTopology_IgnoresRootSpans ----------

func TestEmitTopology_IgnoresRootSpans(t *testing.T) {
	json := `{
		"resourceSpans":[{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc-root"}}]},
			"scopeSpans":[{"spans":[
				{"spanId":"root1","parentSpanId":""},
				{"spanId":"root2","parentSpanId":""}
			]}]
		}]
	}`

	callCount := 0
	sink := TopologySinkFunc(func(from, to string) { callCount++ })

	emitTopology([]byte(json), sink)

	if callCount != 0 {
		t.Errorf("want 0 edges for root spans, got %d", callCount)
	}
}

// ---------- TestEmitTopology_IgnoresUnknownParent ----------

func TestEmitTopology_IgnoresUnknownParent(t *testing.T) {
	// Parent span is NOT in this request's index.
	json := `{
		"resourceSpans":[{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"svc-orphan"}}]},
			"scopeSpans":[{"spans":[
				{"spanId":"child1","parentSpanId":"deadbeef00000000"}
			]}]
		}]
	}`

	callCount := 0
	sink := TopologySinkFunc(func(from, to string) { callCount++ })

	emitTopology([]byte(json), sink)

	if callCount != 0 {
		t.Errorf("want 0 edges for unknown parent, got %d", callCount)
	}
}
