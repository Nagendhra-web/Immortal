package autolearn

import (
	"sort"
	"sync"
	"time"
)

type HealEvent struct {
	RuleName  string    `json:"rule_name"`
	Source    string    `json:"source"`
	Message  string    `json:"message"`
	Severity string    `json:"severity"`
	Success  bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
}

type LearnedRule struct {
	Name        string    `json:"name"`
	Pattern     string    `json:"pattern"`
	Source      string    `json:"source"`
	Severity    string    `json:"severity"`
	Confidence  float64   `json:"confidence"`
	Occurrences int       `json:"occurrences"`
	Successes   int       `json:"successes"`
	LastSeen    time.Time `json:"last_seen"`
	Suggested   bool      `json:"suggested"`
}

type Learner struct {
	mu        sync.RWMutex
	events    []HealEvent
	rules     map[string]*LearnedRule
	threshold int
	maxEvents int
}

func New(threshold int) *Learner {
	if threshold <= 0 {
		threshold = 5
	}
	return &Learner{
		rules:     make(map[string]*LearnedRule),
		threshold: threshold,
		maxEvents: 50000,
	}
}

func (l *Learner) Record(ruleName, source, message, severity string, success bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ev := HealEvent{
		RuleName:  ruleName,
		Source:    source,
		Message:  message,
		Severity: severity,
		Success:  success,
		Timestamp: time.Now(),
	}

	l.events = append(l.events, ev)
	if len(l.events) > l.maxEvents {
		l.events = l.events[len(l.events)-l.maxEvents:]
	}

	pattern := source + ":" + severity

	rule, ok := l.rules[pattern]
	if !ok {
		rule = &LearnedRule{
			Name:     "auto-" + pattern,
			Pattern:  pattern,
			Source:   source,
			Severity: severity,
		}
		l.rules[pattern] = rule
	}

	rule.Occurrences++
	if success {
		rule.Successes++
	}
	rule.LastSeen = ev.Timestamp
	rule.Confidence = float64(rule.Successes) / float64(rule.Occurrences)

	if rule.Occurrences >= l.threshold && rule.Confidence > 0.5 {
		rule.Suggested = true
	}
}

func (l *Learner) SuggestedRules() []LearnedRule {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var suggested []LearnedRule
	for _, r := range l.rules {
		if r.Suggested && r.Confidence > 0.5 {
			suggested = append(suggested, *r)
		}
	}

	sort.Slice(suggested, func(i, j int) bool {
		return suggested[i].Confidence > suggested[j].Confidence
	})
	return suggested
}

func (l *Learner) AllRules() []LearnedRule {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var all []LearnedRule
	for _, r := range l.rules {
		all = append(all, *r)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Occurrences > all[j].Occurrences
	})
	return all
}

func (l *Learner) Confidence(pattern string) float64 {
	l.mu.RLock()
	defer l.mu.RUnlock()

	r, ok := l.rules[pattern]
	if !ok {
		return 0
	}
	return r.Confidence
}

func (l *Learner) Forget(pattern string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.rules, pattern)
}

func (l *Learner) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = nil
	l.rules = make(map[string]*LearnedRule)
}

func (l *Learner) Stats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	suggested := 0
	topPattern := ""
	topCount := 0
	for _, r := range l.rules {
		if r.Suggested {
			suggested++
		}
		if r.Occurrences > topCount {
			topCount = r.Occurrences
			topPattern = r.Pattern
		}
	}

	return map[string]interface{}{
		"total_events":   len(l.events),
		"total_rules":    len(l.rules),
		"suggested_rules": suggested,
		"top_pattern":    topPattern,
	}
}
