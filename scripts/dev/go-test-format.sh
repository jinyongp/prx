#!/usr/bin/env bash
set -euo pipefail

go test -race "$@" ./... | awk '
  BEGIN { FS = "\t" }
  NF >= 3 {
    status = $1
    gsub(/^ +| +$/, "", status)
    if (status != "ok" && status != "?" && status != "FAIL" && status != "") {
      print
      next
    }
    pkg = $2
    elapsed = $3
    detail = $4
    if (status == "") {
      status = " "
    }
    if (detail == "") {
      printf "%-4s %-30s %s\n", status, pkg, elapsed
    } else {
      printf "%-4s %-30s %-12s %s\n", status, pkg, elapsed, detail
    }
    next
  }
  { print }
'
