# Contributing to Immortal

Thanks for your interest in contributing! This guide will help you get started.

## Quick Start

```bash
git clone https://github.com/Nagendhra-web/Immortal.git
cd Immortal
go test ./...
go build -o immortal ./cmd/immortal
./immortal start --ghost
```

## Project Structure

```
cmd/immortal/        CLI entrypoint
internal/            58 packages (each works independently)
  engine/            Core healing orchestrator
  dna/               Anomaly detection
  causality/         Root cause analysis
  predict/           Predictive healing
  pattern/           Recurring failure detection
  sla/               SLA tracking
  audit/             Immutable audit log
  dependency/        Service dependency graph
  webhook/           HMAC-signed notifications
  healing/           Healing rules and execution
  security/          WAF, RASP, rate limiter, anti-scrape, secrets, zero-trust
  api/rest/          REST API (21 endpoints)
  cli/               CLI (16 commands)
  ... and 30+ more
sdk/
  go/                Go SDK
  typescript/        TypeScript SDK
  python/            Python SDK
demo/                Integration test scenarios
```

## How to Contribute

### Bug Reports

Open an issue with the **Bug Report** template. Include:
- What happened vs what you expected
- Steps to reproduce
- Go version and OS

### Feature Requests

Open an issue with the **Feature Request** template. Describe:
- The problem you're solving
- Your proposed solution
- Alternatives considered

### Pull Requests

1. **Fork** the repo and create a branch: `git checkout -b feat/your-feature`
2. **Write tests first** — every new function needs a test
3. **Run all tests**: `go test ./...` — all 59 suites must pass
4. **Keep PRs small** — one feature per PR, under 500 lines preferred
5. **Write a clear description** — explain what and why
6. Submit the PR

### Code Standards

- Follow Go conventions (`gofmt`, `go vet`)
- Every package must be **thread-safe** (`sync.RWMutex`)
- Every public function needs a **test**
- Packages should work **standalone** — minimal cross-dependencies
- No external dependencies without discussion
- JSON struct tags on all exported types

### Testing

```bash
# All tests
go test ./...

# Specific package with verbose output
go test -v ./internal/engine/

# With race detection
go test -race ./...

# Demo scenarios
go test -v ./demo/

# TypeScript SDK
cd sdk/typescript && npm test

# Python SDK
cd sdk/python && pytest
```

## Good First Issues

Look for issues labeled [`good first issue`](https://github.com/Nagendhra-web/Immortal/labels/good%20first%20issue) — these are chosen for new contributors.

## Architecture Decisions

- **Single binary** — no external dependencies (embedded SQLite)
- **Plugin-first** — new features as packages, not monolith changes
- **Offline-first** — works without internet, LLM is optional
- **Test-first** — tests before implementation
- **Honest** — only claim features that are tested and working

## Code of Conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Be respectful, constructive, and kind.

## License

By contributing, you agree that your contributions will be licensed under [Apache 2.0](LICENSE).
