#!/usr/bin/env bash
set -euo pipefail

DRY_RUN=0
AUTO_PUSH=0
TAG_INPUT=""

SCRIPT_DIR="$(CDPATH="" cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/../lib/ui.sh"

abort_interrupted() {
  printf '\n'
  echo "Aborted."
  exit 130
}

trap abort_interrupted INT

for arg in "$@"; do
  if [ -z "$arg" ]; then
    continue
  fi

  case "$arg" in
    --dry-run|-n)
      DRY_RUN=1
      ;;
    --yes|-y)
      AUTO_PUSH=1
      ;;
    tag=*)
      TAG_INPUT="${arg#tag=}"
      ;;
    patch|minor|major)
      TAG_INPUT="$arg"
      ;;
    v[0-9]*.[0-9]*.[0-9]*)
      TAG_INPUT="$arg"
      ;;
    *)
      echo "Unknown argument: $arg"
      echo "Usage: scripts/release/publish.sh [--dry-run|-n] [--yes|-y] [patch|minor|major|vX.Y.Z]"
      exit 1
      ;;
  esac
done

get_latest_tag() {
  git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1
}

sync_tags() {
  if git remote get-url origin >/dev/null 2>&1; then
    git fetch --tags --prune origin
  fi
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

format_commits() {
  local range="$1"

  if [ -z "$range" ]; then
    git log --oneline --no-decorate
  else
    git log --oneline --no-decorate "$range"
  fi
}

# run_checks runs the full gate quietly, collapsing its output to a single
# status line on success and only surfacing the detail when something fails.
run_checks() {
  printf '\nRunning checks (test, lint, vuln)... '
  local out
  if out="$(just check 2>&1)"; then
    ui_ok
  else
    echo "failed"
    printf '\n%s\n\n' "$out"
    echo "Checks failed; aborting release."
    exit 1
  fi
}

confirm_push() {
  local tag="$1"
  local auto="$2"

  if [ "$auto" -eq 1 ]; then
    return 0
  fi

  if [ -n "${CI:-}" ]; then
    return 1
  fi

  ui_prompt "Push branch main and tag ${tag} now? [Y/n]:"
  read -r response
  response="${response:-y}"

  response_lower="$(printf '%s' "$response" | tr '[:upper:]' '[:lower:]')"

  case "$response_lower" in
    y|yes)
      return 0
      ;;
    *)
      return 1
      ;;
  esac
}

working_tree_changes() {
  git status --porcelain
}

confirm_dirty_tree() {
  local changes="$1"

  ui_section "Uncommitted changes"
  printf '%s\n' "$changes" | sed 's/^/  /'
  echo

  if [ "$DRY_RUN" -eq 1 ]; then
    ui_dim "This is a dry run; no tag or push will be created."
  else
    ui_warn "continuing releases current HEAD only; uncommitted changes are not included."
  fi

  if [ "$AUTO_PUSH" -eq 1 ] || [ -n "${CI:-}" ]; then
    echo "Dirty working tree requires interactive confirmation; aborting release."
    exit 1
  fi

  ui_prompt "Continue with dirty working tree? [y/N]:"
  if ! read -r response; then
    echo
    echo "No response; aborting release."
    exit 1
  fi

  response_lower="$(printf '%s' "$response" | tr '[:upper:]' '[:lower:]')"
  case "$response_lower" in
    y|yes)
      return 0
      ;;
    *)
      echo "Aborted. Commit or stash changes before releasing."
      exit 0
      ;;
  esac
}

DIRTY_CHANGES="$(working_tree_changes)"
if [ -n "$DIRTY_CHANGES" ]; then
  confirm_dirty_tree "$DIRTY_CHANGES"
fi

if ! sync_tags; then
  echo "Failed to fetch tags from origin; aborting to avoid releasing from stale local tags."
  exit 1
fi

