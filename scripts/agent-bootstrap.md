# prx agent bootstrap

Use this file when an AI agent is setting up `prx` for a repository.

1) Install `prx` binary:

```bash
curl -fsSL https://raw.githubusercontent.com/jinyongp/prx/main/scripts/install.sh | sh
```

2) Install the agent skill:

```bash
if command -v npx >/dev/null 2>&1; then
  npx skills add jinyongp/prx
elif command -v apm >/dev/null 2>&1; then
  apm install jinyongp/prx
else
  echo "No skill manager found (npx or apm); skip skill install and report this."
fi
```

3) Add `prx.toml` from project context:

- If `prx.toml` already exists, keep it and do not overwrite it.
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
- Create `prx.toml` using the detected service name and port:

```toml
[project]
name = "<repo-folder-name>"

[services.web]
domain = "<dns-safe-repo-name>.example.localhost"
port = <detected-port>
```

4) Trust the local root CA once if the machine has not done it yet:

```bash
prx trust
```

5) Suggest and run:

```bash
prx up
```

6) Report where the install landed and what was detected.

7) When removing agent setup, run:

```bash
curl -fsSL https://raw.githubusercontent.com/jinyongp/prx/main/scripts/uninstall.sh | sh
# confirm prompt first, then remove all user-level prx artifacts
# unattended
curl -fsSL https://raw.githubusercontent.com/jinyongp/prx/main/scripts/uninstall.sh | sh -s -- -y
```

This removes user-level `config`, `data`, and `state` directories plus known installer target binaries,
then tries to stop a running daemon if a pid file is found.
Only paths actually present on the current machine are removed.
System trust store entries are intentionally left behind.
