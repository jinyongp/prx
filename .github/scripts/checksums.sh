#!/usr/bin/env bash
set -euo pipefail

sha256sum gate-darwin-amd64 gate-darwin-arm64 gate-linux-amd64 gate-linux-arm64 > checksums.txt
