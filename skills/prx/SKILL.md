---
name: prx
description: Drive the prx local HTTPS reverse proxy and port registry from the command line. Use when a repository has a prx.toml and you need to map domains to local dev servers over trusted HTTPS, allocate or look up ports without hardcoding them, start a dev server on its assigned port, or inspect the machine-wide registry. All commands accept --json for stable, parseable output.
---

# prx

`prx` maps local domains to dev servers over trusted HTTPS and manages a
machine-wide port registry. Every command writes data to stdout and logs to
stderr; pass `--json` for a single JSON value (pipe-safe).

## When to use

- A project has a `prx.toml` declaring services (`domain` → optional `port`).
- You need a stable, non-conflicting port for a dev server.
- You need to bring routes up/down or check what is mapped.

## Core commands

| command | purpose |
| --- | --- |
| `prx up [--json] [--dns localhost\|hosts]` | reserve/allocate ports, reflect DNS, push routes |
| `prx down [--json]` | deactivate this project's routes (reservations kept) |
| `prx ls [--json]` | list reservations with live/down status |
| `prx port <service>` | print the reserved port (for scripts) |
| `prx run <service> -- <cmd...>` | run a command with `PORT` injected |
| `prx add <domain> <port>` | reserve a domain→port mapping |
| `prx rm <domain>` | remove a reservation |
| `prx prune [--json]` | GC reservations whose prx.toml is gone |
| `prx daemon start\|stop\|status` | manage the resident proxy |
| `prx trust` | install the root CA (one time) |
| `prx ca export [--out path]` | export the root CA for other devices |
| `prx expose <service> --via <provider> [--auth user:pass]` | reach a service externally |
| `prx skill path` | print the path of this skill file |

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

`prx add`/`prx rm` edit this file in place, preserving comments. Domains ending
in `.localhost` need no sudo; custom domains use `/etc/hosts` (sudo).
