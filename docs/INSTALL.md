# Installing Immortal

Pick the path that matches your setup. All paths land on the same binary.

## 1. One-line installer (recommended)

**macOS / Linux:**

```sh
curl -fsSL https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.sh | bash
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.ps1 | iex
```

The script tries a pre-built release binary first, falls back to `go install` if you have Go 1.25+ on `PATH`.

Env overrides:

- `IMMORTAL_VERSION=v0.5.0` pins a specific tag (default: `latest`).
- `IMMORTAL_INSTALL=/opt/bin` selects a custom install directory.

## 2. Go install

Requires Go 1.25+.

```sh
go install github.com/Nagendhra-web/Immortal/cmd/immortal@latest
```

To pin a version:

```sh
go install github.com/Nagendhra-web/Immortal/cmd/immortal@v0.5.0
```

## 3. Pre-built binaries

Download the archive for your platform from the [Releases page](https://github.com/Nagendhra-web/Immortal/releases/latest).

| Platform             | Archive                            |
| -------------------- | ---------------------------------- |
| Linux x86_64         | `immortal_linux_amd64.tar.gz`      |
| Linux arm64          | `immortal_linux_arm64.tar.gz`      |
| macOS Intel          | `immortal_darwin_amd64.tar.gz`     |
| macOS Apple Silicon  | `immortal_darwin_arm64.tar.gz`     |
| Windows x86_64       | `immortal_windows_amd64.zip`       |

Each archive contains one `immortal` binary plus `LICENSE`, `README.md`, and `PERFORMANCE.md`. Extract, put `immortal` on your `PATH`, done.

SHA-256 checksums are published as `checksums.txt` alongside the archives.

## 4. From source

```sh
git clone https://github.com/Nagendhra-web/Immortal
cd Immortal
make build
./bin/immortal start
```

Useful make targets:

- `make build` compiles to `./bin/immortal`.
- `make test` runs the full test suite.
- `make lint` runs `golangci-lint`.

## 5. Docker

```sh
docker build -t immortal:local .
docker run -p 7777:7777 immortal:local start --api-port 7777
```

A pre-built image at `ghcr.io/nagendhra-web/immortal` is planned for v0.6 (see issue #15).

## 6. Homebrew (planned for v0.6)

Homebrew support is on the v0.6 roadmap. The GoReleaser pipeline is already wired to publish a formula; it is gated on a `HOMEBREW_TAP_TOKEN` secret being added to the repository. Tracking: [issue #16](https://github.com/Nagendhra-web/Immortal/issues/16).

Once enabled it will be:

```sh
brew tap Nagendhra-web/immortal
brew install immortal
```

Until then, use the one-liner installer or `go install`.

---

## Verifying the install

```sh
immortal version
# immortal v0.5.0 · commit abc1234 · built 2026-04-19

immortal start --pqaudit --twin --agentic --causal --topology --formal
# open http://127.0.0.1:7777/           (landing)
# open http://127.0.0.1:7777/dashboard/  (operator console)
```

## Uninstalling

| Install method               | Uninstall                               |
| ---------------------------- | --------------------------------------- |
| `install.sh` / `install.ps1` | `rm $IMMORTAL_INSTALL/immortal`         |
| `go install`                 | `rm $(go env GOBIN)/immortal`           |
| Docker                       | `docker rmi immortal:local`             |
| Source                       | `rm -rf Immortal/`                      |

No state lives outside the `$IMMORTAL_DATA` directory (default `~/.immortal`). Delete that to fully remove traces.

## Troubleshooting

**`command not found: immortal`**. The install directory is not on your `PATH`. The installer prints a hint. On macOS/Linux add `export PATH="$PATH:$HOME/.local/bin"` to your shell rc.

**`go: no matching versions for query "latest"`**. You are running Go below 1.25 or the tag has not propagated yet. Update Go: https://go.dev/dl/. Or pin directly: `go install github.com/Nagendhra-web/Immortal/cmd/immortal@v0.5.0`.

**`module not found: github.com/Nagendhra-web/Immortal`**. Go is case-sensitive. The module path is exactly `github.com/Nagendhra-web/Immortal` (capital I, capital N) matching the GitHub repo.

**Installer falls back to `go install`**. You are on a platform/arch combination without a pre-built binary (for example Linux 386). Run the installer with `DEBUG=1` for verbose output, or build from source.
