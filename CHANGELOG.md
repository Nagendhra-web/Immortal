# Changelog

All notable changes to Immortal are documented here.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Immortal follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.7.0] - 2026-04-19

Platform release. Closes every remaining roadmap issue. 86 Go packages, 0 failing tests.

### Added

- **Kubernetes operator** at `operator/`. Three CRDs (`Intent`, `Playbook`, `Incident`) with validation schemas + status subresources. Full Helm chart with Deployment + ConfigMap + RBAC + Service + ServiceMonitor + hardened pod security (non-root, seccomp=RuntimeDefault, read-only root filesystem).
- **OTLP trace ingest signals** (`internal/otel/signals.go`). Rolling 5-minute aggregator derives per-service `LatencyP99`, `LatencyCoeffVar`, `ErrorRate`, `RetryRate`, and dependency graph from OTLP traces. Feeds directly into `evolve.SignalBag` so architecture advice reflects live traffic rather than synthetic numbers.
- **Slack + Discord ChatOps** (`internal/chatops/chatops.go`). Parser + dispatcher for `/immortal {status, incidents, explain, suggest, pause, resume}` with Slack Block Kit and Discord embed formatters. Every command is audit-logged via the pluggable `Auditor` interface. Zero external SDK dependencies.
- **eBPF-style host observer** (`internal/ebpf/`). Linux `/proc`-backed observer tracks TCP retransmit rate, fork rate, open-files pressure, and context switches. Watcher goroutine + `SignalsReport` maps to retry-storm / FD-exhaustion / runaway-workload risk scores. No-op implementation on macOS/Windows keeps the engine portable. A cilium/ebpf kprobe path is tracked as follow-up polish.
- **GitHub App auto-PR generator** (`internal/githubapp/githubapp.go`). Converts an `evolve.Suggestion` into a draft Proposal with branch name, PR title/body, and actual generated Go code for `AddCache`, `AddCircuitBreaker`, `TightenTimeout`, `AddRetryBudget`. Unknown kinds fall back to a manual-review stub. The PR body documents how to label-reject the pattern and train the advisor.
- **Long-horizon twin forecast** (`internal/twin/forecast.go`). `Twin.Forecast()` runs K=64 Monte-Carlo trajectories with AR(1) mean-reverting noise over any horizon/step combination. Returns per-metric per-service (mean, p10, p90) bands. Deterministic under fixed seed (covered by test).
- **GitOps / Argo CD integration** (`internal/gitops/gitops.go`). `Client.Commit()` writes state back to a GitOps repo via `git(1)`, attaches the narrator Verdict to the commit body, supports signed commits via GPG key. `IsOurCommit()` prevents reconcile loops by author-email match. `Rollback()` reverts + force-pushes when a post-apply twin check fails.
- **Incident replay to twin** (`internal/twin/replay.go`). `Twin.Replay()` rescores a historical incident Baseline both with and without a candidate Plan. Returns `{Accepted, UnmitigatedScore, MitigatedScore, ImprovementPct, Counterexample}` so a candidate fix is gated on twin-confirmed improvement before prod apply.
- **`/api/config` endpoint** (`internal/api/rest/config.go`). Read-only JSON dump of version metadata, per-feature flags, optional engine-config snapshot via a pluggable callback. Secrets never leak.
- **Grafana dashboard** (`dashboards/immortal-grafana.json`). 16 panels across 5 rows: engine KPIs, throughput + latency, intelligence layer (DNA, twin, agentic), audit + provenance, recent incidents. Prometheus data source variable; ready to import.
- **Systemd service file** (`systemd/immortal.service`). Hardened unit config with `NoNewPrivileges`, `ProtectSystem=strict`, `PrivateTmp`, `SystemCallFilter=@system-service`, `MemoryMax=2G`, `LimitNOFILE=65536`. Install + upgrade + uninstall runbook in `systemd/README.md`.
- **Contract examples** at `examples/` (`protect-checkout.yaml`, `cost-ceiling.yaml`, `never-drop-jobs.json`, `healing-rules.yaml` + README). Real intent contract shapes operators can copy, plus healing-rule patterns covering restart, cache clear, scale, circuit break, and paging.
- **Blog post** at `docs/blog/anomaly-detection.md`. Deep walkthrough of the DNA package: 3-sigma baselines, Welford online stats, warm-up period, per-service isolation, exponentially-weighted drift correction, and the failure modes 3-sigma misses.
- **Distribution templates** at `docs/SUBMISSIONS.md`. Copy-paste-ready PR bodies for awesome-go and awesome-selfhosted, plus a coordinated posting calendar for HN / Reddit / LinkedIn / Twitter.

### Security

- All images on GHCR now carry Sigstore-signed attestations with SLSA level 3 provenance and SPDX SBOMs (shipped in v0.6.x, verified end-to-end this release).

### Test suite

- 86 Go packages pass, 0 failures.

## [0.6.2] - 2026-04-19

### Added

- **Homebrew tap auto-bump**. The `HOMEBREW_TAP_TOKEN` secret is now configured, so GoReleaser publishes a formula update to `Nagendhra-web/homebrew-immortal` on every tag. Install with `brew tap Nagendhra-web/immortal && brew install immortal`.

## [0.6.1] - 2026-04-19

### Fixed

- **Docker publish job**. v0.6.0's first Docker-publish attempt failed with `Cache export is not supported for the docker driver` because `cache-from/to: type=gha` requires the `docker-container` buildx driver. The docker job now runs `docker/setup-qemu-action@v3` and `docker/setup-buildx-action@v3` first, producing a proper multi-arch builder. Multi-arch image (`linux/amd64` + `linux/arm64`) now publishes correctly to `ghcr.io/nagendhra-web/immortal`.

