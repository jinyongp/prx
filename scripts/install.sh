#!/usr/bin/env sh
set -eu

VERSION="${GATE_VERSION:-latest}"
REPO="jinyongp/gate"

SCRIPT_DIR="$(CDPATH="" cd "$(dirname "$0")" 2>/dev/null && pwd || pwd)"
if [ -r "${SCRIPT_DIR}/lib/ui.sh" ]; then
  . "${SCRIPT_DIR}/lib/ui.sh"
else
  ui_section() { printf '\n%s\n' "$1"; }
  ui_kv() { printf '  %-12s %s\n' "$1" "$2"; }
  ui_ok() { printf 'ok: %s\n' "$1"; }
  ui_warn_err() { printf 'warning: %s\n' "$1" >&2; }
  ui_error() { printf 'error: %s\n' "$1" >&2; }
fi

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH_RAW="$(uname -m)"

case "$OS" in
  darwin|linux) ;;
  *)
    ui_error "unsupported OS: $OS"
    echo "supported OS: darwin, linux" >&2
    exit 1
    ;;
esac

case "$ARCH_RAW" in
  x86_64|amd64)
    ARCH="amd64" ;;
  arm64|aarch64)
    ARCH="arm64" ;;
  *)
    ui_error "unsupported architecture: $ARCH_RAW"
    echo "supported architecture: amd64, arm64" >&2
    exit 1
    ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  ui_error "curl is required for installation."
  exit 1
fi

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

BINARY_NAME="gate-${OS}-${ARCH}"
BINARY_PATH="${TMP_DIR}/${BINARY_NAME}"
DOWNLOAD_URL=""
CHECKSUMS_URL=""

resolve_download_url() {
  if [ "$VERSION" = "latest" ]; then
    API_URL="https://api.github.com/repos/${REPO}/releases/latest"
  else
    API_URL="https://api.github.com/repos/${REPO}/releases/tags/${VERSION}"
  fi

  RELEASE_JSON="${TMP_DIR}/release.json"
  if ! curl -fsSL -H "Accept: application/vnd.github+json" -H "User-Agent: gate-install" "$API_URL" > "$RELEASE_JSON"; then
    ui_error "failed to read release metadata from GitHub."
    return 1
  fi

  ASSET_URLS="$(sed -n 's/.*\"browser_download_url\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p' "$RELEASE_JSON")"

  if [ -z "$ASSET_URLS" ]; then
    return 1
  fi

  CHECKSUMS_URL="$(printf '%s\n' "$ASSET_URLS" | grep '/checksums.txt$' | head -n 1 || true)"

  CANDIDATE="$(printf '%s\n' "$ASSET_URLS" | grep "/${BINARY_NAME}$" | head -n 1 || true)"
  if [ -n "$CANDIDATE" ]; then
    DOWNLOAD_URL="$CANDIDATE"
    return 0
  fi

  return 1
}

build_from_source() {
  if ! command -v git >/dev/null 2>&1; then
    ui_error "no prebuilt release was found and 'git' is missing."
    echo "Install git, or publish a release with artifacts for ${BINARY_NAME}." >&2
    return 1
  fi

  if ! command -v go >/dev/null 2>&1; then
    ui_error "no prebuilt release was found and 'go' is missing."
    echo "Install Go, or publish a release with artifacts for ${BINARY_NAME}." >&2
    return 1
  fi

  SOURCE_DIR="${TMP_DIR}/source"
  CLONE_URL="https://github.com/${REPO}.git"

  if [ "$VERSION" = "latest" ]; then
    if ! git clone --quiet --depth 1 "$CLONE_URL" "$SOURCE_DIR" >/dev/null 2>&1; then
      ui_error "failed to clone ${REPO} (latest)"
      return 1
    fi
  else
    if ! git clone --quiet --depth 1 --branch "$VERSION" "$CLONE_URL" "$SOURCE_DIR" >/dev/null 2>&1; then
      if ! git clone --quiet --depth 1 "$CLONE_URL" "$SOURCE_DIR" >/dev/null 2>&1; then
        ui_error "failed to clone ${REPO} for tag ${VERSION}"
        return 1
      fi
      if ! (cd "$SOURCE_DIR" && git checkout "$VERSION" >/dev/null 2>&1); then
        ui_error "release version ${VERSION} not found (tag or branch)."
        return 1
      fi
    fi
  fi

  build_version="$VERSION"
  if [ "$build_version" = "latest" ]; then
    build_version="$(cd "$SOURCE_DIR" && git describe --tags --always 2>/dev/null || echo dev)"
  fi

  if ! (cd "$SOURCE_DIR" && go build -trimpath -ldflags "-s -w -X main.version=${build_version}" -o "$BINARY_PATH" ./cmd/gate); then
    ui_error "failed to build gate from source."
    return 1
  fi

  return 0
}

