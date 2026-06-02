# prx usage

`prx` maps local domains to local dev servers over HTTPS and keeps a
machine-wide registry of domain and port reservations.

Use project mode when a repository should carry its routing in `prx.toml`.
Use standalone mode when you want a machine-local mapping without adding a
project file.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/jinyongp/prx/main/scripts/install.sh | sh
```

If the install directory is not in `PATH`, resolve the binary once:

```bash
if command -v prx >/dev/null 2>&1; then
  PRX_BIN="$(command -v prx)"
elif [ -x "$HOME/.local/bin/prx" ]; then
  PRX_BIN="$HOME/.local/bin/prx"
else
  echo "prx not found" >&2
  exit 1
fi
```

The rest of this document uses `prx`; replace it with `"$PRX_BIN"` when needed.

## Trust HTTPS

`prx` issues local certificates from a local root CA. To remove browser
certificate warnings, trust the CA once:

```bash
prx trust
```

This can require OS administrator approval. `.localhost` domains need no DNS
setup. Custom domains can require `/etc/hosts` changes, so commands that reflect
DNS may ask for permission.

## Project Mode

Project mode uses a `prx.toml` file in the repository. This is the shareable,
repeatable path for a team.

Create a starter config:

```bash
prx init
```

Non-interactive default:

```bash
prx init -y
```

Example `prx.toml`:

```toml
[project]
name = "myapp"

[services.web]
domain = "app.localhost"

[services.api]
domain = "api.localhost"
port = 3001
```

Bring the project up and start the daemon:

```bash
prx up -d
```

Run a dev server with its reserved port injected as `PORT`:

```bash
prx run web -- pnpm dev
```

Open:

```text
https://app.localhost
```

Stop routing for the current project while keeping reservations:

```bash
prx down
```

## Standalone Mode

Standalone mode creates machine-local mappings without `prx.toml`. It is useful
when you already know the domain and port and do not want a project file.

Add a mapping:

```bash
prx add web.localhost 3000
```

If the daemon is running, routes are hot-reloaded. If it is stopped, starting it
later loads active standalone routes from the registry:

```bash
prx daemon start
```

Run a dev server through the standalone reservation:

```bash
prx run web.localhost -- pnpm dev
```

Or use the port in your own command:

```bash
PORT=$(prx port web.localhost) pnpm dev
```

Open:

```text
https://web.localhost
```

Remove the standalone mapping:

```bash
prx rm web.localhost
```

Standalone mode is local-machine routing. `prx expose` currently works with
project services, not standalone domains.

## Inspect Reservations

Current project reservations:

```bash
prx ls
```

All reservations:

```bash
prx ls -a
```

Only live or down reservations:

```bash
prx ls --status live
prx ls --status down
```

Port-focused view for the current project:

```bash
prx port
```

All reserved ports:

```bash
prx port -a
```

One service or standalone domain:

```bash
prx port web
prx port web.localhost
```

## Manage Reservations

Add a domain and fixed port:

```bash
prx add web.localhost 3000
```

Inside a project, this appends a `[services.<name>]` block to `prx.toml` and
updates the registry. Outside a project, this creates a standalone registry
reservation.

Remove a domain:

```bash
prx rm web.localhost
```

Inside a project, if the domain belongs to a service in `prx.toml`, that service
block is removed and the registry is updated. Otherwise, the matching registry
reservation is removed.

Remove all reservations for the current project:

```bash
prx rm --project
```

Remove all reservations for a named project:

```bash
prx rm --project myapp
```

Prune stale reservations whose owning `prx.toml` no longer exists:

```bash
prx prune
```

Standalone reservations are not pruned by `prx prune` because they have no
owning config file.

## Daemon

Start, stop, restart, and inspect the resident proxy:

```bash
prx daemon start
prx daemon stop
prx daemon restart
prx daemon status
prx daemon logs
```

Start on custom front-proxy ports:

```bash
prx daemon start --https-addr 127.0.0.1:18443 --http-addr 127.0.0.1:18080
```

`prx up -d` starts the daemon when needed and reloads project routes.

## JSON Output

Commands that support `--json` write a single JSON object to stdout. Errors in
JSON mode are written to stderr as an error envelope.

Examples:

```bash
prx up --json
prx ls --json
prx port -a --json
prx daemon status --json
prx add web.localhost 3000 --json
prx rm web.localhost --json
prx prune --json
```

## Access From Another Device

Access from another device needs two separate pieces:

1. The other device must resolve the domain to a reachable address.
2. The other device must trust prx's root CA if you want a clean HTTPS page.

Export the root CA certificate:

```bash
prx ca export --out prx-root.crt
```

Install `prx-root.crt` on the other device as a trusted root certificate. Do
not copy or share `root.key`; only export/share the `.crt` file.

### Same Machine

For browser access on the same machine, use `.localhost` domains when possible:

```toml
[services.web]
domain = "app.localhost"
```

Then:

```bash
prx trust
prx up -d
prx run web -- pnpm dev
```

Open:

```text
https://app.localhost
```

### LAN

Use LAN access when a phone, tablet, or another computer on the same network
must reach your dev server.

Prerequisites:

- Use a `.local` service domain.
- Start prx routes first with `prx up -d`.
- Start the dev server, usually with `prx run <service> -- ...`.
- Install the exported prx root CA on other devices if you want trusted HTTPS.
- Make sure the other device can resolve the `.local` name to the development
  machine. If name resolution does not work on your network, add a hosts entry
  or use another local DNS mechanism.

Limitations:

- `prx expose <service> --via lan` only accepts a `.local` domain.
- The current LAN provider does not itself advertise mDNS or edit other devices'
  DNS/hosts files. It validates the domain and marks the running prx route as
  exposed.
- `prx expose` currently works with project services, not standalone domains.
- Devices must be on a network path that can reach the development machine.
- Browser trust still depends on installing the prx root CA on each client
  device.

Example `prx.toml`:

```toml
[project]
name = "myapp"

