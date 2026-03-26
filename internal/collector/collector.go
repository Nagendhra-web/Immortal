package collector

import "github.com/immortal-engine/immortal/internal/event"

// Collector is the interface all collectors implement.
type Collector interface {
	Start() error
	Stop() error
	Name() string
}

// EventCallback is called when a collector produces an event.
type EventCallback func(e *event.Event)
