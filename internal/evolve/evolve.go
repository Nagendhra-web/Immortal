// Package evolve is the architecture advisor. It scans observed signals
// (latency distribution, error rate, cache behaviour, retry rate, dependency
// graph) and produces scored suggestions for structural changes the
// operator should consider.
//
// It does not apply changes. It proposes them. Each suggestion carries
// evidence, predicted impact, and an effort estimate so the operator can
// triage.
package evolve

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Kind enumerates the structural changes the advisor knows how to recommend.
type Kind int

const (
	AddCache Kind = iota
	SplitService
	AddQueue
	MoveToAsync
	AddCircuitBreaker
	ReplicateService
	RemoveDependency
	TightenTimeout
	AddRetryBudget
)

// String gives the Kind a human-readable label.
func (k Kind) String() string {
	switch k {
	case AddCache:
		return "add-cache"
	case SplitService:
		return "split-service"
	case AddQueue:
		return "add-queue"
	case MoveToAsync:
		return "move-to-async"
	case AddCircuitBreaker:
		return "add-circuit-breaker"
	case ReplicateService:
		return "replicate-service"
	case RemoveDependency:
		return "remove-dependency"
	case TightenTimeout:
		return "tighten-timeout"
	case AddRetryBudget:
		return "add-retry-budget"
	default:
		return "unknown"
	}
}

// Effort is a rough implementation-cost estimate.
type Effort string

const (
	Small  Effort = "small"  // config change, within one service
	Medium Effort = "medium" // code change, fits in a sprint
	Large  Effort = "large"  // multi-service refactor, a quarter
)

// Suggestion is the output of the advisor.
type Suggestion struct {
	Kind      Kind
	Service   string
	Rationale string   // one-paragraph "why this matters right now"
	Evidence  []string // observed signals that fired the rule
	Impact    string   // predicted benefit if applied
	Effort    Effort
	Score     float64   // 0.0 - 1.0 confidence
	CreatedAt time.Time
}

// SignalBag is the advisor's view of recent system behaviour. It is
// intentionally a bag of simple scalars the caller fills in; the advisor
// does not reach out to collectors itself.
type SignalBag struct {
	// Per-service metrics. All values are current or recent-window averages.
	LatencyP99       map[string]float64 // ms
	LatencyCoeffVar  map[string]float64 // CV = stddev/mean; >0.5 = bursty
	ErrorRate        map[string]float64 // 0.0-1.0
	RetryRate        map[string]float64 // retries per request
	CacheHitRate     map[string]float64 // 0.0-1.0 (1.0 means no need for more cache)
	DependentCount   map[string]int     // how many services call this one
	DependencyCount  map[string]int     // how many downstreams this service calls
	QueueDepth       map[string]float64 // steady-state queue depth
	SyncCritPath     map[string]bool    // true if service sits on a synchronous critical path
	IncidentCount30d map[string]int     // incidents where this service appeared in the last 30 days
}

// Advisor analyses a SignalBag and returns ranked suggestions.
type Advisor struct {
	mu  sync.Mutex
	now func() time.Time
}

// New returns a ready-to-use Advisor.
func New() *Advisor {
	return &Advisor{now: time.Now}
}

