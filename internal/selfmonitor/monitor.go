package selfmonitor

import (
	"runtime"
	"sync"
	"time"
)

type Stats struct {
	Goroutines      int           `json:"goroutines"`
	HeapAlloc       uint64        `json:"heap_alloc_bytes"`
	HeapSys         uint64        `json:"heap_sys_bytes"`
	NumGC           uint32        `json:"num_gc"`
	Uptime          time.Duration `json:"uptime"`
	EventsProcessed int64        `json:"events_processed"`
	HealsExecuted   int64        `json:"heals_executed"`
}

type Monitor struct {
	mu              sync.RWMutex
	startedAt       time.Time
	eventsProcessed int64
	healsExecuted   int64
}

func New() *Monitor {
	return &Monitor{startedAt: time.Now()}
}

func (m *Monitor) RecordEvent() {
	m.mu.Lock()
	m.eventsProcessed++
	m.mu.Unlock()
}

func (m *Monitor) RecordHeal() {
	m.mu.Lock()
	m.healsExecuted++
	m.mu.Unlock()
}

func (m *Monitor) Stats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return Stats{
		Goroutines:      runtime.NumGoroutine(),
		HeapAlloc:       mem.HeapAlloc,
		HeapSys:         mem.HeapSys,
		NumGC:           mem.NumGC,
		Uptime:          time.Since(m.startedAt),
		EventsProcessed: m.eventsProcessed,
		HealsExecuted:   m.healsExecuted,
	}
}

func (m *Monitor) IsHealthy() bool {
	stats := m.Stats()
	if stats.Goroutines > 10000 {
		return false
	}
	if stats.HeapAlloc > 1<<30 {
		return false
	}
	return true
}
