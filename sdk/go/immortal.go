package immortal

import (
	"github.com/immortal-engine/immortal/internal/bus"
	"github.com/immortal-engine/immortal/internal/dna"
	"github.com/immortal-engine/immortal/internal/event"
	"github.com/immortal-engine/immortal/internal/healing"
)

type Config struct {
	Name      string
	Mode      string
	GhostMode bool
}

type App struct {
	config  Config
	bus     *bus.Bus
	healer  *healing.Healer
	dna     *dna.DNA
	running bool
}

func New(config Config) *App {
	if config.Name == "" {
		config.Name = "immortal-app"
	}
	if config.Mode == "" {
		config.Mode = "reactive"
	}
	h := healing.NewHealer()
	h.SetGhostMode(config.GhostMode)
	return &App{
		config: config,
		bus:    bus.New(),
		healer: h,
		dna:    dna.New(config.Name),
	}
}

func (a *App) Heal(rule healing.Rule) {
	a.healer.AddRule(rule)
}

func (a *App) Ingest(e *event.Event) {
	a.bus.Publish(e)
}

func (a *App) RecordMetric(name string, value float64) {
	a.dna.Record(name, value)
}

func (a *App) IsAnomaly(metric string, value float64) bool {
	return a.dna.IsAnomaly(metric, value)
}

func (a *App) HealthScore(current map[string]float64) float64 {
	return a.dna.HealthScore(current)
}

func (a *App) Start() {
	a.running = true
	a.bus.Subscribe("*", func(e *event.Event) {
		a.healer.Handle(e)
	})
}

func (a *App) Stop()           { a.running = false }
func (a *App) IsRunning() bool { return a.running }
func (a *App) Config() Config  { return a.config }

func MatchSeverity(min event.Severity) healing.MatchFunc { return healing.MatchSeverity(min) }
func MatchSource(source string) healing.MatchFunc        { return healing.MatchSource(source) }
func MatchContains(substr string) healing.MatchFunc      { return healing.MatchContains(substr) }
func ActionExec(cmd string) healing.ActionFunc           { return healing.ActionExec(cmd) }
