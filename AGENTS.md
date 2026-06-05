# AGENTS.md

Guide for AI agents working **on the gate codebase**. To use gate as a tool, see
[`skills/gate/SKILL.md`](skills/gate/SKILL.md) instead — do not duplicate usage docs here.

gate = local-dev global HTTPS reverse proxy + port registry, single Go binary.
Design and implementation spec: [`docs/spec.md`](docs/spec.md).

Module path: `gate` (bare; no VCS host dependency).

## Commands

Use `just` recipes instead of raw `go` when a recipe exists.

- `just test`: race-enabled Go tests.
- `just lint`: golangci-lint.
- `just lint-json`: structured lint diagnostics; use for lint-fix work.
- `just check`: test + lint + vuln; run before opening a PR.
- `just fmt`: gofmt + goimports.

For ordinary changes, run the narrowest relevant check first, then `just check`
when the change is ready.

## Documentation boundaries

Use [`docs/spec.md`](docs/spec.md) for product boundaries, architecture, state
models, security invariants, and implementation constraints. Keep it focused on
what the system must do and why; avoid duplicating command examples, exact
output fields, or CLI reference details there.

Use [`docs/usage.md`](docs/usage.md) for end-user command syntax, examples,
output semantics, JSON behavior, troubleshooting, and exit codes.

Use [`skills/gate/SKILL.md`](skills/gate/SKILL.md) for the concise operational
reference agents need when using gate as a tool. Do not duplicate end-user
examples or JSON schema details here.
