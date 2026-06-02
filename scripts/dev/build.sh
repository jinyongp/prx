#!/usr/bin/env bash
set -euo pipefail

version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
mkdir -p bin
go build -trimpath -ldflags "-s -w -X main.version=${version}" -o bin/gate ./cmd/gate
