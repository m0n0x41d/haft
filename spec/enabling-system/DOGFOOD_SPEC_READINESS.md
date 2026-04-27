# Dogfood Spec Readiness State

This note records the current Haft repository dogfood state. It is not a
`yaml spec-section` carrier and does not create active target-system or
enabling-system authority.

As of 2026-04-26, this repository has local `.haft/specs/*` carriers, but the
root `.gitignore` ignores `.haft/`. Edits to those local carriers are therefore
not captured in a normal repository patch. The local carriers are generated
draft placeholders from `internal/project/spec_carriers.go`:

- `.haft/specs/target-system.md` has one draft target placeholder.
- `.haft/specs/enabling-system.md` has one draft enabling placeholder.
- `.haft/specs/term-map.md` has an empty draft `entries: []` term map.

The honest readiness state is `needs_onboard`, not ready. Operators should keep
the placeholders draft, run `haft spec check --json`, and either unignore the
reviewable `.haft/specs` carriers or continue recording dogfood state in
tracked specs/tests until the project-local carrier policy is reconciled.
