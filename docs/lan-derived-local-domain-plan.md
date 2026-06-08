# LAN Derived Local Domain Plan

## Goal

Allow a service to keep its primary configured domain while LAN exposure uses a
derived `.local` alias by default.

## Scope

- Keep `gate.toml` service schema unchanged.
- Derive the LAN exposure domain from the service primary domain.
- Allow an explicit LAN domain override with `gate expose --via lan --domain`.
- Keep Cloudflared and Tailscale behavior tied to the primary service domain.
- Update `gate init` guidance so users understand the derived LAN name without
  adding new config fields.
- Update docs, completions, and tests for the new LAN behavior.

## Non-goals

- Do not add `lan`, `lan_domain`, or nested exposure fields to `gate.toml`.
- Do not implement mDNS advertisement.
- Do not edit DNS or hosts files on peer devices.
- Do not add custom domain controls for Cloudflared or Tailscale.
- Do not change the primary local routing behavior of `gate up`.

## Constraints

- LAN exposure must still require a final `.local` domain.
- Non-loopback clients must remain blocked unless a route is explicitly exposed.
- The derived LAN alias must not silently replace the primary route.
- Provider records must keep the primary service domain as the target.
- The implementation should reuse the existing exposure alias route machinery.

## Assumptions

- The default LAN alias rule is:
  - if the primary domain ends in `.local`, use it unchanged
  - if the primary domain ends in `.localhost`, replace `.localhost` with
    `.local`
  - otherwise append `.local`
- Examples:
  - `app.example.com` becomes `app.example.com.local`
  - `web.demo.localhost` becomes `web.demo.local`
  - `myapp.local` stays `myapp.local`
- The CLI override flag name is `--domain`.
- `--domain` is valid only with `--via lan`.

## Work Items

- [x] Add a helper that derives a LAN domain from a primary service domain.
- [x] Add `gate expose --domain` and validate that it is LAN-only.
- [x] Route LAN exposure through the derived or overridden LAN domain while
      storing the primary service domain as the exposure target.
- [x] Add conflict checks so a derived or overridden LAN alias cannot collide
      with an existing active route or exposure alias.
- [x] Preserve output and JSON semantics with `public_url` set to the LAN URL
      and `target` set to the primary service domain.
- [x] Update `gate init` user-facing guidance for the derived LAN alias rule.
- [x] Update completion metadata for `expose --domain`.
- [x] Update `docs/spec.md`, `docs/usage.md`, `README.md`, and
      `skills/gate/SKILL.md`.
- [x] Add focused unit tests for derivation, validation, alias routing, and
      provider-specific flag handling.

## Validation

- [x] `app.example.com` exposed through LAN uses
      `https://app.example.com.local -> app.example.com`.
- [x] `web.demo.localhost` exposed through LAN uses
      `https://web.demo.local -> web.demo.localhost`.
- [x] `myapp.local` exposed through LAN uses `https://myapp.local`.
- [x] `gate expose web --via lan --domain phone.local` uses the override.
- [x] `gate expose web --via lan --domain phone.example.com` exits with usage
      error.
- [x] `gate expose web --via cloudflared --domain phone.local` exits with usage
      error.
- [x] LAN alias collisions fail before route reload.
- [x] Run focused Go tests for expose/CLI behavior.
- [x] Run `just test`.
- [x] Run `just check` before PR-ready closeout.

## Risks

- Derived names can be long, especially for custom domains with many labels.
- `.local` resolution still depends on the LAN environment because gate does
  not advertise mDNS.
- Existing exposure records may not include derived aliases until the route
  table is reloaded by a new expose command or daemon reload.
- Alias conflict checks must account for both active registry routes and
  provider public hosts.
