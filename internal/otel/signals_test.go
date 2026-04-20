package otel

import (
	"fmt"
	"testing"
	"time"
)

// buildOTLPTraces synthesizes an OTLP JSON payload for the given services.
// Each service emits one span of `latencyMs` and the given retry/error tag.
func buildOTLPTraces(specs []spanSpec) []byte {
	var rs []string
	for _, s := range specs {
		start := int64(1_700_000_000_000_000_000)
		end := start + int64(s.latencyMs)*1_000_000
		status := "0"
		if s.errored {
			status = "2"
		}
		retry := ""
		if s.retryCount > 0 {
			retry = fmt.Sprintf(`,{"key":"http.retry_count","value":{"intValue":"%d"}}`, s.retryCount)
		}
		peer := ""
		if s.peer != "" {
			peer = fmt.Sprintf(`,{"key":"peer.service","value":{"stringValue":"%s"}}`, s.peer)
		}
		rs = append(rs, fmt.Sprintf(`{
			"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"%s"}}]},
			"scopeSpans":[{"spans":[{
				"traceId":"a","spanId":"b","name":"op",
				"startTimeUnixNano":"%d","endTimeUnixNano":"%d",
				"status":{"code":%s},
				"attributes":[{"key":"dummy","value":{"stringValue":"x"}}%s%s]
			}]}]
		}`, s.service, start, end, status, retry, peer))
	}
	payload := `{"resourceSpans":[`
	for i, entry := range rs {
		if i > 0 {
			payload += ","
		}
		payload += entry
	}
	payload += `]}`
	return []byte(payload)
}

type spanSpec struct {
	service    string
	latencyMs  int
	errored    bool
	retryCount int
	peer       string
}

func TestSignals_IngestAndSignalBag(t *testing.T) {
	s := NewSignals(5 * time.Minute)
	payload := buildOTLPTraces([]spanSpec{
		{service: "api", latencyMs: 50},
		{service: "api", latencyMs: 100},
		{service: "api", latencyMs: 200},
		{service: "api", latencyMs: 300},
		{service: "api", latencyMs: 400, errored: true},
		{service: "api", latencyMs: 500, retryCount: 2, peer: "payments"},
	})
	if err := s.Ingest(payload); err != nil {
		t.Fatalf("ingest failed: %v", err)
	}
	bag := s.SignalBag()
	if p99 := bag.LatencyP99["api"]; p99 < 400 {
		t.Errorf("p99 should reflect the tail; got %v", p99)
	}
	if er := bag.ErrorRate["api"]; er < 0.15 || er > 0.20 {
		t.Errorf("error rate 1/6 ~ 0.166; got %v", er)
	}
	if rr := bag.RetryRate["api"]; rr < 0.15 || rr > 0.20 {
		t.Errorf("retry rate 1/6 ~ 0.166; got %v", rr)
	}
	if n := bag.DependencyCount["api"]; n != 1 {
		t.Errorf("api should have 1 dependency (payments); got %d", n)
	}
	if n := bag.DependentCount["payments"]; n != 1 {
		t.Errorf("payments should have 1 dependent (api); got %d", n)
	}
}

func TestSignals_CoeffVarReflectsSpread(t *testing.T) {
	s := NewSignals(5 * time.Minute)
	// Spread: 50, 50, 50, 500 -> high CV; flat: 100, 100, 100, 100 -> low CV.
	s.Ingest(buildOTLPTraces([]spanSpec{
		{service: "bursty", latencyMs: 50},
		{service: "bursty", latencyMs: 50},
		{service: "bursty", latencyMs: 50},
		{service: "bursty", latencyMs: 500},
		{service: "flat", latencyMs: 100},
		{service: "flat", latencyMs: 100},
		{service: "flat", latencyMs: 100},
		{service: "flat", latencyMs: 100},
	}))
	bag := s.SignalBag()
	if bag.LatencyCoeffVar["bursty"] < bag.LatencyCoeffVar["flat"] {
		t.Errorf("bursty CV (%.3f) should exceed flat CV (%.3f)", bag.LatencyCoeffVar["bursty"], bag.LatencyCoeffVar["flat"])
	}
	if bag.LatencyCoeffVar["flat"] > 0.01 {
		t.Errorf("flat distribution should have near-zero CV; got %v", bag.LatencyCoeffVar["flat"])
	}
}

func TestSignals_WindowExpiresOldSamples(t *testing.T) {
	s := NewSignals(1 * time.Second)
	t0 := time.Unix(1_700_000_000, 0)
	s.nowFn = func() time.Time { return t0 }
	s.Ingest(buildOTLPTraces([]spanSpec{{service: "api", latencyMs: 100}}))
	// Fast-forward: any Ingest after the window should trim old samples.
	s.nowFn = func() time.Time { return t0.Add(10 * time.Second) }
	s.Ingest(buildOTLPTraces([]spanSpec{{service: "api", latencyMs: 50}}))
	bag := s.SignalBag()
	// Only the second sample should remain; p99 should be 50 ms.
	if bag.LatencyP99["api"] != 50 {
		t.Errorf("old sample not expired; p99=%v (want 50)", bag.LatencyP99["api"])
	}
}

func TestSignals_EmptyWindowDefaultsToFiveMin(t *testing.T) {
	s := NewSignals(0) // should not panic; should default
	if s.window < time.Minute {
		t.Errorf("zero window should default; got %v", s.window)
	}
}

func TestSignals_MalformedPayloadReturnsError(t *testing.T) {
	s := NewSignals(time.Minute)
	if err := s.Ingest([]byte("not json")); err == nil {
		t.Errorf("malformed payload should return error")
	}
}

func TestSignals_Services_IsSorted(t *testing.T) {
	s := NewSignals(time.Minute)
	s.Ingest(buildOTLPTraces([]spanSpec{
		{service: "zoo", latencyMs: 1},
		{service: "alpha", latencyMs: 1},
		{service: "middle", latencyMs: 1},
	}))
	out := s.Services()
	if len(out) != 3 || out[0] != "alpha" || out[1] != "middle" || out[2] != "zoo" {
		t.Errorf("services not sorted: %v", out)
	}
}
