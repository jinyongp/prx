# Command runner is `just` (install: https://github.com/casey/just).

[doc('list recipes')]
default:
  @just --list

[doc('build the binary')]
build:
  go build -o bin/prx ./cmd/prx

[doc('run tests with the race detector')]
test:
  go test -race ./...

[doc('tests + coverage')]
cover:
  go test -race -cover ./...

[doc('lint (human-readable)')]
lint:
  golangci-lint run ./...

[doc('lint for AI/scripts: human text -> stderr, JSON diagnostics -> stdout')]
lint-json:
  golangci-lint run ./... --output.text.path=stderr --output.text.colors=false --output.json.path=stdout

[doc('vulnerability scan (narrowed to actually-called code)')]
vuln:
  govulncheck ./...

[doc('format with gofmt + goimports')]
fmt:
  gofmt -w .
  goimports -w .

[doc('full gate — run before opening a PR')]
check: test lint vuln
