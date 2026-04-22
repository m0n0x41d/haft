---
title: "13. sleigh.md — Operator Configuration"
description: The single operator-facing DSL; YAML front matter schema; prompt templates; per-phase config hash formula and freeze semantics.
reading_order: 13
---

# Open-Sleigh: `sleigh.md` — Operator Configuration

> **FPF note.** `sleigh.md` is the only editable operator surface at
> runtime. It is a `Description` of the engine's runtime binding. The
> `Carrier` (a file on disk) produces the `Object` (a compiled
> `SleighConfig` + per-session `config_hash`) via `Sleigh.Compiler`.
> One file, hot-reloaded, size-budgeted — the L6 compiler is the wall
> that keeps the Thai-disaster CLAUDE.md pathology structurally
> impossible in this system.

---

## 1. Example (canonical shape)

```yaml
---
engine:
  poll_interval_ms: 30000
  status_path: ~/.open-sleigh/status.json
  status_interval_ms: 5000
  log_path: ~/.open-sleigh/runtime.jsonl
  concurrency: 2
  status_http:
    enabled: false
    host: 127.0.0.1
    port: 4767

commission_source:
  kind: haft
  selector: runnable
  lease_timeout_s: 300
  plan_ref: null

projection:
  mode: local_only                         # local_only | external_optional | external_required
  targets: []                              # e.g. [{kind: linear, audience: manager}]
  writer_profile: manager_plain

legacy_tracker:
  kind: linear
  team: OCT
  active_states: [In Progress, Review]
  terminal_states: [Done, Won't Do, Closed]

agent:
  kind: codex
  version_pin: 0.14.0
  command: codex-cli --json
  max_turns: 20                        # max continuation turns per phase session (AGENT_PROTOCOL §3)
  max_tokens_per_turn: 80000
  wall_clock_timeout_s: 600
  max_retry_backoff_ms: 300000         # 5 min cap (Symphony §8.4)
  max_concurrent_agents: 2
  max_concurrent_agents_by_state:      # legacy tracker-first cap; commission-first uses locksets/envelope
    "In Progress": 2
    "Review": 1

workspace:
  root: ~/.open-sleigh/workspaces
  cleanup_policy: keep                 # non-destructive default; delete policies need approval

codex:
  approval_policy: auto_approve_in_session  # trusted env; walls are at phase boundaries
  thread_sandbox: workspace-write
  turn_sandbox_policy:
    type: workspaceWrite
  read_timeout_ms: 5000                # handshake read (AGENT_PROTOCOL §1)
  turn_timeout_ms: 3600000             # 1h total turn bound
  stall_timeout_ms: 300000             # 5min inactivity bound (AGENT_PROTOCOL §8)

judge:
  kind: codex
  adapter_version: mvp1-judge
  max_tokens_per_turn: 4000
  wall_clock_timeout_s: 120

hooks:
  timeout_ms: 60000
  failure_policy:
    after_create: blocking
    before_run: blocking
    after_run: warning
  after_create: |                       # runs once when workspace first created
    git clone --depth 1 $REPO_URL .
    mix deps.get || true
  before_run: |                         # runs before each agent turn loop start
    git pull --ff-only origin main || true
  after_run: null                       # optional; logged+ignored on failure
  before_remove: null                   # optional; for terminal cleanup

haft:
  command: haft serve
  version: ">= 0.8.0"

external_publication:
  branch_regex: "^(main|master|release/.*)$"
  external_transition_to: ["Done"]
  approvers: ["ivan@weareocta.com"]
  timeout_h: 24

phases:
  preflight:
    agent_role: preflight_checker
    tools: [haft_query, read, grep, bash]
    gates:
      structural:
        - commission_runnable
        - decision_fresh
        - scope_snapshot_fresh
        - lockset_available
        - autonomy_envelope_allows
      semantic: [context_material_change_review]
  frame:
    agent_role: frame_verifier     # Verifier role — does NOT author ProblemCards.
    tools: [haft_query, read, grep]  # haft_problem intentionally absent: Frame
                                      # does not write to the Haft problem graph.
    gates:
      structural:
        - problem_card_ref_present       # Hard prereq at Frame entry (v0.5).
        - described_entity_field_present  # On the upstream ProblemCard.
        - valid_until_field_present
      semantic: [object_of_talk_is_specific]
  execute:
    agent_role: executor
    tools: [read, write, edit, bash, haft_note]
    gates:
      structural: [design_runtime_split_ok, mutation_within_commission_scope]
      semantic: [lade_quadrants_split_ok]
  measure:
    agent_role: measurer
    tools: [haft_decision, haft_refresh]
    gates:
      structural:
        - evidence_ref_not_self
        - valid_until_field_present
        - mutation_within_commission_scope
        - projection_debt_recorded
      semantic: [no_self_evidence_semantic]
---

# Prompt templates

## Frame
You are the Frame verifier. Given WorkCommission {{commission.id}} and its
linked ProblemCardRef {{commission.problem_card_ref}}, verify that the upstream
ProblemCard is present, fresh (`valid_until` in the future), and
sufficiently specific. Do NOT author a new ProblemCard — if missing or
vague, exit with fail verdict so the human can frame it upstream.

## Execute
You are the Executor. Given the upstream ProblemCard {{problem_card.id}} ...

## Measure
You are the Measurer. Given the closed PR ...
```

