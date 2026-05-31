#!/usr/bin/env bash
set -euo pipefail

go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...
