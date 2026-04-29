# Agent Operating Contract

> Normative rules for any agent (human or AI) editing the Haft codebase.
> This is the authority file. When code and spec diverge, follow these rules.

## Authority hierarchy

1. **This contract** — operating rules, always followed
2. **spec/** — intended semantics of Haft's target and enabling systems
3. **Implementation** — current runtime behavior

When spec and implementation diverge:
- Preserve runtime safety first
- Open or annotate the semantic mismatch
- Do NOT silently "fix" docs to match legacy behavior
- Do NOT silently change runtime to match spec without explicit migration

## What may be edited

| Category | Editable by agent? | Notes |
|----------|-------------------|-------|
| `internal/` Go code | Yes | Follow architecture rules below |
| `desktop/` frontend | Yes | Follow existing component patterns |
| `spec/` documents | Yes, but flag semantic changes | New terms → add to TERM_MAP first |
| `.haft/specs/*.md` project specs | Yes, only through explicit user/project onboarding work | Treat as project-local spec carriers; keep strict section IDs and YAML blocks intact. |
| `.haft/*.md` projections | **No** | Derived from database. Never edit directly. |
| `db/migrations.go` | Yes, append only | Never modify existing migrations |
| `CHANGELOG.md` | Yes | Add to Unreleased section |

## What is derived (never edit)

- `.haft/*.md` projection files — regenerated from SQLite on every artifact write
- Derived health (maturity + freshness) — computed at query time from stored status + evidence
- R_eff — computed from evidence verdicts + CL penalties + decay
- Pareto front — computed from comparison data
- Module coverage — computed from knowledge graph queries
- SpecCoverage state — derived from spec/artifact/code/test/evidence links

## Architecture rules

- **Core** (`internal/artifact`, `internal/graph`, `internal/fpf`, `internal/reff`, `internal/codebase`, `internal/spec` when added) must NOT import `desktop/`, `internal/cli/`, or `internal/agentloop/`
- **Flow** may import Core. **Governor** may import Core + Flow. **Surfaces** may import anything below.
- Side effects only at Flow layer and above. Core is pure queries + mutations through Store interface.

## Decision boundary

- Agent may frame, explore, compare, and recommend when delegated
- Agent must **stop at Choose → Execute boundary** and wait for human confirmation
- `selected_ref` from compare is advisory — not the decision
- Exception: autonomous mode explicitly enabled for the session
- Even in autonomous mode: one-way-door decisions require confirmation
- Agents may draft TargetSystemSpec and EnablingSystemSpec sections, but the human principal approves load-bearing target-system role, boundary, acceptance, and autonomy statements.

## Verdict vocabulary

| Canonical (evidence) | Alias (measurement) | Use canonical in |
|---------------------|--------------------|-|
| `supports` | `accepted` | R_eff computation, evidence queries |
| `weakens` | `partial` | R_eff computation, evidence queries |
| `refutes` | `failed` | R_eff computation, evidence queries |
| `superseded` | — | Excluded from R_eff |

Measurement aliases appear only in `measure` action interface.

## Enforcement status of illegal states

| Enforcement | Meaning | What to do |
|-------------|---------|-----------|
| **Enforced** | Runtime rejects this state | Respect the guard |
| **Warned** | Runtime warns but allows | Do not suppress the warning |
| **Planned** | Spec says illegal, runtime allows | Implement enforcement when touching related code |
| **Deferred** | Known gap, intentionally left | Do not enforce without explicit decision |

## Legacy terms (deprecated but may appear in code/comments)

| Legacy term | Current term | Where you'll see it |
|-------------|-------------|-------------------|
| `quint-code` | `haft` | Old comments, git history, some URLs |
| `quint_*` | `haft_*` | Old MCP tool names in comments/tests |
| `/q-*` | `/h-*` | Old slash commands in comments |
| `.quint/` | `.haft/` | Old directory name |
| `~/.quint-code/` | `~/.haft/` | Old home directory |
| `/q-refresh` | `/h-verify` | Command rename |
| `parity_rules` (string) | `ParityPlan` (structured) | Migration in progress |
