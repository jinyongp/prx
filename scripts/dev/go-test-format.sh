#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(CDPATH="" cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${SCRIPT_DIR}/../lib/ui.sh"

go test -race "$@" ./... | awk '
  BEGIN {
    FS = "\t"
    color = ENVIRON["UI_COLOR"] == "1"
    reset = ENVIRON["UI_RESET"]
    bold = ENVIRON["UI_BOLD"]
    dim = ENVIRON["UI_DIM"]
    green = ENVIRON["UI_GREEN"]
    yellow = ENVIRON["UI_YELLOW"]
    red = ENVIRON["UI_RED"]
    status_width = 4
    pkg_width = 30
    elapsed_width = 12
  }
  function padding(value, width,    n, out, i) {
    n = width - length(value)
    out = ""
    for (i = 0; i < n; i++) {
      out = out " "
    }
    return out
  }
  function max(a, b) {
    return a > b ? a : b
  }
  function styled_status(value) {
    if (!color) {
      return value
    }
    if (value == "ok") {
      return green value reset
    }
    if (value == "?") {
      return yellow value reset
    }
    if (value == "FAIL") {
      return red value reset
    }
    return value
  }
  function styled_pkg(value) {
    return color ? bold value reset : value
  }
  function styled_elapsed(value) {
    if (!color) {
      return value
    }
    if (value == "(cached)" || value == "[no test files]") {
      return dim value reset
    }
    return value
  }
  function styled_detail(value) {
    if (color && value ~ /^coverage:/) {
      return dim value reset
    }
    return value
  }
  NF >= 3 {
    line_count++
    status = $1
    gsub(/^ +| +$/, "", status)
    if (status != "ok" && status != "?" && status != "FAIL" && status != "") {
      raw[line_count] = $0
      next
    }
    pkg = $2
    elapsed = $3
    detail = $4
    if (status == "" && detail ~ /^coverage:/) {
      status = "?"
      elapsed = "[no test files]"
    } else if (status == "") {
      status = " "
    }

    formatted[line_count] = 1
    statuses[line_count] = status
    pkgs[line_count] = pkg
    elapsed_values[line_count] = elapsed
    details[line_count] = detail
    status_width = max(status_width, length(status))
    pkg_width = max(pkg_width, length(pkg))
    elapsed_width = max(elapsed_width, length(elapsed))
    next
  }
  {
    line_count++
    raw[line_count] = $0
  }
  END {
    for (i = 1; i <= line_count; i++) {
      if (!formatted[i]) {
        print raw[i]
        continue
      }
      status = statuses[i]
      pkg = pkgs[i]
      elapsed = elapsed_values[i]
      detail = details[i]
      if (detail == "") {
        printf "%s%s %s%s %s\n", styled_status(status), padding(status, status_width), styled_pkg(pkg), padding(pkg, pkg_width), styled_elapsed(elapsed)
      } else {
        printf "%s%s %s%s %s%s %s\n", styled_status(status), padding(status, status_width), styled_pkg(pkg), padding(pkg, pkg_width), styled_elapsed(elapsed), padding(elapsed, elapsed_width), styled_detail(detail)
      }
    }
  }
'
