#!/usr/bin/env bash
set -euo pipefail

scripts/dev/golangci-lint.sh text "$@"
