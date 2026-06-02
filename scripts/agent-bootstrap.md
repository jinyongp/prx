# gate agent bootstrap

Use this file when an AI agent is setting up `gate` for a repository.

1) Install `gate` binary:

```bash
curl -fsSL https://raw.githubusercontent.com/jinyongp/gate/main/scripts/install.sh | sh
```

2) Resolve the installed binary path. Agent sessions may not have
`$HOME/.local/bin` in `PATH`, so do not assume `gate` is directly executable:

```bash
if command -v gate >/dev/null 2>&1; then
  GATE_BIN="$(command -v gate)"
elif [ -x "$HOME/.local/bin/gate" ]; then
  GATE_BIN="$HOME/.local/bin/gate"
else
  echo "gate installed but not found; check installer output." >&2
  exit 1
fi
```

Use `"$GATE_BIN"` for every later `gate` command in this bootstrap.

3) Install the agent skill:

```bash
if command -v npx >/dev/null 2>&1; then
  npx skills add jinyongp/gate
elif command -v apm >/dev/null 2>&1; then
  apm install jinyongp/gate
else
  echo "No skill manager found (npx or apm); skip skill install and report this."
fi
```

4) Add `gate.toml` from project context:

- If `gate.toml` already exists, keep it and do not overwrite it.
- If missing, inspect the project and infer the dev port:
  - `package.json` scripts: Vite → `5173`; Next/Nuxt → `3000`; explicit script/config port wins
  - `go.mod` projects → `8080` default
  - `docker-compose.yml`/`docker-compose.yaml` with mapped host port → use that port
  - fallback: `3000`
- Convert the repo folder name to a DNS-safe label for the domain:
  - lowercase
  - replace every run of non-`[a-z0-9]` characters with `-`
  - trim leading/trailing `-`
  - fallback to `app` if the result is empty
- Create `gate.toml` using the detected service name and port:

```toml
[project]
name = "<repo-folder-name>"

[services.web]
domain = "<dns-safe-repo-name>.example.localhost"
port = <detected-port>
```

5) Trust the local root CA once if the machine has not done it yet:

```bash
"$GATE_BIN" trust
```

6) Suggest and run:

```bash
"$GATE_BIN" up
```

7) Report `GATE_BIN`, where the install landed, and what was detected.

8) When removing agent setup, run:

```bash
curl -fsSL https://raw.githubusercontent.com/jinyongp/gate/main/scripts/uninstall.sh | sh
# confirm prompt first, then remove all user-level gate artifacts
# unattended
curl -fsSL https://raw.githubusercontent.com/jinyongp/gate/main/scripts/uninstall.sh | sh -s -- -y
```

This removes user-level `config`, `data`, and `state` directories plus known installer target binaries,
then tries to stop a running daemon if a pid file is found.
Only paths actually present on the current machine are removed.
System trust store entries are intentionally left behind.
