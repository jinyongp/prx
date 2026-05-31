# Command runner is `just` (install: https://github.com/casey/just).
set quiet

export GOCACHE := "/tmp/prx-gocache"
export GOLANGCI_LINT_CACHE := "/tmp/prx-golangci-cache"

[private]
default:
  @just --list

[doc('build the binary')]
build:
  go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o bin/prx ./cmd/prx

[doc('run prx from source, e.g. `just prx ls`, `just prx --help`')]
prx *args:
  go run ./cmd/prx {{args}}

[doc('run tests with the race detector')]
test:
  bash scripts/dev/go-test-format.sh

[doc('tests + coverage')]
cover:
  bash scripts/dev/go-test-format.sh -cover

[doc('check gofmt without writing files')]
fmt-check:
  #!/usr/bin/env bash
  set -euo pipefail
  unformatted="$(gofmt -l $(git ls-files '*.go' | grep -v '^internal/truststore/'))"
  test -z "$unformatted" || { echo "unformatted: $unformatted"; exit 1; }

[doc('go vet all packages')]
vet:
  go vet ./...

[doc('lint (human-readable)')]
lint:
  go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...

[doc('lint for AI/scripts: human text -> stderr, JSON diagnostics -> stdout')]
lint-json:
  go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./... --output.text.path=stderr --output.text.colors=false --output.json.path=stdout

[doc('vulnerability scan (narrowed to actually-called code)')]
vuln:
  go run golang.org/x/vuln/cmd/govulncheck@v1.3.0 ./...

[doc('shell script syntax/lint smoke checks')]
scripts-check:
  bash scripts/dev/check-scripts.sh

[doc('format with gofmt + goimports')]
fmt:
  gofmt -w .
  go run golang.org/x/tools/cmd/goimports@latest -w .

[doc('full gate — run before opening a PR')]
check: fmt-check vet cover lint vuln scripts-check

[doc('release a new version: no arg => interactive patch/minor/major; patch/minor/major -> bump from latest tag; explicit vX.Y.Z')]
release tag="":
  ./scripts/release/publish.sh "{{tag}}"

[doc('cross-compile all release targets into bin/')]
build-all version="dev":
  scripts/release/build-prx.sh "{{version}}" bin
