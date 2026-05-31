# Command runner is `just` (install: https://github.com/casey/just).

[doc('list recipes')]
default:
  @just --list

[doc('build the binary')]
build:
  go build -trimpath -ldflags "-s -w" -o bin/prx ./cmd/prx

[doc('run tests with the race detector')]
test:
  go test -race ./...

[doc('tests + coverage')]
cover:
  go test -race -cover ./...

[doc('lint (human-readable)')]
lint:
  go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...

[doc('lint for AI/scripts: human text -> stderr, JSON diagnostics -> stdout')]
lint-json:
  go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./... --output.text.path=stderr --output.text.colors=false --output.json.path=stdout

[doc('vulnerability scan (narrowed to actually-called code)')]
vuln:
  govulncheck ./...

[doc('format with gofmt + goimports')]
fmt:
  gofmt -w .
  goimports -w .

[doc('full gate — run before opening a PR')]
check: test lint vuln

[doc('push main and publish patch/minor/major tags to latest main commit')]
release-publish tag="":
  #!/usr/bin/env bash
  set -euo pipefail
  TAG_INPUT="{{tag}}"

  if [ -z "$TAG_INPUT" ]; then
    echo "Usage: just release-publish tag=<vX.Y.Z>"
    exit 1
  fi
  if [[ ! "$TAG_INPUT" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Patch tag must be semver form: vX.Y.Z (for example: v1.2.3)"
    exit 1
  fi

  if [ "$(git symbolic-ref --short HEAD)" != "main" ]; then
    echo "release-publish must run on branch 'main'."
    exit 1
  fi

  if [ -n "$(git status --porcelain)" ]; then
    echo "Working tree is dirty. Commit or stash changes first."
    exit 1
  fi

  git push origin main

  VERSION="${TAG_INPUT#v}"
  MAJOR="${VERSION%%.*}"
  REST="${VERSION#*.}"
  MINOR="${REST%%.*}"
  PATCH="${VERSION}"
  MINOR_TAG="v${MAJOR}.${MINOR}"
  MAJOR_TAG="v${MAJOR}"
  PATCH_TAG="v${PATCH}"
  TARGET_SHA="$(git rev-parse origin/main)"

  if git rev-parse -q --verify "refs/tags/$PATCH_TAG" >/dev/null; then
    echo "Patch tag already exists: $PATCH_TAG"
    exit 1
  fi
  if git ls-remote --exit-code --tags origin "refs/tags/$PATCH_TAG" >/dev/null 2>&1; then
    echo "Remote patch tag already exists: $PATCH_TAG"
    exit 1
  fi

  git tag -a "$PATCH_TAG" -m "Release $PATCH_TAG" "$TARGET_SHA"
  for alias_tag in "$MINOR_TAG" "$MAJOR_TAG"; do
    if git rev-parse -q --verify "refs/tags/$alias_tag" >/dev/null; then
      git tag -d "$alias_tag"
    fi
    git tag -a "$alias_tag" -m "Release $alias_tag" "$TARGET_SHA"
  done

  git push origin "$PATCH_TAG" "$MINOR_TAG" "$MAJOR_TAG" --force

[doc('cross-compile all release targets into bin/')]
build-all version="dev":
  #!/usr/bin/env bash
  set -euo pipefail
  for t in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
    os="${t%/*}"; arch="${t#*/}"
    GOOS="$os" GOARCH="$arch" go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o "bin/prx-$os-$arch" ./cmd/prx
    echo "built bin/prx-$os-$arch"
  done
