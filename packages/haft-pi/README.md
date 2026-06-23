# @haft/pi

Pi-native Haft package.

This package is a first-class Pi carrier for Haft and FPF-assisted software
engineering. It does not wrap the generic MCP adapter. The extension owns a
small NDJSON MCP bridge to `haft serve`, registers the full typed `haft_*`
tool suite natively, and injects a compact, kernel-budgeted Haft governor
block into Pi's `before_agent_start` system prompt hook.

## What It Provides

- Native Pi tools (full kernel suite): `haft_query`, `haft_problem`,
  `haft_solution`, `haft_decision`, `haft_note`, `haft_refresh`,
  `haft_method`, `haft_commission`, `haft_spec_section`
- Prompt templates: `/h-status`, `/h-reason`, `/h-frame`, `/h-explore`,
  `/h-compare`, `/h-verify`, `/h-note`, `/h-method`, and the manual gates
  `/h-decide`, `/h-commission`
- Agent Skills: `h-status`, `h-method` (MethodPack loop), `fpf-development`
  (development-for-the-developed discipline), `fpf-semiotics` (semiotic
  discipline for project artifacts)
- Startup prompt governor that resolves the nearest `.haft` directory and
  reads the kernel's compact governor projection
  (`haft_query(action="status", view="governor")`), falling back to a
  client-side clip for older haft binaries
- Cockpit surfaces (D4): a session widget with overseer/decision counts and
  the open method run, plus a footer status line after every `haft_*` tool
  call — visibility only, via Pi's string-based `ctx.ui` API

Haft remains the authority. Pi is only the host carrier. The kernel,
`.haft/` artifacts, and `haft serve` validation own project state and
governance rules; the extension routes, scaffolds, and reminds — it never
binds decisions or commissions on its own (Transformer Mandate).

## Install

The package is embedded into the `haft` binary. In any project:

```sh
haft init --pi
```

This materializes the package under `.haft/pi/haft-pi` and registers it in
`.pi/settings.json` (project-local); Pi loads it after project trust. No npm
step is required — the extension's only runtime import (`typebox`) is a
Pi-bundled core package resolved via `peerDependencies`. Re-running
`haft init --pi` refreshes the materialized package, keeping carriers in
lockstep with the kernel in the same binary.

## Local Install (repo development)

From a project that uses Pi:

```sh
pi install ./packages/haft-pi
```

For project-local settings:

```sh
pi install -l ./packages/haft-pi
```

Pi package docs support local paths, npm packages, and git sources.
Project-local installs are written to `.pi/settings.json` and load after the
project is trusted.

## Runtime Requirements

- `haft` must be on `PATH`, or set `HAFT_BIN=/absolute/path/to/haft`.
- The current working directory, or one of its parents, must contain `.haft/`.
- Tools missing from an older `haft serve` are reported with a clear
  upgrade message instead of an opaque protocol error.

The bridge starts:

```sh
haft serve
```

with `HAFT_PROJECT_ROOT` set to the resolved project root.

## Development

```sh
npm run typecheck
node --experimental-strip-types scripts/smoke-bridge.ts <haft-project-root>
```

Tool parameter schemas in `extensions/haft/tools.ts` mirror the MCP
contracts served by `haft serve` and are checked against the kernel
contract-generation surfaces. Use `haft interface contract-generation --json`
or `haft_query(action="contract_generation")` to inspect the generated
fragments before changing Pi tool, skill, or prompt wording. The kernel
re-validates every call server-side; generated text, schema visibility, and
model-supplied fields are not approval receipts. The mirrors exist for
provider tool-calling ergonomics, not as a second authority.
Contract fragment: `read-only/generated text is discovery only; it is not evidence truth, gate passage, global approval, or operator authorization`.

## Boundary

Still out of scope: npm publish metadata, custom
`renderCall`/`renderResult` components (needs a pinned
`@earendil-works/pi-tui` dependency and a live Pi session to verify theme
APIs), active-tool lane switching (D3), and provider-payload interception
(D5 — lab-only per `sol-20260610-92b4e846`).
