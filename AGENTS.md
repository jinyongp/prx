# AGENTS.md

Guide for AI agents working **on the gate codebase**. To use gate as a tool, see
[`skills/gate/SKILL.md`](skills/gate/SKILL.md) instead — do not duplicate usage docs here.

gate = local-dev global HTTPS reverse proxy + port registry, single Go binary.
Design and implementation spec: [`docs/spec.md`](docs/spec.md).

Module path: `gate` (bare; no VCS host dependency).

## Commands

Command runner is **`just`** (install: <https://github.com/casey/just>). Prefer recipes over raw `go`.

| recipe | action |
| --- | --- |
| `just build` | `go build -o bin/gate ./cmd/gate` |
| `just test` | `go test -race ./...` |
| `just lint` | `golangci-lint run ./...` |
| `just lint-json` | JSON diagnostics → stdout, human text → stderr |
| `just vuln` | `govulncheck ./...` |
| `just check` | test + lint + vuln (**must pass before opening a PR**) |
| `just fmt` | gofmt + goimports |

## Working loop

Do **not** "read the code and fix it" blindly. Instead:

1. Run `just lint-json` and parse the JSON diagnostics from stdout.
2. Fix issues based on the structured output.
3. Repeat `just check` until green.

For changes that are not lint-driven, still run the narrowest relevant check
first, then `just check` before opening a PR.

## Source of truth

Use [`docs/spec.md`](docs/spec.md) for architecture, implementation constraints,
platform support, command behavior, output contracts, and exit codes.

Use [`skills/gate/SKILL.md`](skills/gate/SKILL.md) for gate usage docs. Do not
duplicate end-user command examples or JSON schema details here.
