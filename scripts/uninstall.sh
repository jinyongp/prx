#!/usr/bin/env sh
set -eu

FOUND=0

TARGET_DIRS="
/usr/local/bin
$HOME/.local/bin
"

if [ -n "${PRX_BIN_DIR:-}" ]; then
  TARGET_DIRS="${TARGET_DIRS}
${PRX_BIN_DIR%/}
"
fi

for DIR in $TARGET_DIRS; do
  BIN="${DIR}/prx"
  if [ -f "$BIN" ] || [ -L "$BIN" ]; then
    rm -f "$BIN"
    echo "Removed: $BIN"
    FOUND=1
  fi
done

if [ "$FOUND" -eq 0 ]; then
  echo "No prx binary found in install locations."
  exit 0
fi

if command -v rehash >/dev/null 2>&1; then
  rehash
fi

if command -v hash >/dev/null 2>&1; then
  hash -r
fi

echo "prx uninstalled."
