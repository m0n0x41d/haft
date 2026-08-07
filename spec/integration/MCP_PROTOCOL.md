# MCP Integration

> How AI agents interact with Haft via MCP (Model Context Protocol).

## Stability Contract

The v9 public discovery surface contains the twelve tool names below.
`tools/list` is atomic: a host sees the whole catalog in one response. The
task-level onboarding, entity, and typed-memory recovery surfaces stay
advertised before project-profile onboarding completes; their
handlers return a closed recovery result instead of disappearing.

Within the v9 minor line:

- listed public tool names are not silently renamed or removed;
- documented task-level required inputs and result kinds remain compatible;
- return shapes may grow through additive optional fields;
- a breaking contract change requires an explicit migration and release act.

This is not a blanket stability promise for every diagnostic, migration, or
compatibility action currently visible in a generated action enum. The live
kernel schema is the exact call-shape authority; schema visibility is not
operator authorization, evidence truth, installed-runtime proof, or a claim
that every expert action belongs to ordinary agent UX.

## Tool Surface

| Tool | Public role | Contract posture |
|------|-------------|------------------|
| `haft_note` | Record a non-binding project fact | Task-level |
| `haft_problem` | Frame or update an engineering problem | Task-level |
| `haft_solution` | Explore or compare solution variants | Task-level |
| `haft_decision` | Inspect decision lifecycle and submit decision proposals; binding remains human-gated | Task-level with binding gate |
| `haft_refresh` | Inspect or update artifact lifecycle under the action's authority boundary | Task-level |
| `haft_query` | Read project state, source-native FPF, fused code graph, exact artifact relations, and typed-memory `resolve` / `neighborhood` / `recall` | Task-level read surface |
| `haft_method` | Pull, inspect, and close task-local SWE MethodRuns | Task-level engineering method |
| `haft_commission` | Inspect and manage WorkCommission lifecycle; authority creation remains human-gated | Task-level with authority gate |
| `haft_spec_section` | Read and manage typed SpecSection lifecycle; FPF source fit is separate and lifecycle acts stay human-gated | Task-level with lifecycle gates |
| `haft_onboard` | Read setup status or prepare a non-binding project-profile review; `haft init` installs default project memory automatically | Task-level setup |
| `haft_entity` | Proactively establish one minimum non-binding EntityOfConcern for an explicit save request or a concrete operator-named or agent-inferred receiving use | Task-level identity |
| `haft_memory` | Validate or admit a raw typed `MemoryChangeSet` through a nested `request` envelope; ordinary EntityOfConcern creation uses `haft_entity` | Expert low-level surface |

The normal project-memory path is:

1. run `haft init`; default project memory is installed without an enable/defer
   choice or any user-visible schema selection;
2. use `haft_onboard(action="status")` when setup readiness is unknown;
3. resolve identity through
   `haft_query(action="memory", memory_request={"mode":"resolve", ...})`;
4. treat `known_absent` as an identity result, not persistence permission;
5. for an explicit save request or a concrete receiving use recognized from
   current Work, whether operator-named or agent-inferred, call
   `haft_entity(action="establish", ...)`;
6. follow the returned exact `next_read`, preserving conflict, restart,
   rejection, and unknown-outcome result kinds.

The operator does not need to pre-name that receiving use or separately permit
the minimum non-binding establishment. `known_absent`, an empty graph, and
generic possible future usefulness are insufficient by themselves. Binding
decisions, commissions, specification lifecycle, and evidence truth remain
human-gated.

Users and ordinary agents do not choose, declare, or need to understand an
internal memory schema. Raw `haft_memory`
validate/admit is an expert diagnostic or implementation surface and never
admits automatically.

## Host Agents

v9 product support targets Claude Code and Codex. Bare `haft init` opens a
zero-preselection interactive host picker only in a TTY; scripts and CI name
their host explicitly. Other installable hosts remain experimental or legacy:
protocol compatibility is not stable-host parity.

Supported:

| Agent | Config location | Init flag |
|-------|----------------|-----------|
| Claude Code | `.mcp.json` | `--claude` |
| Codex CLI / App | `.codex/config.toml` | `--codex` |

Experimental/legacy:

| Agent/adapter | Init flag |
|---------------|-----------|
| Grok CLI | `--grok` |
| Pi compatibility package | `--pi` |
| Hermes | `--hermes` |
| Zed | `--zed` |
| Google Antigravity | `--agy` |
| Cursor | `--cursor` |
| Gemini CLI | `--gemini` |
| OpenCode | `--opencode` |
| JetBrains Air | `--air` |

Host adapters converge on the same Haft kernel and `.haft/` graph. Ordinary MCP
hosts run `haft serve` over JSON-RPC on stdin/stdout; Pi uses its experimental
native bridge to that server. Only Claude Code and Codex are stable-host
acceptance targets.

## Environment

| Variable | Purpose |
|----------|---------|
| `HAFT_PROJECT_ROOT` | Project directory (set by init, passed to serve) |
| `HAFT_EXPECTED_PROJECT_ID` | Guards a host config against opening a different Haft project |

## What Agents See

The MCP server supplies:

- one atomic catalog with all twelve public tool names;
- compact tool descriptions and closed JSON input schemas;
- structured result kinds with readable recovery for onboarding, identity,
  restart, conflict, rejection, and unknown commit outcomes;
- source-native FPF candidates that are not themselves applicability,
  recommendation, evidence, precedence, or authority;
- code-graph and typed-memory read results with explicit basis, coverage, and
  limitation fields;
- server instructions that route ordinary entity creation through
  `haft_entity`, keep raw memory expert-only, and preserve human gates.

Source or schema presence is not proof that one installed candidate passed P14.
RC or release status additionally requires release authority.
