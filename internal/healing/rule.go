package healing

import (
	"strings"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

type MatchFunc func(e *event.Event) bool
type ActionFunc func(e *event.Event) error

type Rule struct {
	Name   string
	Match  MatchFunc
	Action ActionFunc
}

type Recommendation struct {
	RuleName string
	Event    *event.Event
	Message  string
}

func MatchSeverity(min event.Severity) MatchFunc {
	return func(e *event.Event) bool {
		return e.Severity.Level() >= min.Level()
	}
}

func MatchSource(source string) MatchFunc {
	return func(e *event.Event) bool {
		return strings.EqualFold(e.Source, source)
	}
}

func MatchAll(matchers ...MatchFunc) MatchFunc {
	return func(e *event.Event) bool {
		for _, m := range matchers {
			if !m(e) {
				return false
			}
		}
		return true
	}
}

func MatchContains(substr string) MatchFunc {
	return func(e *event.Event) bool {
		return strings.Contains(strings.ToLower(e.Message), strings.ToLower(substr))
	}
}
