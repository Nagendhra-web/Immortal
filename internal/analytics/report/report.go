package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/Nagendhra-web/Immortal/internal/analytics/metrics"
	"github.com/Nagendhra-web/Immortal/internal/health"
)

type Report struct {
	Title     string    `json:"title"`
	Generated time.Time `json:"generated"`
	Period    string    `json:"period"`
	Sections  []Section `json:"sections"`
}

type Section struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type Generator struct {
	aggregator *metrics.Aggregator
	registry   *health.Registry
}

func NewGenerator(agg *metrics.Aggregator, reg *health.Registry) *Generator {
	return &Generator{aggregator: agg, registry: reg}
}

func (g *Generator) Generate(title string) *Report {
	r := &Report{
		Title:     title,
		Generated: time.Now(),
		Period:    "current",
	}

	// Health section
	r.Sections = append(r.Sections, g.healthSection())

	// Metrics section
	r.Sections = append(r.Sections, g.metricsSection())

	return r
}

func (g *Generator) healthSection() Section {
	services := g.registry.All()
	var lines []string
	lines = append(lines, fmt.Sprintf("Overall: %s", g.registry.OverallStatus()))
	lines = append(lines, fmt.Sprintf("Services: %d", len(services)))
	for _, svc := range services {
		uptime := g.registry.UptimePercent(svc.Name)
		lines = append(lines, fmt.Sprintf("  - %s: %s (%.1f%% uptime)", svc.Name, svc.Status, uptime))
	}
	return Section{Name: "Health", Content: strings.Join(lines, "\n")}
}

func (g *Generator) metricsSection() Section {
	names := g.aggregator.Names()
	var lines []string
	for _, name := range names {
		s := g.aggregator.Summarize(name)
		if s == nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s: mean=%.2f p95=%.2f p99=%.2f min=%.2f max=%.2f",
			name, s.Mean, s.P95, s.P99, s.Min, s.Max))
	}
	if len(lines) == 0 {
		lines = append(lines, "  No metrics recorded yet")
	}
	return Section{Name: "Metrics", Content: strings.Join(lines, "\n")}
}

func (r *Report) String() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("=== %s ===\n", r.Title))
	b.WriteString(fmt.Sprintf("Generated: %s\n\n", r.Generated.Format(time.RFC3339)))
	for _, s := range r.Sections {
		b.WriteString(fmt.Sprintf("[%s]\n%s\n\n", s.Name, s.Content))
	}
	return b.String()
}
