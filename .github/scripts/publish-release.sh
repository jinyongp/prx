#!/usr/bin/env bash
set -euo pipefail

tag="${1:?Usage: publish-release.sh vX.Y.Z}"

# Use the annotated tag message as the release notes. subject+body excludes any
# GPG signature block; fall back to a bare line for lightweight tags.
notes="$(git tag -l --format='%(contents:subject)%0a%0a%(contents:body)' "$tag")"
[ -n "$(echo "$notes" | tr -d '[:space:]')" ] || notes="Release $tag"

if gh release view "$tag" >/dev/null 2>&1; then
  gh release upload "$tag" gate-* checksums.txt --clobber
else
  gh release create "$tag" \
    gate-darwin-amd64 gate-darwin-arm64 gate-linux-amd64 gate-linux-arm64 checksums.txt \
    --title "$tag" \
    --notes "$notes" \
    --verify-tag
fi
