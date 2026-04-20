# Grafana dashboards

## `immortal-grafana.json`

Single-panel dashboard that surfaces everything the engine exposes via `/api/metrics` (Prometheus format). 16 panels across four rows:

1. **Engine**: events/s, healings/min, active incidents, health score
2. **Throughput + latency**: per-source ingestion rate, p99 healing duration
3. **Intelligence layer**: DNA anomalies, twin simulations (accepted/rejected), agentic ReAct step p95
4. **Audit + provenance**: PQ audit entry count, chain verification state, per-service SLA uptime
5. **Recent incidents**: table view, top 20 by severity

## Install

1. In Prometheus, scrape the engine:

   ```yaml
   scrape_configs:
     - job_name: immortal
       static_configs:
         - targets: ["localhost:7777"]
       metrics_path: /api/metrics
       scrape_interval: 10s
   ```

2. In Grafana, **Dashboards > New > Import**, upload `immortal-grafana.json`.

3. Select your Prometheus data source when prompted. The dashboard variable `${HOST}` controls the "Operator dashboard" link; default `127.0.0.1`.

## Metrics

All metrics have the `immortal_` prefix and the conventional Prometheus naming:

| Metric | Description |
| ------ | ----------- |
| `immortal_events_ingested_total` | counter, labeled by `source` |
| `immortal_healings_total` | counter |
| `immortal_healing_duration_seconds` | histogram |
| `immortal_incidents_active` | gauge |
| `immortal_health_score` | gauge (0..1) |
| `immortal_anomalies_detected_total` | counter, labeled by `service` |
| `immortal_twin_simulations_total` | counter, labeled by `accepted` (bool) |
| `immortal_agentic_steps` | histogram (steps per ReAct run) |
| `immortal_pqaudit_entries` | gauge |
| `immortal_pqaudit_verified` | gauge (1/0) |
| `immortal_sla_uptime_ratio` | gauge, labeled by `service` |
| `immortal_incident_severity` | gauge, one series per incident |

## Customization

- **Thresholds**: edit the `thresholds.steps` arrays per panel. Defaults target a 867k events/sec rig; lower them for smaller deployments.
- **Refresh**: default `10s`. Change via the dashboard settings.
- **Time range**: default `now-1h` rolling; switch to longer windows for weekly review.

## Alerting

This dashboard is view-only. For alerts, pair with `prometheus-alert.yml` (coming in a follow-up) or define Grafana alerts on any panel via the panel menu.
