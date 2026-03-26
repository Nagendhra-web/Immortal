package connector

import (
	"fmt"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/process"

	"github.com/immortal-engine/immortal/internal/event"
)

type ProcessConfig struct {
	Name     string
	PID      int32
	Interval time.Duration
	Callback func(e *event.Event)
}

type ProcessConnector struct {
	config ProcessConfig
	done   chan struct{}
}

func NewProcessConnector(config ProcessConfig) *ProcessConnector {
	if config.Interval == 0 {
		config.Interval = 5 * time.Second
	}
	return &ProcessConnector{
		config: config,
		done:   make(chan struct{}),
	}
}

func (p *ProcessConnector) Start() error {
	go p.run()
	return nil
}

func (p *ProcessConnector) Stop() error {
	close(p.done)
	return nil
}

func (p *ProcessConnector) run() {
	ticker := time.NewTicker(p.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.check()
		}
	}
}

func (p *ProcessConnector) check() {
	procs, err := process.Processes()
	if err != nil {
		p.config.Callback(event.New(event.TypeError, event.SeverityError,
			fmt.Sprintf("failed to list processes: %v", err)).
			WithSource("process:" + p.config.Name))
		return
	}

	found := false
	for _, proc := range procs {
		name, err := proc.Name()
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(name), strings.ToLower(p.config.Name)) {
			found = true
			cpuPercent, _ := proc.CPUPercent()
			memInfo, _ := proc.MemoryInfo()

			severity := event.SeverityInfo
			msg := fmt.Sprintf("process '%s' (pid %d) is running", name, proc.Pid)

			if cpuPercent > 90 {
				severity = event.SeverityWarning
				msg = fmt.Sprintf("process '%s' (pid %d) high CPU: %.1f%%", name, proc.Pid, cpuPercent)
			}

			e := event.New(event.TypeHealth, severity, msg).
				WithSource("process:" + p.config.Name).
				WithMeta("pid", proc.Pid).
				WithMeta("cpu_percent", cpuPercent)

			if memInfo != nil {
				e.WithMeta("memory_rss", memInfo.RSS)
			}

			p.config.Callback(e)
			break
		}
	}

	if !found {
		p.config.Callback(
			event.New(event.TypeHealth, event.SeverityCritical,
				fmt.Sprintf("process '%s' is NOT running", p.config.Name)).
				WithSource("process:" + p.config.Name).
				WithMeta("status", "down"),
		)
	}
}
