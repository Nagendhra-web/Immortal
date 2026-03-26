package healing

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/immortal-engine/immortal/internal/event"
)

const defaultActionTimeout = 30 * time.Second

type Healer struct {
	mu        sync.RWMutex
	rules     []Rule
	ghostMode bool
	history   []HealRecord
}

type HealRecord struct {
	RuleName string
	EventID  string
	Success  bool
	Error    string
}

func NewHealer() *Healer {
	return &Healer{}
}

func (h *Healer) AddRule(rule Rule) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.rules = append(h.rules, rule)
}

func (h *Healer) SetGhostMode(enabled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ghostMode = enabled
}

func (h *Healer) Handle(e *event.Event) []Recommendation {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var recommendations []Recommendation

	for _, rule := range h.rules {
		if rule.Match(e) {
			rec := Recommendation{
				RuleName: rule.Name,
				Event:    e,
				Message:  fmt.Sprintf("rule '%s' matched event: %s", rule.Name, e.Message),
			}
			recommendations = append(recommendations, rec)

			if !h.ghostMode {
				go func(r Rule) {
					record := HealRecord{
						RuleName: r.Name,
						EventID:  e.ID,
					}

					// Run action with timeout + panic recovery
					done := make(chan error, 1)
					go func() {
						defer func() {
							if p := recover(); p != nil {
								done <- fmt.Errorf("panic: %v", p)
							}
						}()
						done <- r.Action(e)
					}()

					select {
					case err := <-done:
						record.Success = err == nil
						if err != nil {
							record.Error = err.Error()
							log.Printf("[immortal] healing action '%s' failed: %v", r.Name, err)
						}
					case <-time.After(defaultActionTimeout):
						record.Success = false
						record.Error = fmt.Sprintf("timeout after %s", defaultActionTimeout)
						log.Printf("[immortal] healing action '%s' timed out after %s", r.Name, defaultActionTimeout)
					}

					h.mu.Lock()
					h.history = append(h.history, record)
					if len(h.history) > 10000 {
						h.history = h.history[len(h.history)-5000:]
					}
					h.mu.Unlock()
				}(rule)
			}
		}
	}

	return recommendations
}

func (h *Healer) History() []HealRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]HealRecord, len(h.history))
	copy(out, h.history)
	return out
}
