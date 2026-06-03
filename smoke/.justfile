# Local smoke-test server recipes.
set quiet

root := justfile_directory()

[private]
default:
  @just --justfile "{{root}}/justfile" --working-directory "{{root}}" --list smoke

[private]
build-gate:
  cd "{{root}}" && scripts/dev/build.sh

[doc('run the smoke server until stopped, then clean up')]
serve: build-gate
  tmp="$(mktemp -d /tmp/gate-smoke.XXXXXX)"; \
  port="$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')"; \
  export SMOKE_PORT="$port" XDG_CONFIG_HOME="$tmp/config" XDG_DATA_HOME="$tmp/data" XDG_STATE_HOME="$tmp/state"; \
  cleanup() { \
    cd "{{root}}/smoke/app" && ../../bin/gate down >/dev/null 2>&1 || true; \
    cd "{{root}}/smoke/app" && ../../bin/gate daemon stop >/dev/null 2>&1 || true; \
    cd "{{root}}/smoke/app" && ../../bin/gate rm --project smoke >/dev/null 2>&1 || true; \
    rm -rf "$tmp"; \
  }; \
  trap cleanup EXIT INT TERM; \
  cd "{{root}}/smoke/app"; \
  ../../bin/gate up --daemon --https-addr :0 --http-addr :0; \
  printf '\nstop with Ctrl-C; cleanup runs automatically\ncert: browser warning is expected until you trust the local CA\n\n'; \
  ../../bin/gate run web -- go run .

[doc('run the smoke server health check, then clean up')]
check: build-gate
  tmp="$(mktemp -d /tmp/gate-smoke.XXXXXX)"; \
  app_pid=""; \
  port="$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')"; \
  export SMOKE_PORT="$port" XDG_CONFIG_HOME="$tmp/config" XDG_DATA_HOME="$tmp/data" XDG_STATE_HOME="$tmp/state"; \
  cleanup() { \
    if [ -n "$app_pid" ]; then kill "$app_pid" >/dev/null 2>&1 || true; fi; \
    cd "{{root}}/smoke/app" && ../../bin/gate down >/dev/null 2>&1 || true; \
    cd "{{root}}/smoke/app" && ../../bin/gate daemon stop >/dev/null 2>&1 || true; \
    cd "{{root}}/smoke/app" && ../../bin/gate rm --project smoke >/dev/null 2>&1 || true; \
    rm -rf "$tmp"; \
  }; \
  trap cleanup EXIT INT TERM; \
  cd "{{root}}/smoke/app"; \
  up_output="$(../../bin/gate up --daemon --https-addr :0 --http-addr :0)"; \
  printf '%s\n' "$up_output"; \
  url="$(printf '%s\n' "$up_output" | awk '/smoke\/web/ {print $2; exit}')"; \
  test -n "$url"; \
  ../../bin/gate run web -- go run . >/dev/null 2>&1 & app_pid="$!"; \
  for _ in 1 2 3 4 5 6 7 8 9 10; do \
    if body="$(curl -kfsS "$url/healthz" 2>/dev/null)" && [ "$body" = "ok" ]; then printf '%s\n' "$body"; exit 0; fi; \
    sleep 0.25; \
  done; \
  body="$(curl -kfsS "$url/healthz")"; \
  printf '%s\n' "$body"; \
  test "$body" = "ok"

[doc('show the smoke route allocation, then clean up')]
route: build-gate
  tmp="$(mktemp -d /tmp/gate-smoke.XXXXXX)"; \
  port="$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')"; \
  export SMOKE_PORT="$port" XDG_CONFIG_HOME="$tmp/config" XDG_DATA_HOME="$tmp/data" XDG_STATE_HOME="$tmp/state"; \
  cleanup() { \
    cd "{{root}}/smoke/app" && ../../bin/gate down >/dev/null 2>&1 || true; \
    cd "{{root}}/smoke/app" && ../../bin/gate daemon stop >/dev/null 2>&1 || true; \
    cd "{{root}}/smoke/app" && ../../bin/gate rm --project smoke >/dev/null 2>&1 || true; \
    rm -rf "$tmp"; \
  }; \
  trap cleanup EXIT INT TERM; \
  cd "{{root}}/smoke/app"; \
  ../../bin/gate up --daemon --https-addr :0 --http-addr :0; \
  ../../bin/gate ls
