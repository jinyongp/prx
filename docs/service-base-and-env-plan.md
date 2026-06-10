# Service Base And Env Plan

## Goal

Let projects keep local service-to-service configuration on loopback addresses
while gate derives stable service domains from a project base domain.

## Scope

- Add a project-level `base` domain.
- Derive service domains as `<service-name>.<base>` by default.
- Add a per-service `host` override for the leftmost label.
- Keep `domain` as a full-domain escape hatch for services that do not fit the
  base-derived model.
- Inject loopback URLs and ports into `gate run` child processes.
- Let each service publish its own internal loopback URL under one or more
  configured env names.
- Automatically inject both loopback and route URLs through gate-owned
  `GATE_*` env names.
- Update init, docs, usage, skill reference, command completion metadata, and
  tests.

## Non-goals

- Do not require application code to read gate-specific domains.
- Do not solve browser direct API URL discovery across local, LAN,
  Cloudflared, or Tailscale exposure modes.
- Do not add browser public URL maps, runtime config helpers, gate headers, or
  gate-specific endpoints in application code.
- Do not add `public_env` in this work.
- Do not preserve legacy `gate add <service> <domain> <port>` behavior.

## Constraints

- `host` and `domain` are mutually exclusive in one service.
- If `project.base` is present and a service has neither `host` nor `domain`,
  the service name is used as the host label.
- `host = "."` means the service domain is exactly `project.base`.
- Other `host` values are a single DNS label, not a dotted hostname.
- If `project.base` is absent, services must use `domain`.
- Injected service URLs for process-to-process communication use
  `http://127.0.0.1:<port>`.
- Explicit service-published env names always receive loopback URLs.
- Automatically injected gate-owned env names use a `GATE_` prefix and include
  both loopback and HTTPS route URLs.
- Gate-owned env service keys are derived by uppercasing the service name and
  replacing non-alphanumeric characters with `_`.
- Service-declared env names may write unprefixed names because the config
  author opted into those names.
- Service-declared env names accept either a string or a list of strings.
- Two services must not publish the same env name.
- Two services must not derive the same gate-owned env service key.
- Service-declared env names must not use the reserved `GATE_` prefix.
- Env names must match shell env key syntax: `[A-Za-z_][A-Za-z0-9_]*`.
- `gate run` env injection uses reservations in the selected scope. If a
  service that publishes env names does not have a resolved port, `gate run`
  fails before spawning the child command.

## Assumptions

- `base` defaults to localhost mode during `gate init`, using the project name:
  `base = "<project>.localhost"`.
- Default service domain with base:
  - service `web`, base `myapp.localhost` becomes `web.myapp.localhost`
  - service `api`, base `myapp.localhost` becomes `api.myapp.localhost`
- Base-root service domain:
  - service with `host = "."`, base `myapp.localhost` becomes
    `myapp.localhost`
- Full-domain escape hatch:
  - service with `domain = "console.internal.example.com"` uses that domain and
    does not derive from `base`.
- `gate run web -- ...` injects all known peer services:
  - `GATE_WEB_PORT=<web-port>`
  - `GATE_API_PORT=<api-port>`
  - `GATE_WEB_URL=http://127.0.0.1:<web-port>`
  - `GATE_API_URL=http://127.0.0.1:<api-port>`
  - `GATE_WEB_ROUTE_URL=https://web.myapp.localhost`
  - `GATE_API_ROUTE_URL=https://api.myapp.localhost`
- A service may publish its internal loopback URL under project env names:
  - `[services.api] env = "API_URL"` injects
    `API_URL=http://127.0.0.1:<api-port>` into every `gate run` child.
  - `[services.api] env = ["API_URL", "INTERNAL_API_URL"]` injects both names.
- `gate add` uses the new base-derived shape:
  - `gate add web 3000`
  - `gate add api 3001 --host app`
  - `gate add admin 3002 --domain console.internal.example.com`
- `gate add <service> <port>` requires the current project to have
  `project.base`; projects without `base` must pass `--domain`.
- Browser direct API calls across exposure modes are intentionally out of scope.
  Projects may choose same-origin proxying, DNS/provider-specific hostnames, or
  a project-owned host derivation convention later.

## Work Items