## [0.6.0] - 2026-04-19

Docker images, honest install story, and hardening based on a post-v0.5 audit.

### Added

- **Multi-arch Docker images** at `ghcr.io/nagendhra-web/immortal` (`linux/amd64` + `linux/arm64`). Auto-published on every tag via `actions/docker` with GoReleaser's version/commit/date baked in. First tag with the new release job. Pull with `docker pull ghcr.io/nagendhra-web/immortal:v0.6.0`.
- `runtime/debug.ReadBuildInfo()` fallback in `internal/version`, so `go install github.com/Nagendhra-web/Immortal/cmd/immortal@vX.Y.Z` users get the correct version string even without ldflags.
- `.github/ISSUE_TEMPLATE/config.yml` disables blank issues and routes users to private security advisories, Discussions, or enterprise support.
- GitHub Discussions enabled on the repo.

### Changed

- **`Dockerfile` hardened**: base bumped from `golang:1.22-alpine` (broken for our `go.mod` 1.25 requirement) to `golang:1.25-alpine`. Non-root user, `HEALTHCHECK` against `/api/health`, `-trimpath`, and VERSION/COMMIT/DATE build args so images carry correct version metadata.
- **`Makefile` LDFLAGS** corrected from the stale `github.com/immortal-engine/immortal` module path to `github.com/Nagendhra-web/Immortal`. `make build` now actually injects version info.
- **`scripts/install.sh`** normalizes `uname -s` so Git Bash / MSYS / Cygwin users no longer hit 404s from `immortal_mingw64_nt-...tar.gz` URLs. Windows shells are now redirected to `install.ps1`.
- **`README.md` + `docs/INSTALL.md`** no longer advertise `brew install immortal` (the tap formula does not yet exist). Homebrew is marked "planned for v0.6" with a short setup note. `docs/INSTALL.md` rewritten throughout with v0.5/v0.6 references and em dashes removed.
- Release + Pages workflows opt in to Node 24 runtime to clear Node 20 deprecation warnings.

### Fixed

- `version.Version` default was frozen at `0.3.0`. Every `go install` user saw the wrong version after installing v0.5.0.
- `.goreleaser.yaml` had a GoReleaser v1 `folder:` key under `brews:`, which caused v0.5.0's initial release workflow to fail. Uses `directory:` now.

### Security

- `SECURITY.md` supported-version matrix refreshed for v0.5/v0.6.

## [0.5.0] - 2026-04-19

First release with the new dashboard, install pipeline, and hosted landing page.

### Added

- **Operator dashboard rewrite (vanilla HTML/CSS/JS, zero deps)**. 52 files, embedded in the Go binary via `embed.FS`. 12 views:
  - Mission Control: overview, topology, audit, terminal
  - Intelligence: twin forecasts, agentic traces, causal root-cause (PCMCI), formal check
  - Authoring: NL to plan compiler, economic planner (Pareto frontier)
  - Knowledge: federated graph, post-quantum certificates
- Command palette (`Ctrl/Cmd + K`) with per-view scoped commands, fuzzy + subsequence matching, recency boost.
- Global keyboard shortcuts (`g o/t/a/w/A/f/c/n/e/F/x`, `/` filter, `.` pause, `?` help).
- Pure-SVG chart primitives: line, area, bar, sparkline, heatmap, force-directed graph. No chart library.
- Three-tier OKLCH design token system with dark default and light swap.
- Hosted landing page on GitHub Pages at https://nagendhra-web.github.io/Immortal/, with auto-deploy workflow.
- One-line installers (`install.sh`, `install.ps1`) that try pre-built binaries first, fall back to `go install`.
- GoReleaser v2 pipeline for `linux/darwin/windows` x `amd64/arm64`. Homebrew tap auto-bump is wired but disabled pending a `HOMEBREW_TAP_TOKEN` secret (planned for v0.6).
- Go vanity-import scaffolding at `vanity/index.html` for future `immortal.dev` domain.
- Enterprise support contact in landing-page footer (`nagendhra.madishetti24@gmail.com`).

### Changed

- Module path renamed from `github.com/immortal-engine/immortal` to `github.com/Nagendhra-web/Immortal` across all 152 Go files. `go install` now resolves correctly.
- `README.md` rewritten for discovery: why-section, comparison table, feature inventory, dashboard tour, community + roadmap sections.
- Repo description and topics updated for GitHub discovery (20 topics covering SRE, observability, agentic-ai, digital-twin, formal-verification, causal-inference, post-quantum).
- Landing-page hero heading moved from hyperbolic ("Nothing else in the space comes close") to descriptive ("The complete self-healing platform, in one binary").
- Landing-page footer trimmed from 13 placeholder links to 10 real destinations + enterprise email.

### Removed

- Old React + Vite + Tailwind + shadcn dashboard at `dashboard-ui/`. Replaced by the vanilla-JS embed at `internal/api/dashboard/static/`.
- 18 debug screenshot PNGs at repo root (iteration artifacts).
- Tracked local agent state (`.omc/`, `.playwright-mcp/`, `dashboard-ui/.omc/`, `demo/app.log`). Hardened `.gitignore` to prevent re-adding.

### Fixed

- Landing page install-command snippet no longer advertises a 404ing `go install` path.
- Live GitHub star count on landing page replaces previously hard-coded "1.2k".

### Tests

- **79 packages, 0 failures** on the full test matrix.

## [0.4.x] - earlier

Historical versions prior to the install-story fix. See `git log` for commit-level history.

[Unreleased]: https://github.com/Nagendhra-web/Immortal/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/Nagendhra-web/Immortal/releases/tag/v0.5.0
