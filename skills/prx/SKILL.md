---
name: prx
description: Drive the prx local HTTPS reverse proxy and port registry from the command line. Use when a repository has a prx.toml and you need to map domains to local dev servers over trusted HTTPS, allocate or look up ports without hardcoding them, start a dev server on its assigned port, or inspect the machine-wide registry. Commands that document --json emit stable, parseable output.
---

# prx

`prx` maps local domains to dev servers over trusted HTTPS and manages a
machine-wide port registry. Commands write data to stdout and diagnostics to
stderr. Commands marked `--json` emit a single JSON value on success; JSON-mode
errors use a stderr envelope.

## When to use

- A project has a `prx.toml` declaring services (`domain` → optional `port`).
- You need a stable, non-conflicting port for a dev server.
- You need to bring routes up/down or check what is mapped.

## Core commands

| command | purpose |
| --- | --- |
| `prx init [--json] [--name name] [--force]` | scaffold a starter `prx.toml` |
| `prx up [--json] [--dns localhost\|hosts]` | reserve/allocate ports, reflect DNS, push routes |
| `prx down [--json]` | deactivate this project's routes (reservations kept) |
| `prx ls [--json]` | list reservations with live/down status |
| `prx port <service> [--json]` | print the reserved port (for scripts) |
| `prx run <service> -- <cmd...>` | run a command with `PORT` injected |
| `prx add <domain> <port> [--json]` | reserve a domain→port mapping |
| `prx rm <domain> [--json]` | remove a reservation |
| `prx prune [--json]` | GC reservations whose prx.toml is gone |
| `prx daemon start\|stop\|status [--json]\|logs` | manage the resident proxy; `--json` is for `status` |
| `prx trust` | install the root CA (one time) |
| `prx ca export [--out path]` | export the root CA for other devices |
| `prx expose <service> --via <provider> [--auth user:pass] [--json]` | reach a service externally |
| `prx upgrade [--yes]` | upgrade to the latest GitHub release |
| `prx skill path\|print` | locate or print this skill file |

## Exit codes

`0` ok · `1` error · `2` usage · `3` permission (needs sudo) · `4` port/domain conflict.

## Recipes

Start a dev server on its assigned port:

```bash
prx up
prx run web -- pnpm dev          # PORT is injected
```

Get a port for a script:

```bash
PORT=$(prx port web) pnpm dev
```

Inspect mappings as JSON:

```bash
prx ls --json
```

JSON-mode errors are written to stderr:

```json
{"error":{"code":"port_conflict","message":"port 4310 already reserved by myapp/web"}}
```

## prx.toml

```toml
[project]
name = "myapp"

[services.web]
domain = "app.example.com"       # port omitted -> auto-allocated

[services.api]
domain = "api.example.com"
port = 3001                      # fixed when needed
```

Inside a project, `prx add`/`prx rm` edit this file in place, preserving comments.
Outside a project they create/remove adhoc registry reservations. Domains ending
in `.localhost` need no sudo; custom domains use `/etc/hosts` (sudo).
