#!/usr/bin/env bash
set -euo pipefail

: "${GITHUB_OUTPUT:?}"

tag=""
type="none"
target=""
release="not-applicable"
release_error=""
on_main="false"

if [ "${GITHUB_REF_TYPE:-}" = "tag" ]; then
  tag="${GITHUB_REF_NAME:-}"
else
  tag="$(git tag --points-at HEAD --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1 || true)"
fi

if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  tag=""
fi

if [ -n "$tag" ]; then
  if git cat-file -e "${tag}^{tag}" 2>/dev/null; then
    type="annotated"
    target="$(git rev-parse "${tag}^{commit}")"
  else
    type="lightweight"
    target="$(git rev-parse "$tag")"
  fi

  git fetch --force --quiet origin refs/heads/main:refs/remotes/origin/main
  if git merge-base --is-ancestor "$target" refs/remotes/origin/main; then
    on_main="true"
  fi

  release_error="$(mktemp)"
  if gh release view "$tag" >/dev/null 2>"$release_error"; then
    release="existing"
  elif grep -Eiq 'not found|could not resolve to a release' "$release_error"; then
    release="missing"
  else
    release="unknown"
  fi
  rm -f "$release_error"
fi

{
  echo "tag=${tag}"
  echo "type=${type}"
  echo "target=${target}"
  echo "release=${release}"
  echo "on_main=${on_main}"
} >> "$GITHUB_OUTPUT"
