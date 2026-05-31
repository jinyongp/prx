#!/usr/bin/env bash
set -euo pipefail

sh -n scripts/install.sh scripts/uninstall.sh scripts/lib/*.sh scripts/release/build-prx.sh
bash -n scripts/dev/*.sh scripts/release/*.sh

if command -v shellcheck >/dev/null 2>&1; then
  shellcheck -S warning scripts/*.sh scripts/dev/*.sh scripts/lib/*.sh scripts/release/*.sh
fi

if command -v shfmt >/dev/null 2>&1; then
  shfmt -d scripts/*.sh scripts/dev/*.sh scripts/lib/*.sh scripts/release/*.sh
fi
