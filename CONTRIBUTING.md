# Contributing to Immortal

Thank you for your interest in contributing to Immortal! This guide will help you get started.

## Quick Start

```bash
# Clone the repo
git clone https://github.com/immortal-engine/immortal.git
cd immortal

# Install Go 1.22+
# https://go.dev/dl/

# Run tests
go test ./...

# Build
go build -o bin/immortal ./cmd/immortal

# Run
./bin/immortal start --ghost
```

## Project Structure

```
immortal/
├── cmd/immortal/          # CLI entry point
├── internal/              # Core packages (48 packages)
│   ├── engine/            # Core engine (wires everything)
│   ├── event/             # Universal event format
│   ├── healing/           # Rule matching + actions
│   ├── dna/               # Health fingerprinting
│   ├── brain/             # Predictive trend detection
│   ├── causality/         # Root cause analysis
│   ├── security/          # WAF, rate limit, RASP, etc.
│   ├── cluster/           # Multi-node coordination
│   ├── learning/          # Persistent pattern learning
│   ├── llm/               # Optional LLM integration
│   └── ...                # 38 more packages
├── sdk/                   # Language SDKs
│   ├── go/                # Go SDK
│   ├── typescript/        # TypeScript SDK
│   └── python/            # Python SDK
├── demo/                  # Real-world demo tests
└── docs/                  # Design specs and plans
```

## How to Contribute

### Bug Reports
Open an issue with:
- What happened
- What you expected
- Steps to reproduce
- Go version and OS

### Feature Requests
Open an issue describing:
- The problem you're solving
- Your proposed solution
- Why it matters

### Pull Requests

1. Fork the repo
2. Create a branch: `git checkout -b feat/your-feature`
3. Write tests first (TDD)
4. Implement the feature
5. Run all tests: `go test ./...`
6. Submit a PR

### Code Standards

- Follow Go conventions (`gofmt`, `golint`)
- Every package must have tests
- Every public function must have a comment
- No external dependencies without discussion
- Thread-safety required for all shared state

### Testing

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test -v ./internal/engine/...

# Run demo scenarios
go test -v ./demo/...

# Run benchmarks
go test -bench=. ./internal/...
```

## Architecture Decisions

- **Single binary**: No external dependencies (embedded SQLite)
- **Plugin-first**: New features as packages, not monolith changes
- **Offline-first**: Works without internet, LLM is optional
- **Test-first**: Tests before implementation, 1:1 ratio

## Code of Conduct

Be respectful. Be constructive. Be kind. We're building something together.

## License

Apache 2.0 - see [LICENSE](LICENSE)