verify_checksum() {
  if [ -z "${CHECKSUMS_URL:-}" ]; then
    ui_warn_err "release has no checksums.txt; skipping integrity check."
    return 0
  fi

  CHECKSUMS_FILE="${TMP_DIR}/checksums.txt"
  if ! curl -fsSL "$CHECKSUMS_URL" -o "$CHECKSUMS_FILE"; then
    ui_error "failed to download checksums.txt; refusing to install unverified binary."
    return 1
  fi

  asset_name="$(basename "$DOWNLOAD_URL")"
  expected="$(awk -v f="$asset_name" '$2 == f || $2 == "*"f {print $1; exit}' "$CHECKSUMS_FILE")"
  if [ -z "$expected" ]; then
    ui_error "no checksum entry for ${asset_name}; refusing to install unverified binary."
    return 1
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$BINARY_PATH" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$BINARY_PATH" | awk '{print $1}')"
  else
    ui_error "no sha256 tool found (sha256sum/shasum); refusing to install unverified binary."
    return 1
  fi

  if [ "$actual" != "$expected" ]; then
    ui_error "checksum verification failed for ${asset_name}."
    echo "  expected: ${expected}" >&2
    echo "  actual:   ${actual}" >&2
    return 1
  fi

  ui_ok "verified checksum for ${asset_name}."
  return 0
}

if resolve_download_url; then
  if curl -fsSL "$DOWNLOAD_URL" -o "$BINARY_PATH"; then
    verify_checksum
  else
    build_from_source
  fi
else
  build_from_source
fi

if [ ! -f "$BINARY_PATH" ]; then
  ui_error "no installable binary found."
  exit 1
fi
chmod +x "$BINARY_PATH"

if [ -n "${GATE_BIN_DIR:-}" ]; then
  if ! mkdir -p "${GATE_BIN_DIR}" 2>/dev/null || [ ! -w "${GATE_BIN_DIR}" ]; then
    ui_error "GATE_BIN_DIR is set but not writable: ${GATE_BIN_DIR}"
    exit 1
  fi
  DEST_DIR="${GATE_BIN_DIR}"
elif [ -w "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin"; then
  DEST_DIR="$HOME/.local/bin"
else
  ui_error "no writable install directory found."
  echo "Grant permissions or use a custom destination in your shell manually." >&2
  exit 1
fi

DEST="${DEST_DIR}/gate"

if command -v install >/dev/null 2>&1; then
  install -m 755 "$BINARY_PATH" "$DEST"
else
  cp "$BINARY_PATH" "$DEST"
  chmod 755 "$DEST"
fi

ui_section "Install complete"
ui_kv "Binary" "$DEST"

case ":${PATH}:" in
  *":${DEST_DIR}:"*)
    ui_ok "gate is already in your current PATH."
    ;;
  *)
    ui_warn_err "PATH does not currently include ${DEST_DIR}."
    echo "Run one of the following in your shell to use gate:"
    echo "  export PATH=\"${DEST_DIR}:\$PATH\""
    ;;
esac

resolved="$(command -v gate 2>/dev/null || true)"
if [ -n "$resolved" ] && [ "$resolved" != "$DEST" ]; then
  ui_warn_err "another gate is earlier in PATH and will shadow this install:"
  echo "  ${resolved}"
  echo "Remove it, or reorder PATH so ${DEST_DIR} comes first."
fi
