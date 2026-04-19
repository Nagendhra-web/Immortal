#!/usr/bin/env bash
#
# Immortal — one-line installer
#
#   curl -fsSL https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/scripts/install.sh | bash
#
# Tries, in order:
#   1. Download a pre-built release binary from GitHub Releases (fastest).
#   2. Fall back to `go install` (requires Go 1.25+ on $PATH).
#
# Env overrides:
#   IMMORTAL_VERSION   pin a specific release tag (default: latest)
#   IMMORTAL_INSTALL   target directory (default: $HOME/.local/bin, or /usr/local/bin on mac)

set -euo pipefail

REPO="Nagendhra-web/Immortal"
VERSION="${IMMORTAL_VERSION:-latest}"
INSTALL_DIR="${IMMORTAL_INSTALL:-}"

# ── Detect install dir ─────────────────────────────────────────────────────────
if [[ -z "$INSTALL_DIR" ]]; then
  case "$(uname -s)" in
    Darwin*) INSTALL_DIR="/usr/local/bin" ;;
    Linux*)  INSTALL_DIR="$HOME/.local/bin" ;;
    *)       INSTALL_DIR="$HOME/.local/bin" ;;
  esac
fi
mkdir -p "$INSTALL_DIR"

# ── Detect platform ────────────────────────────────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "unsupported arch: $ARCH" >&2; exit 1 ;;
esac

# ── Path 1: download pre-built release ─────────────────────────────────────────
try_release() {
  if ! command -v curl >/dev/null 2>&1; then
    echo "curl not found — skipping release download"
    return 1
  fi

  local url
  if [[ "$VERSION" == "latest" ]]; then
    url="https://github.com/${REPO}/releases/latest/download/immortal_${OS}_${ARCH}.tar.gz"
  else
    url="https://github.com/${REPO}/releases/download/${VERSION}/immortal_${OS}_${ARCH}.tar.gz"
  fi

  echo "→ Fetching $url"
  local tmpdir
  tmpdir="$(mktemp -d)"
  trap "rm -rf '$tmpdir'" EXIT

  if ! curl -fsSL "$url" -o "$tmpdir/immortal.tar.gz"; then
    echo "  no release binary available at $url"
    return 1
  fi
  tar -xzf "$tmpdir/immortal.tar.gz" -C "$tmpdir"
  install -m 0755 "$tmpdir/immortal" "$INSTALL_DIR/immortal"
  echo "✓ installed $("$INSTALL_DIR/immortal" version 2>/dev/null | head -1) → $INSTALL_DIR/immortal"
  return 0
}

# ── Path 2: build from source via `go install` ─────────────────────────────────
try_go_install() {
  if ! command -v go >/dev/null 2>&1; then
    echo "go not found — cannot fall back to source build"
    return 1
  fi

  local tag="@latest"
  [[ "$VERSION" != "latest" ]] && tag="@$VERSION"

  echo "→ Building via: go install github.com/${REPO}/cmd/immortal$tag"
  GOBIN="$INSTALL_DIR" go install "github.com/${REPO}/cmd/immortal${tag}"
  echo "✓ installed $("$INSTALL_DIR/immortal" version 2>/dev/null | head -1) → $INSTALL_DIR/immortal"
  return 0
}

# ── Main ───────────────────────────────────────────────────────────────────────
echo "Installing Immortal ($VERSION) for $OS/$ARCH"

if try_release; then :
elif try_go_install; then :
else
  echo "Neither a release binary nor a local Go toolchain is available." >&2
  echo "Install Go (https://go.dev/dl/) and rerun, or clone + make build." >&2
  exit 1
fi

# ── PATH hint ──────────────────────────────────────────────────────────────────
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo ""
    echo "⚠  $INSTALL_DIR is not on your PATH."
    echo "   Add this to your shell rc:"
    echo "     export PATH=\"\$PATH:$INSTALL_DIR\""
    ;;
esac

echo ""
echo "Get started:"
echo "  immortal start --pqaudit --twin --agentic --causal --topology --formal"
