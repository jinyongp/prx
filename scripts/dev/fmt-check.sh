#!/usr/bin/env bash
set -euo pipefail

files=()
while IFS= read -r file; do
  files+=("$file")
done < <(git ls-files '*.go' | grep -v '^internal/truststore/' || true)
if [ "${#files[@]}" -eq 0 ]; then
  exit 0
fi

unformatted="$(gofmt -l "${files[@]}")"
if [ -n "$unformatted" ]; then
  echo "unformatted: $unformatted"
  exit 1
fi