[services.web]
domain = "myapp.local"
```

Start the proxy and service:

```bash
prx trust
prx up -d
prx run web -- pnpm dev
```

Expose the route for LAN clients:

```bash
prx expose web --via lan
```

On another device, make sure `myapp.local` resolves to the development machine,
then open:

```text
https://myapp.local
```

If needed, find the development machine's LAN IP with your OS network settings
and map the name manually on the other device:

```text
192.168.0.42 myapp.local
```

### Public URL With Cloudflared

Use this when you want a temporary public URL. `cloudflared` must be installed
and available in `PATH`.

Prerequisites:

- Install [`cloudflared`](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/).
- Allow outbound internet access from the development machine.
- Start prx routes first with `prx up -d`.
- Start the dev server, usually with `prx run <service> -- ...`.
- Run `prx trust` on the development machine if `cloudflared` needs to trust
  prx's local HTTPS origin.

No Cloudflare account, zone, DNS record, or tunnel config file is required for
this quick-tunnel mode.

Example:

```bash
prx expose web --via cloudflared
```

The command starts a quick tunnel to `https://<service-domain>` and prints a
`trycloudflare.com` URL:

```text
web exposed via cloudflared
  https://random-name.trycloudflare.com -> app.localhost
```

For a public URL, prefer auth:

```bash
prx expose web --via cloudflared --auth user:pass
```

Without `--auth`, prx warns because anyone with the public URL can reach your
dev server.

Limitations:

- The URL is temporary and random.
- The URL is not tied to your own domain.
- The tunnel lasts only while the `cloudflared` process keeps running.
- `prx expose` currently works with project services, not standalone domains.
- The tunnel targets prx's local HTTPS origin. If origin certificate trust fails,
  run `prx trust` on the development machine and retry.
- This mode is not a stable production tunnel configuration.

### Tailnet With Tailscale

Use this when devices are on the same Tailscale tailnet. `tailscale` must be
installed, logged in, and available in `PATH`.

Prerequisites:

- Install [Tailscale](https://tailscale.com/download).
- Log the development machine into a tailnet.
- Make sure target devices are also in the same tailnet, or otherwise allowed by
  your tailnet policy.
- Enable/use [Tailscale Serve](https://tailscale.com/docs/features/tailscale-serve)
  support for the machine.
- Start prx routes first with `prx up -d`.
- Start the dev server, usually with `prx run <service> -- ...`.

Limitations:

- `prx expose` currently works with project services, not standalone domains.
- Access is limited to devices allowed by the tailnet and ACLs.
- The current implementation runs `tailscale serve --bg`; prx does not manage
  detailed Tailscale Serve state after that.
- Tear-down is manual with Tailscale commands.

```bash
prx expose web --via tailscale
```

This runs `tailscale serve --bg https://<service-domain>`. Tear it down with
Tailscale's serve controls, for example:

```bash
tailscale serve reset
```

### Expose Command Reference

Supported providers:

| provider | purpose | notes |
| --- | --- | --- |
| `local` | no external exposure | returns the local HTTPS URL |
| `lan` | same-network access | requires a `.local` service domain |
| `cloudflared` | temporary public URL | requires `cloudflared` |
| `tailscale` | tailnet access | requires `tailscale` |

`prx expose` currently requires a project service name:

```bash
prx expose <service> --via <provider>
```

It does not currently accept standalone domains directly.

## CA Export

Export the root CA certificate for another device:

```bash
prx ca export --out prx-root.crt
```

## Upgrade

```bash
prx upgrade
```

Skip confirmation:

```bash
prx upgrade -y
```

## Completion

```bash
prx completion bash
prx completion zsh
prx completion fish
```

## Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/jinyongp/prx/main/scripts/uninstall.sh | sh
```

Non-interactive:

```bash
curl -fsSL https://raw.githubusercontent.com/jinyongp/prx/main/scripts/uninstall.sh | sh -s -- -y
```

The uninstall script removes user-level config, data, state, and known binary
paths that exist on the machine. System trust store entries are intentionally
left behind.

## Exit Codes

| code | meaning |
| --- | --- |
| 0 | success |
| 1 | error |
| 2 | usage error |
| 3 | permission required |
| 4 | port or domain conflict |
