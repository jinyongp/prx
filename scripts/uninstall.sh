#!/usr/bin/env sh
set -eu

FOUND=0
TMP_FILE="$(mktemp)"
SORTED_FILE="${TMP_FILE}.sorted"
trap 'rm -f "$TMP_FILE" "$SORTED_FILE"' EXIT

collect_prx_paths() {
  if command -v which >/dev/null 2>&1; then
    which -a prx 2>/dev/null | sed '/^$/d' > "$TMP_FILE" || true
    return
  fi

  command -v prx 2>/dev/null | awk '/\// {print $NF}' >> "$TMP_FILE" || true

  if [ -n "${PATH:-}" ]; then
    IFS_BACKUP="$IFS"
    IFS=":"
    for dir in $PATH; do
      if [ -n "$dir" ]; then
        if [ -f "$dir/prx" ] || [ -L "$dir/prx" ]; then
          printf "%s\n" "$dir/prx" >> "$TMP_FILE"
        fi
      fi
    done
    IFS="$IFS_BACKUP"
  fi
}

collect_prx_paths

sort -u "$TMP_FILE" > "$SORTED_FILE"

if [ ! -s "$SORTED_FILE" ]; then
  echo "No prx executable found in PATH."
  exit 0
fi

while IFS= read -r BIN; do
  if [ -f "$BIN" ] || [ -L "$BIN" ]; then
    rm -f "$BIN"
    echo "Removed: $BIN"
    FOUND=1
  fi
done < "$SORTED_FILE"

if [ "$FOUND" -eq 0 ]; then
  echo "No prx binary found in install locations."
  exit 0
fi

if command -v rehash >/dev/null 2>&1; then
  rehash
fi

if command -v hash >/dev/null 2>&1; then
  hash -r
fi

echo "prx uninstalled."
