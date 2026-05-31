#!/usr/bin/env bash
set -euo pipefail

gofmt -w .
go run golang.org/x/tools/cmd/goimports@latest -w .
