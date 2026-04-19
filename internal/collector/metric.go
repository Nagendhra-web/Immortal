package collector

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/Nagendhra-web/Immortal/internal/event"
)

type MetricCollector struct {
	interval time.Duration
	callback EventCallback
	done     chan struct{}
}

func NewMetricCollector(interval time.Duration, callback EventCallback) *MetricCollector {
	return &MetricCollector{
		interval: interval,
		callback: callback,
		done:     make(chan struct{}),
	}
}

func (m *MetricCollector) Name() string { return "metric:system" }

func (m *MetricCollector) Start() error {
	go m.run()
	return nil
}

func (m *MetricCollector) Stop() error {
	close(m.done)
	return nil
}

func (m *MetricCollector) run() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			m.collect()
		}
	}
}

func (m *MetricCollector) collect() {
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		severity := event.SeverityInfo
		if cpuPercent[0] > 90 {
			severity = event.SeverityCritical
		} else if cpuPercent[0] > 75 {
			severity = event.SeverityWarning
		}

		e := event.New(event.TypeMetric, severity,
			fmt.Sprintf("cpu usage: %.1f%%", cpuPercent[0])).
			WithSource("system").
			WithMeta("cpu_percent", cpuPercent[0])
		m.callback(e)
	}

	memInfo, err := mem.VirtualMemory()
	if err == nil {
		severity := event.SeverityInfo
		if memInfo.UsedPercent > 90 {
			severity = event.SeverityCritical
		} else if memInfo.UsedPercent > 75 {
			severity = event.SeverityWarning
		}

		e := event.New(event.TypeMetric, severity,
			fmt.Sprintf("memory usage: %.1f%%", memInfo.UsedPercent)).
			WithSource("system").
			WithMeta("memory_percent", memInfo.UsedPercent).
			WithMeta("memory_used_bytes", memInfo.Used).
			WithMeta("memory_total_bytes", memInfo.Total)
		m.callback(e)
	}
}
