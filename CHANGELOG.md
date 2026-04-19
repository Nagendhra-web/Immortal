# Changelog

All notable changes to Immortal are documented here.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Immortal follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
