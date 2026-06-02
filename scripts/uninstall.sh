#!/usr/bin/env sh
set -eu

FOUND=0
FAILED=0
FORCE=0
WORKFILE="$(mktemp)"
SORTED_FILE="${WORKFILE}.sorted"
trap 'rm -f "$WORKFILE" "$SORTED_FILE"' EXIT

SCRIPT_DIR="$(CDPATH="" cd "$(dirname "$0")" 2>/dev/null && pwd || pwd)"
if [ -r "${SCRIPT_DIR}/lib/ui.sh" ]; then
  . "${SCRIPT_DIR}/lib/ui.sh"
else
  ui_section() { printf '\n%s\n' "$1"; }
  ui_ok() { printf 'ok: %s\n' "$1"; }
  ui_error() { printf 'error: %s\n' "$1" >&2; }
  ui_prompt() { printf '\n%s ' "$1"; }
fi

for arg in "$@"; do
  case "$arg" in
    -y|--yes|--force)
      FORCE=1
      ;;
    -h|--help)
      echo "Usage: sh uninstall.sh [--yes|--force|-y]" >&2
      exit 0
      ;;
    *)
      ui_error "unsupported argument: $arg"
      exit 1
      ;;
  esac
done

OS="$(uname -s | tr "[:upper:]" "[:lower:]")"
HOME_DIR="${HOME:?HOME is required to locate user uninstall targets}"

gate_config_dir() {
  if [ -n "${XDG_CONFIG_HOME:-}" ]; then
    printf '%s\n' "${XDG_CONFIG_HOME}/gate"
    return
  fi
  printf '%s\n' "${HOME_DIR}/.config/gate"
}

gate_data_dir() {
  if [ -n "${XDG_DATA_HOME:-}" ]; then
    printf '%s\n' "${XDG_DATA_HOME}/gate"
    return
  fi
  printf '%s\n' "${HOME_DIR}/.local/share/gate"
}

gate_state_dir() {
  if [ -n "${XDG_STATE_HOME:-}" ]; then
    printf '%s\n' "${XDG_STATE_HOME}/gate"
    return
  fi
  if [ "$OS" = "darwin" ]; then
    printf '%s\n' "${HOME_DIR}/Library/Logs/gate"
    return
  fi
  printf '%s\n' "${HOME_DIR}/.local/state/gate"
}

collect_paths() {
  cfg_dir="$(gate_config_dir)"
  dat_dir="$(gate_data_dir)"
  st_dir="$(gate_state_dir)"

  if [ -e "$cfg_dir" ] || [ -L "$cfg_dir" ]; then
    printf '%s\n' "$cfg_dir" >> "$WORKFILE"
  fi
  if [ -e "$dat_dir" ] || [ -L "$dat_dir" ]; then
    printf '%s\n' "$dat_dir" >> "$WORKFILE"
  fi
  if [ -e "$st_dir" ] || [ -L "$st_dir" ]; then
    printf '%s\n' "$st_dir" >> "$WORKFILE"
  fi

  if [ -n "${GATE_BIN_DIR:-}" ] && { [ -f "${GATE_BIN_DIR}/gate" ] || [ -L "${GATE_BIN_DIR}/gate" ]; }; then
    printf '%s\n' "${GATE_BIN_DIR}/gate" >> "$WORKFILE"
  fi
  if [ -f "${HOME_DIR}/.local/bin/gate" ] || [ -L "${HOME_DIR}/.local/bin/gate" ]; then
    printf '%s\n' "${HOME_DIR}/.local/bin/gate" >> "$WORKFILE"
  fi
  if [ -f "/usr/local/bin/gate" ] || [ -L "/usr/local/bin/gate" ]; then
    printf '%s\n' "/usr/local/bin/gate" >> "$WORKFILE"
  fi
}

collect_paths
sort -u "$WORKFILE" > "$SORTED_FILE"
if [ -s "$SORTED_FILE" ]; then
  ui_section "Discovered artifacts"
  printf '  Only existing discovered paths will be removed:\n'
  sed 's/^/  - /' "$SORTED_FILE"
  if [ "$FORCE" -ne 1 ]; then
    ui_prompt "Type y to proceed, anything else to cancel [y/N]:"
    if ! read -r response; then
      echo "Uninstall canceled."
      exit 0
    fi
    case "$response" in
      y|Y|yes|Yes|YES)
        ;;
      *)
        echo "Uninstall canceled."
        exit 0
        ;;
    esac
  fi
fi

stop_daemon() {
  pid_file="$1/gate.pid"
  if [ ! -f "$pid_file" ]; then
    return
  fi
  PID="$(tr -dc '0-9' < "$pid_file" | sed 's/[[:space:]]//g')"
  if [ -z "$PID" ]; then
    return
  fi
  if kill -0 "$PID" 2>/dev/null; then
    args="$(ps -p "$PID" -o args= 2>/dev/null || true)"
    case "$args" in
      gate\ __serve*|*/gate\ __serve*) ;;
      *)
        ui_error "skipping daemon stop for stale/non-gate pid: $PID"
        return
        ;;
    esac
    kill "$PID" 2>/dev/null || true
  fi
}

while IFS= read -r target; do
  if [ ! -e "$target" ] && [ ! -L "$target" ]; then
    continue
  fi

  if [ -d "$target" ]; then
    if [ "$(basename "$target")" = "gate" ]; then
      stop_daemon "$target"
    fi
    if rm -rf "$target"; then status=0; else status=$?; fi
  elif [ -f "$target" ] || [ -L "$target" ]; then
    if rm -f "$target"; then status=0; else status=$?; fi
  else
    status=1
  fi

  if [ "$status" = "0" ]; then
    ui_ok "removed $target"
    FOUND=1
  else
    if [ -e "$target" ] || [ -L "$target" ]; then
      ui_error "failed to remove: $target"
      FAILED=1
    fi
  fi
done < "$SORTED_FILE"

if [ "$FOUND" -eq 0 ]; then
  ui_section "Uninstall complete"
  echo "No gate installation artifacts found."
  exit 0
fi

if command -v rehash >/dev/null 2>&1; then
  rehash
fi
if command -v hash >/dev/null 2>&1; then
  hash -r
fi

if [ "$FAILED" -eq 1 ]; then
  ui_error "gate uninstall completed with errors."
  exit 1
fi

ui_section "Uninstall complete"
ui_ok "gate uninstalled."