if [ -z "$TAG_INPUT" ]; then
  LATEST_TAG="$(get_latest_tag)"

  if [ -z "$LATEST_TAG" ]; then
    BASE_TAG="v0.0.0"
    LATEST_TAG="$BASE_TAG"
    RANGE=""
    ui_section "Release base"
    printf '  No previous release tag found. First semver tag starts from v0.0.0.\n'
    ui_section "Commits since initial commit"
    COMMITS="$(format_commits "")"
  else
    BASE_TAG="$LATEST_TAG"
    RANGE="${LATEST_TAG}..HEAD"
    ui_section "Release base"
    ui_kv "Last tag" "$LATEST_TAG"
    ui_section "Commits since $LATEST_TAG"
    COMMITS="$(format_commits "$RANGE")"
  fi

  echo "$COMMITS" | sed 's/^/  - /'
  CHANGE_COUNT="$(printf '%s\n' "$COMMITS" | sed '/^$/d' | wc -l | tr -d ' ')"

  if [ "$CHANGE_COUNT" -eq 0 ]; then
    echo
    echo "No commits to release."
    exit 0
  fi

  read -r BASE_MAJOR BASE_MINOR BASE_PATCH <<<"$(semver_from_tag "$BASE_TAG")"
  PATCH_CANDIDATE="$(next_version "$BASE_MAJOR" "$BASE_MINOR" "$BASE_PATCH" patch)"
  MINOR_CANDIDATE="$(next_version "$BASE_MAJOR" "$BASE_MINOR" "$BASE_PATCH" minor)"
  MAJOR_CANDIDATE="$(next_version "$BASE_MAJOR" "$BASE_MINOR" "$BASE_PATCH" major)"

  ui_section "Version bump"
  ui_kv "Commits" "$CHANGE_COUNT"
  printf '  [1] %-7s %s\n' "patch" "$PATCH_CANDIDATE"
  printf '  [2] %-7s %s\n' "minor" "$MINOR_CANDIDATE"
  printf '  [3] %-7s %s\n' "major" "$MAJOR_CANDIDATE"

  while true; do
    ui_prompt "Select bump [1/2/3] (default: 1):"
    read -r REPLY

    case "${REPLY:-1}" in
      1|patch)
        TAG_INPUT="patch"
        PATCH_TAG="$PATCH_CANDIDATE"
        break
        ;;
      2|minor)
        TAG_INPUT="minor"
        PATCH_TAG="$MINOR_CANDIDATE"
        break
        ;;
      3|major)
        TAG_INPUT="major"
        PATCH_TAG="$MAJOR_CANDIDATE"
        break
        ;;
      *)
        echo "Please enter 1, 2, or 3 (or patch/minor/major)."
        ;;
    esac
  done
else
  LATEST_TAG="$(get_latest_tag)"

  if [ -z "$LATEST_TAG" ]; then
    LATEST_TAG="v0.0.0"
    RANGE=""
  else
    RANGE="${LATEST_TAG}..HEAD"
  fi

  ui_section "Release base"
  ui_kv "Last tag" "$LATEST_TAG"
  ui_section "Commits since $LATEST_TAG"
  format_commits "$RANGE" | sed 's/^/  - /'
fi

case "$TAG_INPUT" in
  patch|minor|major)
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
  ui_section "Resolved version"
  ui_kv "Tag" "${PATCH_TAG} (from ${TAG_INPUT} bump)"
fi

if [ "$(git symbolic-ref --short HEAD)" != "main" ]; then
  echo "release must run on branch 'main'."
  exit 1
fi

TARGET_SHA="$(git rev-parse HEAD)"

if git rev-parse -q --verify "refs/tags/$PATCH_TAG" >/dev/null; then
  echo "Tag already exists: $PATCH_TAG"
  exit 1
fi

if [ "$DRY_RUN" -eq 0 ] && git ls-remote --exit-code --tags origin "refs/tags/$PATCH_TAG" >/dev/null 2>&1; then
  echo "Remote tag already exists: $PATCH_TAG"
  exit 1
fi

if [ "$DRY_RUN" -eq 1 ]; then
  ui_section "Dry run"
  ui_kv "Tag" "$PATCH_TAG"
  ui_kv "Target" "$TARGET_SHA"
  echo "  No tag or push was created."
  exit 0
fi

run_checks

if ! confirm_push "$PATCH_TAG" "$AUTO_PUSH"; then
  echo "Aborted. No tag created."
  exit 0
fi

RELEASE_NOTES="$(printf 'Release %s\n\n%s' "$PATCH_TAG" "$(format_commits "$RANGE" | sed 's/^/- /')")"
git tag -a "$PATCH_TAG" -m "$RELEASE_NOTES" "$TARGET_SHA"
git push --atomic origin HEAD:main "refs/tags/$PATCH_TAG:refs/tags/$PATCH_TAG"
echo "Created and pushed tag $PATCH_TAG"