## 2. Config hash-pinning (prompt provenance)

When a `WorkCommission × Phase` session starts, `WorkflowStore` resolves the effective
`sleigh.md` content and computes a **phase-scoped** hash:

```
config_hash = sha256(
  engine
  || commission_source
  || projection
  || legacy_tracker
  || agent
  || judge
  || haft
  || workspace
  || external_publication
  || phases[this_phase]      — only the section for this phase
  || prompts[this_phase]      — only the prompt template for this phase
)
```

Changes to `phases.measure.*` do **not** re-pin in-flight `Frame` sessions —
that's v0.2 scope-creep, corrected in v0.3. Cross-phase sections
(`engine`, `commission_source`, `projection`, `legacy_tracker`, `agent`, `codex`, `judge`, `hooks`, `haft`,
`workspace`, `external_publication`) affect all sessions and are included
in every hash.

The hash is **frozen** for the lifetime of the session. The hash is:

1. Included in every prompt sent to the agent (as a `<!-- config_hash:
   abc123 -->` trailer line).
2. Attached to every Haft artifact produced in that session (via the
   `config_hash` field on `haft_*` calls that accept metadata, or as
   `haft_note`-attached evidence otherwise).
3. Stored on the `AgentWorker`'s process state.

Hot-reload applies only to **new** sessions. In-flight sessions continue on
the pinned hash. This makes every artifact traceable to the exact prompt +
config that produced it — the missing provenance from v0.1.

## 3. Size budget (Thai-disaster guardrail)

Per `../enabling-system/FUNCTIONAL_ARCHITECTURE.md §Thai-disaster
architectural guardrails`:

- **≤ 300 lines** total for the whole file (`CF1`).
- **≤ 150 lines** per prompt template (`CF2`).
- **≤ 50 KB** bytes as a secondary constraint to prevent base64
  / Unicode-dense smuggling (`CF10`, v0.5 hardening).
- Transitive include-directive following if any such directive is ever
  added (`CF9`, v0.5 hardening).

Exceeding any → L6 compile fails. This is the mechanised prevention of
the CLAUDE.md-pathology.

## 4. Immutable compiled config

Post-compile, `SleighConfig` cannot be mutated. Hot-reload replaces the
struct atomically in `WorkflowStore`. Direct struct-update syntax on
`SleighConfig` is forbidden by Credo rule
`OpenSleigh.Credo.ImmutableCompiledConfig` (`ILLEGAL_STATES.md` CF8).

---

## See also

- [TARGET_SYSTEM_MODEL.md](TARGET_SYSTEM_MODEL.md) — `SleighConfig`, `PhaseConfig`, `ConfigHash` entity definitions
- [ILLEGAL_STATES.md](ILLEGAL_STATES.md) — CF category (config / DSL invariants), CT3/CT4 (`max_turns` validation)
- [GATES.md](GATES.md) — gate names referenced in `phases.*.gates`
- [AGENT_PROTOCOL.md](AGENT_PROTOCOL.md) — §1 timeouts, §3 `max_turns`, §8 `stall_timeout_ms`
- [WORKSPACE.md](WORKSPACE.md) — hook kinds and trust posture
- [../enabling-system/FUNCTIONAL_ARCHITECTURE.md](../enabling-system/FUNCTIONAL_ARCHITECTURE.md) — L6 compiler / size-budget enforcement
- [ADAPTER_PARITY.md](ADAPTER_PARITY.md) — `agent.version_pin` discipline across adapters
