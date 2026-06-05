# Follow-ups

This file tracks useful follow-up work that is not required for the current
documentation boundary cleanup.

## Documentation

### Output Reference Shape

Problem: JSON and text output behavior is documented in prose, but there is no
single reference for scripts that need field-level contracts.

Suggested work:

- Decide whether `docs/usage.md` is enough for output contracts, or whether a
  dedicated `docs/reference.md` should hold JSON shapes and status values.
- If a reference file is added, link to it from usage and keep spec limited to
  stdout/stderr and automation-safety principles.

### Documentation Ownership Check

Problem: contributors need a low-friction way to decide which document to edit.

Suggested work:

- Add a small documentation checklist to PR review notes or contributor docs:
  product invariant -> spec, user command/output detail -> usage, agent shortcut
  -> skill, future idea -> follow-up document.
- Consider a lightweight docs lint that flags command examples added to
  `docs/spec.md`.

## Tooling

### Command Reference Drift

Problem: `docs/usage.md`, shell completion specs, and command help can drift as
flags and subcommands change.

Suggested work:

- Generate or lint the quick reference against the command/completion specs.
- Add a narrow docs check that fails when public flags exist in code but are
  missing from `docs/usage.md`.
- Keep exact command syntax out of `docs/spec.md`; usage remains the user-facing
  reference.

## Implementation

### ACME Implementation Boundary

Problem: public docs no longer expose ACME as a supported configuration option,
but ACME-related implementation files still exist under `internal/tlsprov`.

Suggested work:

- Decide whether ACME should be removed, kept as dormant experimental code, or
  moved behind an explicitly private future-work boundary.
- If retained, document the internal-only status in developer docs without
  presenting it as a public feature.
- Add a public-docs check that rejects `acme`, `acme_dns`, and `tlsprov`
  references outside intentional developer/internal contexts.