- [x] Extend config schema with `project.base`, service `host`, and
      service-declared env names.
- [x] Validate base-derived domain rules and host/domain mutual exclusion.
- [x] Parse service `env` as either a single string or a list of strings.
- [x] Validate env names, reject duplicate env publishers, reject reserved
      `GATE_` names, and reject duplicate gate-owned env service keys.
- [x] Update domain resolution so registry reservations and route tables use
      the resolved domain.
- [x] Update config editing helpers to preserve comments while writing new
      fields.
- [x] Update `gate init` so localhost mode emits `base = "<project>.localhost"`
      and services derive from service names by default.
- [x] Change `gate add` to accept `<service> <port>` plus optional `--host` or
      `--domain`.
- [x] Reject `gate add <service> <port>` when the selected project has no
      `base`.
- [x] Update completion metadata for the new `gate add --host` and `--domain`
      flags.
- [x] Update `gate run` to inject `GATE_<SERVICE>_PORT`,
      `GATE_<SERVICE>_URL`, `GATE_<SERVICE>_ROUTE_URL`, and configured
      service-published env names.
- [x] Make `gate run` fail before spawning when a service that publishes env
      names has no resolved port.
- [x] Update docs/spec/usage/README/skill reference.
- [x] Add focused tests for config parsing, validation, init rendering,
      registry activation, add/up edits, and run env injection.

## Validation

- [x] Config with `[project] base = "myapp.localhost"` and `[services.web]`
      resolves `web.myapp.localhost`.
- [x] Config with `[services.web] domain = "web.localhost"` loads and routes as
      a domain escape hatch.
- [x] `host = "."` resolves to the bare base domain.
- [x] `host = "app"` resolves to `app.<base>`.
- [x] `host = "api.v1"` is rejected.
- [x] `host` plus `domain` in the same service is rejected.
- [x] Missing both `base` and service `domain` is rejected.
- [x] `gate init -y --name myapp` generates a valid base-derived config.
- [x] `gate add web 3000` writes a base-derived service.
- [x] `gate add web 3000 --host app` writes a host override.
- [x] `gate add web 3000 --domain custom.example.com` writes a domain escape
      hatch.
- [x] `gate add web 3000` fails in a project with no `base`.
- [x] `gate add web 3000 --domain custom.example.com` succeeds in a project
      with no `base`.
- [x] `gate up` reserves generated domains and keeps stable ports.
- [x] `gate run web -- env` includes peer `GATE_*` loopback URLs and ports.
- [x] `gate run web -- env` includes peer `GATE_*_ROUTE_URL` HTTPS route URLs.
- [x] Service name `admin-web` injects `GATE_ADMIN_WEB_PORT`,
      `GATE_ADMIN_WEB_URL`, and `GATE_ADMIN_WEB_ROUTE_URL`.
- [x] Services `admin-web` and `admin_web` in the same selected scope are
      rejected because they derive the same `GATE_ADMIN_WEB` env key.
- [x] `[services.api] env = "API_URL"` injects
      `API_URL=http://127.0.0.1:<api-port>` into `gate run web`.
- [x] `[services.api] env = ["API_URL", "INTERNAL_API_URL"]` injects both names.
- [x] Duplicate service env names are rejected.
- [x] Service-declared env name `GATE_API_URL` is rejected because `GATE_` is
      reserved for gate-owned env values.
- [x] Invalid env names are rejected.
- [x] `gate run web -- ...` fails before spawning when a service that publishes
      env names has no reserved port.
- [x] Run focused Go tests for config/init/up/run behavior.
- [x] Run `just test`.
- [x] Run `just check`.

## Risks

- Changing init output changes the default generated `gate.toml` shape.
- `gate add` and comment-preserving edits may need careful behavior when a
  project uses `base`.
- Service-declared env injection can overwrite existing child env values when
  explicitly configured; this must be documented as intentional.
- HTTPS route URLs depend on gate routing, trust, and daemon availability; they
  are provided as opt-in `GATE_*_ROUTE_URL` values rather than the default
  service-published env value.
- Full-domain escape hatches may not participate cleanly in peer host derivation
  conventions.
- Browser direct API calls across exposure modes remain unsolved by this
  structure-only work. That limitation is intentional.
