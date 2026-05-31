# Command runner is `just` (install: https://github.com/casey/just).
set quiet

export GOCACHE := "/tmp/prx-gocache"
export GOLANGCI_LINT_CACHE := "/tmp/prx-golangci-cache"

[private]
default:
  @just --list

[doc('build the binary')]
build:
  scripts/dev/build.sh

[doc('run prx from source, e.g. `just prx ls`, `just prx --help`')]
prx *args:
  scripts/dev/run-prx.sh {{args}}

[doc('run tests with the race detector')]
test:
  scripts/dev/test.sh

[doc('tests + coverage')]
cover:
  scripts/dev/cover.sh

[doc('check gofmt without writing files')]
fmt-check:
  scripts/dev/fmt-check.sh

[doc('go vet all packages')]
vet:
  scripts/dev/vet.sh

[doc('lint (human-readable)')]
lint:
  scripts/dev/lint.sh

[doc('lint for AI/scripts: human text -> stderr, JSON diagnostics -> stdout')]
lint-json:
  scripts/dev/lint-json.sh

[doc('vulnerability scan (narrowed to actually-called code)')]
vuln:
  scripts/dev/vuln.sh

[doc('shell script syntax/lint smoke checks')]
scripts-check:
  bash scripts/dev/check-scripts.sh

[doc('format with gofmt + goimports')]
fmt:
  scripts/dev/fmt.sh

[doc('full gate — run before opening a PR')]
check:
  scripts/dev/check.sh

[doc('release a new version: no arg => interactive patch/minor/major; patch/minor/major -> bump from latest tag; explicit vX.Y.Z')]
release tag="":
  ./scripts/release/publish.sh "{{tag}}"

[doc('cross-compile all release targets into bin/')]
build-all version="dev":
  scripts/release/build-prx.sh "{{version}}" bin
