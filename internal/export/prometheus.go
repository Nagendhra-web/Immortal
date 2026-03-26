package export

import (
	"fmt"
	"strings"
	"sync"
)

type PrometheusExporter struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]float64
}

func NewPrometheus() *PrometheusExporter {
	return &PrometheusExporter{
		gauges:   make(map[string]float64),
		counters: make(map[string]float64),
	}
}

func (p *PrometheusExporter) SetGauge(name string, value float64) {
	p.mu.Lock()
	p.gauges[name] = value
	p.mu.Unlock()
}

func (p *PrometheusExporter) IncCounter(name string) {
	p.mu.Lock()
	p.counters[name]++
	p.mu.Unlock()
}

func (p *PrometheusExporter) AddCounter(name string, value float64) {
	p.mu.Lock()
	p.counters[name] += value
	p.mu.Unlock()
}

func (p *PrometheusExporter) Export() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var b strings.Builder
	for name, val := range p.gauges {
		b.WriteString(fmt.Sprintf("# TYPE %s gauge\n%s %g\n", name, name, val))
	}
	for name, val := range p.counters {
		b.WriteString(fmt.Sprintf("# TYPE %s counter\n%s %g\n", name, name, val))
	}
	return b.String()
}

func (p *PrometheusExporter) Get(name string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if v, ok := p.gauges[name]; ok {
		return v
	}
	if v, ok := p.counters[name]; ok {
		return v
	}
	return 0
}
