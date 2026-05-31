#!/usr/bin/env bash
set -euo pipefail

TAG_INPUT="${1:-}"
TAG_INPUT="${TAG_INPUT#tag=}"

get_latest_tag() {
  git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1
}

semver_from_tag() {
  local tag="$1"
  local stripped="${tag#v}"

  printf "%s\n" "$stripped" | awk -F. '{print $1" "$2" "$3}'
}

next_version() {
  local major="$1"
  local minor="$2"
  local patch="$3"
  local bump="$4"

  case "$bump" in
    major)
      major=$((major + 1))
      minor=0
      patch=0
      ;;
    minor)
      minor=$((minor + 1))
      patch=0
      ;;
    patch)
      patch=$((patch + 1))
      ;;
    *)
      echo "Unknown bump type: $bump" >&2
      exit 1
      ;;
  esac

  echo "v${major}.${minor}.${patch}"
}

if [ -z "$TAG_INPUT" ]; then
  echo "Usage: scripts/release-publish.sh <patch|minor|major|vX.Y.Z>"
  exit 1
fi

case "$TAG_INPUT" in
  patch|minor|major)
    LATEST_TAG="$(get_latest_tag)"

    if [ -z "$LATEST_TAG" ]; then
      LATEST_TAG="v0.0.0"
    fi

    read -r MAJOR MINOR PATCH <<<"$(semver_from_tag "$LATEST_TAG")"
    PATCH_TAG="$(next_version "$MAJOR" "$MINOR" "$PATCH" "$TAG_INPUT")"
    ;;
  *)
    if [[ ! "$TAG_INPUT" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo "Tag must be vX.Y.Z or one of: patch, minor, major"
      exit 1
    fi

    PATCH_TAG="$TAG_INPUT"
    ;;
esac

if [ "${TAG_INPUT}" != "${PATCH_TAG}" ]; then
  echo "Resolved tag: ${PATCH_TAG} (from ${TAG_INPUT} bump)"
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
git push origin "$PATCH_TAG"