// Analyze runs every heuristic rule and returns sorted suggestions.
func (a *Advisor) Analyze(sig SignalBag) []Suggestion {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []Suggestion
	for _, fn := range rules {
		out = append(out, fn(sig, a.now())...)
	}
	// Deduplicate per (kind, service). When multiple rules target the same
	// pair, keep the highest-scoring one and merge evidence.
	out = dedupe(out)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// ── heuristic rules ────────────────────────────────────────────────────────

type rule func(SignalBag, time.Time) []Suggestion

var rules = []rule{
	ruleAddCache,
	ruleSplitService,
	ruleAddQueue,
	ruleMoveToAsync,
	ruleAddCircuitBreaker,
	ruleReplicateService,
	ruleTightenTimeout,
	ruleAddRetryBudget,
}

// ruleAddCache: high p99 latency + low cache hit rate on the same service.
func ruleAddCache(s SignalBag, now time.Time) []Suggestion {
	var out []Suggestion
	for svc, lat := range s.LatencyP99 {
		hit, hasHit := s.CacheHitRate[svc]
		if !hasHit || hit > 0.85 {
			continue
		}
		if lat < 100 {
			continue
		}
		score := clamp01(((lat-100)/400)*0.5 + (0.85-hit)*0.5)
		out = append(out, Suggestion{
			Kind:      AddCache,
			Service:   svc,
			Rationale: fmt.Sprintf("%s has p99 latency %.0f ms and cache hit rate %.0f%%. A read-through cache on its hot path would absorb repeat reads and cut latency.", svc, lat, hit*100),
			Evidence:  []string{fmt.Sprintf("latency_p99=%.0fms", lat), fmt.Sprintf("cache_hit_rate=%.2f", hit)},
			Impact:    "Typically reduces p99 by 30-60% when the read skew is concentrated.",
			Effort:    Medium,
			Score:     score,
			CreatedAt: now,
		})
	}
	return out
}

// ruleSplitService: a service is called by many others AND has bursty latency.
// Fan-in that big becomes a single point of contention.
func ruleSplitService(s SignalBag, now time.Time) []Suggestion {
	var out []Suggestion
	for svc, dependents := range s.DependentCount {
		if dependents < 5 {
			continue
		}
		cv, ok := s.LatencyCoeffVar[svc]
		if !ok || cv < 0.5 {
			continue
		}
		score := clamp01(float64(dependents-5)/10*0.5 + (cv-0.5)*0.5)
		out = append(out, Suggestion{
			Kind:      SplitService,
			Service:   svc,
			Rationale: fmt.Sprintf("%s is called by %d services and has bursty latency (CV %.2f). Splitting by read/write or by tenant keeps fan-in contention localised.", svc, dependents, cv),
			Evidence:  []string{fmt.Sprintf("dependents=%d", dependents), fmt.Sprintf("latency_cv=%.2f", cv)},
			Impact:    "Isolates tail latency; each split serves a narrower workload.",
			Effort:    Large,
			Score:     score,
			CreatedAt: now,
		})
	}
	return out
}

// ruleAddQueue: sustained queue depth OR synchronous critical path with high variance.
func ruleAddQueue(s SignalBag, now time.Time) []Suggestion {
	var out []Suggestion
	for svc, depth := range s.QueueDepth {
		if depth < 50 {
			continue
		}
		score := clamp01(depth / 500)
		out = append(out, Suggestion{
			Kind:      AddQueue,
			Service:   svc,
			Rationale: fmt.Sprintf("%s shows sustained queue depth around %.0f. Adding a durable buffer (or sizing the existing one) keeps bursts from dropping jobs.", svc, depth),
			Evidence:  []string{fmt.Sprintf("queue_depth=%.0f", depth)},
			Impact:    "Prevents job loss under spikes; trades latency for durability.",
			Effort:    Medium,
			Score:     score,
			CreatedAt: now,
		})
	}
	return out
}

// ruleMoveToAsync: service sits on synchronous critical path AND has high latency CV.
func ruleMoveToAsync(s SignalBag, now time.Time) []Suggestion {
	var out []Suggestion
	for svc, onCrit := range s.SyncCritPath {
		if !onCrit {
			continue
		}
		cv, ok := s.LatencyCoeffVar[svc]
		if !ok || cv < 0.6 {
			continue
		}
		score := clamp01((cv - 0.6) * 1.2)
		out = append(out, Suggestion{
			Kind:      MoveToAsync,
			Service:   svc,
			Rationale: fmt.Sprintf("%s sits on the synchronous critical path and has high latency variance (CV %.2f). Moving the non-essential parts to async would smooth the tail.", svc, cv),
			Evidence:  []string{"sync_critical_path=true", fmt.Sprintf("latency_cv=%.2f", cv)},
			Impact:    "Removes a stochastic contributor from the tail.",
			Effort:    Large,
			Score:     score,
			CreatedAt: now,
		})
	}
	return out
}

// ruleAddCircuitBreaker: dependent count high AND downstream error rate elevated.
func ruleAddCircuitBreaker(s SignalBag, now time.Time) []Suggestion {
	var out []Suggestion
	for svc, deps := range s.DependencyCount {
		if deps < 3 {
			continue
		}
		errs, ok := s.ErrorRate[svc]
		if !ok || errs < 0.02 {
			continue
		}
		score := clamp01(float64(deps-3)/5*0.4 + (errs-0.02)*20*0.6)
		out = append(out, Suggestion{
			Kind:      AddCircuitBreaker,
			Service:   svc,
			Rationale: fmt.Sprintf("%s depends on %d downstreams and is showing %.1f%% errors. A circuit breaker would stop cascading failures when a downstream misbehaves.", svc, deps, errs*100),
			Evidence:  []string{fmt.Sprintf("dependencies=%d", deps), fmt.Sprintf("error_rate=%.4f", errs)},
			Impact:    "Prevents error cascades; fails fast with a clear signal.",
			Effort:    Small,
			Score:     score,
			CreatedAt: now,
		})
	}
	return out
}

// ruleReplicateService: service has many dependents AND low redundancy (implied by incident count).
func ruleReplicateService(s SignalBag, now time.Time) []Suggestion {
	var out []Suggestion
	for svc, deps := range s.DependentCount {
		if deps < 5 {
			continue
		}
		inc := s.IncidentCount30d[svc]
		if inc < 2 {
			continue
		}
		score := clamp01(float64(inc)/10*0.6 + float64(deps-5)/10*0.4)
		out = append(out, Suggestion{
			Kind:      ReplicateService,
			Service:   svc,
			Rationale: fmt.Sprintf("%s has %d dependents and appeared in %d incidents in the last 30 days. Replicating across zones/instances would absorb single-instance failures.", svc, deps, inc),
			Evidence:  []string{fmt.Sprintf("dependents=%d", deps), fmt.Sprintf("incidents_30d=%d", inc)},
			Impact:    "Cuts single-point-of-failure risk; adds capacity headroom.",
			Effort:    Medium,
			Score:     score,
			CreatedAt: now,
		})
	}
	return out
}

// ruleTightenTimeout: retry rate high suggests timeouts are too generous.
// Capped at 0.75 because tightening alone cannot fix a true retry storm;
// AddRetryBudget is the structural answer at high retry rates.
func ruleTightenTimeout(s SignalBag, now time.Time) []Suggestion {
	var out []Suggestion
	for svc, retry := range s.RetryRate {
		if retry < 0.1 {
			continue
		}
		score := clamp01(retry * 2)
		if score > 0.75 {
			score = 0.75
		}
		out = append(out, Suggestion{
			Kind:      TightenTimeout,
			Service:   svc,
			Rationale: fmt.Sprintf("%s has %.1f retries per request. Shorter timeouts would fail fast and free up client capacity faster.", svc, retry),
			Evidence:  []string{fmt.Sprintf("retry_rate=%.2f", retry)},
			Impact:    "Reduces blocked client capacity; exposes real latency faster.",
			Effort:    Small,
			Score:     score,
			CreatedAt: now,
		})
	}
	return out
}

// ruleAddRetryBudget: retry rate VERY high = budget exhausted, classic retry storm.
func ruleAddRetryBudget(s SignalBag, now time.Time) []Suggestion {
	var out []Suggestion
	for svc, retry := range s.RetryRate {
		if retry < 0.3 {
			continue
		}
		score := clamp01(retry)
		out = append(out, Suggestion{
			Kind:      AddRetryBudget,
			Service:   svc,
			Rationale: fmt.Sprintf("%s has %.1f retries per request. This is a retry storm signature; cap the retry budget (e.g. max 3, or 10%% of non-retry traffic).", svc, retry),
			Evidence:  []string{fmt.Sprintf("retry_rate=%.2f", retry)},
			Impact:    "Stops the fan-out amplification that drives cascading failures.",
			Effort:    Small,
			Score:     score,
			CreatedAt: now,
		})
	}
	return out
}

// ── utilities ──────────────────────────────────────────────────────────────

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func dedupe(ss []Suggestion) []Suggestion {
	seen := make(map[string]*Suggestion, len(ss))
	order := make([]string, 0, len(ss))
	for i := range ss {
		key := fmt.Sprintf("%s::%s", ss[i].Kind, ss[i].Service)
		if prev, ok := seen[key]; ok {
			if ss[i].Score > prev.Score {
				prev.Score = ss[i].Score
				prev.Rationale = ss[i].Rationale
				prev.Impact = ss[i].Impact
			}
			prev.Evidence = mergeEvidence(prev.Evidence, ss[i].Evidence)
			continue
		}
		copy := ss[i]
		seen[key] = &copy
		order = append(order, key)
	}
	out := make([]Suggestion, 0, len(order))
	for _, k := range order {
		out = append(out, *seen[k])
	}
	return out
}

func mergeEvidence(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []string
	for _, s := range a {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, s := range b {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// Format returns a short human string for logs or UI. Stable output.
func (s Suggestion) Format() string {
	return fmt.Sprintf("[%s, score=%.2f, effort=%s] %s: %s",
		s.Kind, s.Score, s.Effort, s.Service, strings.TrimSuffix(s.Rationale, "."),
	)
}
