#!/usr/bin/env sh
set -eu

VERSION="${1:-dev}"
LD_FLAGS="-s -w -X main.version=${VERSION}"

GOOS_LIST="darwin linux"
GOARCH_LIST="arm64 amd64"

for target_os in $GOOS_LIST; do
  for target_arch in $GOARCH_LIST; do
    GOOS="$target_os" GOARCH="$target_arch" go build \
      -trimpath \
      -ldflags "$LD_FLAGS" \
      -o "prx-${target_os}-${target_arch}" \
      ./cmd/prx
  done
done
