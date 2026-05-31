#!/usr/bin/env sh
set -eu

VERSION="${PRX_VERSION:-latest}"
SKIP_SKILL_INSTALL="${SKIP_SKILL_INSTALL:-false}"
REPO="jinyongp/prx"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH_RAW="$(uname -m)"

case "$OS" in
  darwin|linux) ;;
  *)
    echo "Unsupported OS: $OS" >&2
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
    echo "Unsupported architecture: $ARCH_RAW" >&2
    echo "supported architecture: amd64, arm64" >&2
    exit 1
    ;;
esac

if ! command -v curl >/dev/null 2>&1; then
  echo "Error: curl is required for installation." >&2
  exit 1
fi

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

BINARY_NAME="prx-${OS}-${ARCH}"
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
  if ! curl -fsSL -H "Accept: application/vnd.github+json" -H "User-Agent: prx-install" "$API_URL" > "$RELEASE_JSON"; then
    echo "Failed to read release metadata from GitHub." >&2
    return 1
  fi

  ASSET_URLS="$(sed -n 's/.*\"browser_download_url\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p' "$RELEASE_JSON")"

  if [ -z "$ASSET_URLS" ]; then
    return 1
  fi

  CHECKSUMS_URL="$(printf '%s\n' "$ASSET_URLS" | grep '/checksums.txt$' | head -n 1 || true)"

  for attempt in "$BINARY_NAME" "${BINARY_NAME}.tar.gz" "${BINARY_NAME}.zip"; do
    CANDIDATE="$(printf '%s\n' "$ASSET_URLS" | grep "/${attempt}$" | head -n 1 || true)"
    if [ -n "$CANDIDATE" ]; then
      DOWNLOAD_URL="$CANDIDATE"
      return 0
    fi
  done

  return 1
}

build_from_source() {
  if ! command -v git >/dev/null 2>&1; then
    echo "No prebuilt release was found and 'git' is missing." >&2
    echo "Install git, or publish a release with artifacts for ${BINARY_NAME}." >&2
    return 1
  fi

  if ! command -v go >/dev/null 2>&1; then
    echo "No prebuilt release was found and 'go' is missing." >&2
    echo "Install Go, or publish a release with artifacts for ${BINARY_NAME}." >&2
    return 1
  fi

  SOURCE_DIR="${TMP_DIR}/source"
  CLONE_URL="https://github.com/${REPO}.git"

  if [ "$VERSION" = "latest" ]; then
    if ! git clone --quiet --depth 1 "$CLONE_URL" "$SOURCE_DIR" >/dev/null 2>&1; then
      echo "Failed to clone ${REPO} (latest)" >&2
      return 1
    fi
  else
    if ! git clone --quiet --depth 1 --branch "$VERSION" "$CLONE_URL" "$SOURCE_DIR" >/dev/null 2>&1; then
      if ! git clone --quiet --depth 1 "$CLONE_URL" "$SOURCE_DIR" >/dev/null 2>&1; then
        echo "Failed to clone ${REPO} for tag ${VERSION}" >&2
        return 1
      fi
      if ! (cd "$SOURCE_DIR" && git checkout "$VERSION" >/dev/null 2>&1); then
        echo "Release version ${VERSION} not found (tag or branch)." >&2
        return 1
      fi
    fi
  fi

  if ! (cd "$SOURCE_DIR" && go build -trimpath -ldflags "-s -w" -o "$BINARY_PATH" ./cmd/prx); then
    echo "Failed to build prx from source." >&2
    return 1
  fi

  return 0
}

verify_checksum() {
  if [ -z "${CHECKSUMS_URL:-}" ]; then
    echo "Warning: release has no checksums.txt; skipping integrity check." >&2
    return 0
  fi

  CHECKSUMS_FILE="${TMP_DIR}/checksums.txt"
  if ! curl -fsSL "$CHECKSUMS_URL" -o "$CHECKSUMS_FILE"; then
    echo "Warning: failed to download checksums.txt; skipping integrity check." >&2
    return 0
  fi

  asset_name="$(basename "$DOWNLOAD_URL")"
  expected="$(awk -v f="$asset_name" '$2 == f || $2 == "*"f {print $1; exit}' "$CHECKSUMS_FILE")"
  if [ -z "$expected" ]; then
    echo "Warning: no checksum entry for ${asset_name}; skipping integrity check." >&2
    return 0
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$BINARY_PATH" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$BINARY_PATH" | awk '{print $1}')"
  else
    echo "Warning: no sha256 tool found (sha256sum/shasum); skipping integrity check." >&2
    return 0
  fi

  if [ "$actual" != "$expected" ]; then
    echo "Checksum verification failed for ${asset_name}." >&2
    echo "  expected: ${expected}" >&2
    echo "  actual:   ${actual}" >&2
    return 1
  fi

  echo "Verified checksum for ${asset_name}."
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
  echo "No installable binary found." >&2
  exit 1
fi
chmod +x "$BINARY_PATH"

if [ -n "${PRX_BIN_DIR:-}" ]; then
  if ! mkdir -p "${PRX_BIN_DIR}" 2>/dev/null || [ ! -w "${PRX_BIN_DIR}" ]; then
    echo "Error: PRX_BIN_DIR is set but not writable: ${PRX_BIN_DIR}" >&2
    exit 1
  fi
  DEST_DIR="${PRX_BIN_DIR}"
elif [ -w /usr/local/bin ]; then
  DEST_DIR="/usr/local/bin"
elif [ -w "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin"; then
  DEST_DIR="$HOME/.local/bin"
else
  echo "Error: no writable install directory found." >&2
  echo "Grant permissions or use a custom destination in your shell manually." >&2
  exit 1
fi

DEST="${DEST_DIR}/prx"

if command -v install >/dev/null 2>&1; then
  install -m 755 "$BINARY_PATH" "$DEST"
else
  cp "$BINARY_PATH" "$DEST"
  chmod 755 "$DEST"
fi

echo "Installed prx to ${DEST}"

if [ "$SKIP_SKILL_INSTALL" = "1" ] || [ "$SKIP_SKILL_INSTALL" = "true" ]; then
  echo "Skipped skill installation (SKIP_SKILL_INSTALL=${SKIP_SKILL_INSTALL})."
  exit 0
fi

echo "Installing agent skill..."

if command -v npx >/dev/null 2>&1; then
  if npx -y skills add jinyongp/prx; then
    echo "Installed prx skill using: npx skills add jinyongp/prx"
    exit 0
  fi
  echo "npx skills add failed." >&2
fi

if command -v apm >/dev/null 2>&1; then
  if apm install jinyongp/prx; then
    echo "Installed prx skill using: apm install jinyongp/prx"
    exit 0
  fi
  echo "apm install failed." >&2
fi

echo "Skill installation failed." >&2
echo "Install a skill manager (npx or apm) and run one of:" >&2
echo "  npx skills add jinyongp/prx" >&2
echo "  apm install jinyongp/prx" >&2
echo "Or skip with SKIP_SKILL_INSTALL=true." >&2
exit 1
