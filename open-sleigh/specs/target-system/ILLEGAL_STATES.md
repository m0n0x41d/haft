---
title: "6. Illegal States"
purpose: States that must be IMPOSSIBLE to represent, with precise enforcement labels
principle: '"Make illegal states unrepresentable" — but be honest about the enforcement mechanism'
reading_order: 6
---

# Open-Sleigh: Illegal States Catalogue

## Coverage status legend

| Symbol | Meaning |
|---|---|
| ✅ | **Enforced.** The named mechanism, if implemented faithfully, makes the illegal state unrepresentable / unreachable in the live system. |
| ⚠️ | **Partial.** Rule enforced in primary path but a secondary path exists (silently clamped instead of rejected; guarded in one call site but not another); ticketed for tightening. |
| ❌ | **Pending.** Subsystem not yet in the tree. Listed as design intent; lights up when subsystem lands. |

All rows are **pending** until code exists; status reflects the correctness
of the **proposed mechanism**, not code-level verification.

## Four-label enforcement taxonomy (v0.5)

Every row names exactly one of the following mechanism classes. The
previous version of this doc used "type-level impossibility" as a
catch-all; 5.4 Pro flagged this as overclaim. Be precise.

| Label | What it means | Bypass profile |
|---|---|---|
| **type-level** | Pattern-match exhaustive, sum-type closed, or function-clause dispatch on a closed atom set. The compiler (via pattern-match warnings + `@spec` + Dialyzer) catches violations. | Practically none in idiomatic use. Exotic `apply/3` with dynamic atoms could bypass — banned by Credo rule. |
| **constructor-level** | `@enforce_keys` + constructor-only module API. The module does not expose a setter; callers must go through `new/_` which validates. | Direct struct literal `%Foo{...}` bypasses `new/_`. Forbidden by Credo rule `OpenSleigh.Credo.NoDirectStructLiteral` on all L1 types; caught at CI. Not a type-system wall. |
| **runtime-guard** | Runtime check in the function body; returns `{:error, _}` or raises for programmer-error. | Bypass requires editing the function or calling a lower-level primitive. Testable via negative cases. |
| **CI-or-module-graph-check** | Enforced at build time by `mix xref`, Credo rule, or CI assertion over the source. | Bypass requires editing the check itself — visible in `git diff`, caught by PR review. |

The four labels exist to make a single promise honest: we say what kind of
wall prevents the illegal state, not just that one exists.

---

## Capability-leak invariants (CL — Layer 4)

States where an agent executes a tool not in its phase's scoped set, or
writes outside its allowed workspace.

### Tool scoping

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| CL1 | ❌ | Executor agent invoking a tool unknown to the declared `Agent.Adapter` (e.g. a tool atom not in the adapter's `@tool_registry`) | **type-level** | Function-clause dispatch on `Agent.Adapter.dispatch_tool/3`; unknown atom → `FunctionClauseError` (programmer error, not a quiet `:error`) |
| CL2 | ❌ | Executor agent invoking a tool known to the adapter but NOT in the active `PhaseConfig.tools` (e.g. `:haft_decision` in `:execute` phase) | **runtime-guard** | `MapSet.member?(session.scoped_tools, tool)` check before dispatch; violation returns `{:error, :tool_forbidden_by_phase_scope}`. This is runtime-enforced because `sleigh.md` hot-reload can change scope without recompile. |
| CL3 | ❌ | Frame-phase agent invoking `haft_problem(frame)` (i.e. attempting to author a ProblemCard) | **runtime-guard** | Frame's `PhaseConfig.tools` deliberately excludes `haft_problem`; any dispatch attempt triggers `:tool_forbidden_by_phase_scope` per CL2. The agent cannot author upstream framing from inside the harness (v0.5 framing-ownership lock). |
| CL4 | ❌ | Adapter dispatcher accepting a tool name string instead of atom | **type-level** | `dispatch_tool/3` takes `atom()` in its `@spec`; strings fail pattern match. |

### Workspace-path safety — bypass test matrix

Per v0.5 P5 guardrail hardening, the workspace-path allowlist is
specified in `FUNCTIONAL_ARCHITECTURE.md §LAYER 4 Inexpressible` with an
explicit algorithm (canonical path + recursive readlink + inode compare
+ `.git` remote-URL check). Each bypass route below has a specific
negative test.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| CL5 | ❌ | Adapter writing to an absolute path outside `workspace_path` via `..` traversal | **runtime-guard** | `PathGuard.canonical/1` applies `Path.expand/1` before prefix check; traversal resolves out of workspace, rejected as `:path_outside_workspace`. Negative test: adapter attempts write to `Path.expand("../../etc/hosts", workspace_path)`. |
| CL6 | ❌ | Adapter writing via a symlink that points outside `workspace_path` | **runtime-guard** | `PathGuard.canonical/1` dereferences symlinks recursively (max depth 8) before prefix check; the real path is compared, not the lexical path. Negative test: symlink tree `workspace/evil -> /tmp/target` rejected as `:path_symlink_escape`. |
| CL7 | ❌ | Adapter writing via a hardlink to a file inside `open_sleigh/` | **runtime-guard** | `PathGuard.canonical/1` also stat's the target and inode-compares against the forbidden tree's inodes (maintained in ETS); hardlink-alias rejected as `:path_hardlink_escape`. Negative test: `ln` creates hardlink from `workspace/file` to `open_sleigh/lib/foo.ex`; write attempt rejected. |
| CL8 | ❌ | Adapter running shell that resolves symlink recursion to loop indefinitely | **runtime-guard** | `PathGuard.canonical/1` caps readlink recursion at depth 8; loop returns `:path_symlink_loop`. Negative test: `a -> b -> a` cycle. |
| CL9 | ❌ | Workspace is a `git clone` of the Open-Sleigh repository (bypasses allowlist by making the harness source *live inside* the allowed path) | **runtime-guard** | `Session.new/1` inspects `.git/config` inside `workspace_path`; if `remote.origin.url` matches the Open-Sleigh canonical remote, session construction fails `:workspace_is_self`. Negative test: `git clone open-sleigh /tmp/work; session with workspace_path=/tmp/work` rejected. |
| CL10 | ❌ | Adapter `:bash` tool (where allowed) runs command that `cd`s out of workspace | **runtime-guard** | `:bash` dispatch wrapper sets `$PWD` to `workspace_path` and rewrites absolute-path arguments; absolute paths outside workspace fail before exec. Negative test: `bash -c 'cd / && touch /tmp/evil'` from within Executor phase. |
| CL11 | ❌ | Adapter uses Elixir `File.*` API directly instead of `PathGuard` wrapper | **CI-or-module-graph-check** | Credo rule `OpenSleigh.Credo.NoDirectFilesystemIO`: every L4 adapter module's filesystem calls must go through `OpenSleigh.Adapter.PathGuard`. `mix credo --strict` fails on direct `File.write!/1` in adapter code. |
| CL12 | ❌ | Adapter writes to a file whose parent directory is inside workspace but the file was created by `mount --bind` to a forbidden tree | **runtime-guard** + **known-weakness** | `PathGuard` does not detect bind-mounts across different filesystems. Documented as a weakness; realistic bypass only on a compromised host. Negative test covers common cases; this one is *acknowledged* rather than prevented. |

---

## Provenance invariants (PR — Layer 1 core types)

States where a `PhaseOutcome` or `Evidence` is constructed without its
provenance fields.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| PR1 | ❌ | `PhaseOutcome` without `config_hash` | **constructor-level** | `PhaseOutcome.new/2` signature has `config_hash` in the required provenance map; `@enforce_keys [:config_hash, :valid_until, :authoring_role, :self_id, :gate_results, :evidence, :phase_config]` on the struct. Direct literal bypass caught by Credo. |
| PR2 | ❌ | `PhaseOutcome` without `valid_until` | **constructor-level** | Same as PR1. |
| PR3 | ❌ | `PhaseOutcome` without `authoring_role` | **constructor-level** | Same as PR1. |
| PR4 | ❌ | `PhaseOutcome` with `rationale` > 1000 chars | **constructor-level** | Validated in `PhaseOutcome.new/2`; truncation is NEVER silent — returns `{:error, :rationale_too_long}`. |
| PR5 | ❌ | `Evidence.ref == PhaseOutcome.self_id` (self-referential evidence) | **constructor-level at `PhaseOutcome.new/2`** (v0.5 fix to 5.4 Pro CRITICAL-2) | The check lives on `PhaseOutcome.new/2`, NOT on `Evidence.new/5`, because evidence in isolation has no reference to the authoring artifact's id. `PhaseOutcome.new/2` iterates its `evidence` list and rejects any element whose `ref == self_id`. |
| PR6 | ❌ | `Evidence` with `cl` outside `0..3` | **constructor-level** | `Evidence.new/5` validates `cl in 0..3`; `@spec` documents the range. |
| PR7 | ❌ | `Evidence` without `authoring_source` | **constructor-level** | `@enforce_keys [:kind, :ref, :cl, :authoring_source, :captured_at]` on the struct. |
| PR8 | ❌ | `HumanGateApproval` with `approver` not in the active `SleighConfig.approvers` | **runtime-guard at `HumanGateListener`** | `HumanGateListener` validates approver against the session's frozen `config_hash` approvers list; mismatch returns `{:error, :approver_not_authorised_for_config_hash}`. |
| PR9 | ❌ | `HumanGateApproval` without `config_hash` pinned at approval time | **constructor-level** | `@enforce_keys [:approver, :at, :config_hash, :signal_source, :signal_ref]`. |
| PR10 | ❌ | `PhaseOutcome` from a phase whose `PhaseConfig` declares a human gate, but `gate_results` contains no approved `{:human, HumanGateApproval.t()}` entry | **constructor-level** (v0.5 Q-OS-3 resolution — replaces v0.4's proposed `new_external/3` path) | `PhaseOutcome.new/2` checks gate-config consistency: for each `gate_name` in `phase_config.gates.human` (`publish_approved`, `one_way_door_approved`, etc.), at least one matching approved `{:human, approval}` must be in `gate_results`. Missing match fails `:human_gate_required_by_phase_config_but_missing`. Single constructor; no bifurcated path. |

---

## Phase-transition invariants (TR — Layer 3 phase machine)

States where a session's phase moves illegally.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| TR1 | ❌ | `:frame` → `:measure` (skipping Execute) in MVP-1 | **type-level** | `PhaseMachine.next/2` has no clause producing that transition; pattern-match exhaustiveness on closed sum catches missing clauses at compile time (with warnings-as-errors). |
| TR2 | ❌ | `:terminal` → any non-terminal | **type-level** | Terminal is absorbing in `Workflow.transitions`; `next/2` returns `{:terminal, verdict}` and no clause re-enters. |
| TR3 | ❌ | `WorkflowState` with two simultaneously active phases | **type-level + constructor-level** | `current :: Phase.t()` is a singleton field (not a list). `@enforce_keys` ensures it's set. |
| TR4 | ❌ | Advance while a `HumanGate` is pending | **type-level** | `:await_human` is a sticky branch; `next/2` returns `:await_human` until `HumanGateApproval` resolves. No clause advances past pending human gate. |
| TR5 | ❌ | Dynamic phase atom outside the declared workflow alphabet | **type-level** (v0.5 Q-OS-2 resolution) | `Phase.t()` is a closed sum with all MVP-1 and MVP-2 atoms pre-declared; arbitrary atoms fail at function-clause match. No `{:m2, atom()}` open variant. |
| TR6 | ❌ | Re-running a completed phase's gates on the same outcome | **runtime-guard** | Phase completion is recorded in `WorkflowState`; `next/2` with a completed outcome rejects `{:error, :already_completed}`. |
| TR7 | ❌ | Regressing to an earlier phase without a declared reason | **runtime-guard** | Regression paths in MVP-2 are explicit (e.g. `:select` → `:generate` on parity failure with a declared reason). MVP-1 has no regression — `Verdict.fail` is terminal. |

---

## Gate-kind invariants (GK — Layer 2 gate algebra)

States where gate kinds are confused or merged illegally.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| GK1 | ❌ | `GateResult.combine/1` receiving a mixed list without kind-aware combining | **type-level** | Pattern-match on kind-tag; untyped merges fail clause. |
| GK2 | ❌ | A `:semantic` `GateResult` without `cl` field | **constructor-level** | Semantic variant's payload struct has `@enforce_keys [:verdict, :cl, :rationale]`. |
| GK3 | ❌ | Invoking a gate on anything other than a `PhaseOutcome` | **type-level** | `apply/2` guards with `is_struct(arg, PhaseOutcome)`. |
| GK4 | ❌ | Gate name in `sleigh.md` resolving to neither structural nor semantic registry | **CI-or-module-graph-check** (L6 compile) | `Sleigh.Compiler.compile/1` checks every gate name against compile-time `OpenSleigh.Gates.Registry`; unknown → `{:error, :unknown_gate, name}`. |
| GK5 | ❌ | A semantic gate running without a `JudgeCalibration` / GoldenSet | **runtime-guard** | `SemanticGate.apply/3` checks `SleighConfig.judge_calibration` first; absent → `{:error, :uncalibrated}`. |
| GK6 | ❌ | Collapsing pass/fail across kinds into a single Boolean | **type-level** | `GateResult.combine/1` returns `:advance | :block | :await_human` sum — not Boolean. |

---

## Config / DSL invariants (CF — Layer 6 compiler)

States where `sleigh.md` compiles to an invalid `SleighConfig`.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| CF1 | ❌ | `sleigh.md` source > 300 lines (raw line count) | **CI-or-module-graph-check** (L6 compile) | `Sleigh.SizeBudget.check/1` runs before compile; exceeds → `{:error, :over_budget_file}`. |
| CF2 | ❌ | Any prompt template > 150 lines | **CI-or-module-graph-check** (L6 compile) | Same; `{:error, :over_budget_prompt, phase}`. |
| CF3 | ❌ | Tool name in `sleigh.md` not in the declared adapter's `@tool_registry` | **CI-or-module-graph-check** (L6 compile) | Compiler checks against adapter module attribute; `{:error, :unknown_tool, adapter, tool}`. |
| CF4 | ❌ | Phase name in `sleigh.md` not in `Workflow.phases` for the declared workflow | **CI-or-module-graph-check** (L6 compile) | Compiler checks against `Workflow.mvp1/0` or `mvp2/0` output; `{:error, :unknown_phase, name}`. |
| CF5 | ❌ | Prompt template referencing an undefined variable | **CI-or-module-graph-check** (L6 compile) | Compiler validates `{{...}}` variables against `PhaseInput.t()` schema for the phase. |
| CF6 | ❌ | `external_publication.approvers` empty with `branch_regex` non-nil | **CI-or-module-graph-check** (L6 compile) | `{:error, :missing_approvers}`. |
| CF7 | ❌ | Two sessions active on the same `(WorkCommission, phase)` pair | **runtime-guard** | `Orchestrator.handle_call({:claim, commission}, ...)` checks ETS ownership table; returns `:already_owned`. |
| CF8 | ❌ | Modifying `SleighConfig` post-compile | **constructor-level** | `SleighConfig` has no public update function; compile produces the only instance. Direct struct update `%{sc \| phases: ...}` is forbidden by Credo rule `OpenSleigh.Credo.ImmutableCompiledConfig`. |
| CF9 | ❌ | `sleigh.md` using `@include` or similar directive to pull in bloat from a side file (adversarial size-budget bypass) | **CI-or-module-graph-check** (L6 compile) | P5 hardening: `Sleigh.SizeBudget.check/1` follows any `@include` / `{{file "..."}}` style reference and counts transitively. No include directive admitted in MVP-1 syntax, but the budget check is structured to follow them if added later. |
| CF10 | ❌ | `sleigh.md` embedding base64-encoded bloat or multi-megabyte comment blocks to satisfy line count while smuggling content | **CI-or-module-graph-check** (L6 compile) | P5 hardening: budget check also asserts `byte_size(source_md) < 50 KB` as a secondary constraint; exceed → `{:error, :over_budget_bytes}`. Lines + bytes together close the numerical-gaming attack. |

---

## Session / Orchestration invariants (SE — Layer 5 OTP)

States where session state is corrupted by concurrency.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| SE1 | ❌ | Two writers mutating the same session's state | **runtime-guard + type-level** | `Orchestrator` GenServer is sole writer; `AgentWorker` sends messages only. The writer-process invariant is OTP-level runtime; the no-direct-write API is type-level (no exposed setter). |
| SE2 | ❌ | `AgentWorker` spawned without a claimed `Session.t()` | **type-level** | `Task.start_link` signature requires `Session.t()`; no sessionless constructor. |
| SE3 | ❌ | `AgentWorker` executing with `workspace_path` that resolves (canonically) to inside `open_sleigh/` or `~/.open-sleigh/` | **runtime-guard** | `Session.new/1` runs `PathGuard.canonical/1` + git-remote check (see CL5–CL9); reject at session construction. |
| SE4 | ❌ | WAL replay out of per-commission append order | **runtime-guard** | WAL is per-commission file; `Haft.Supervisor.replay/1` reads line-by-line; no reordering. Legacy tracker-first WAL remains per-ticket. |
| SE5 | ❌ | `ObservationsBus` emitting an entry that reaches the Haft artifact graph | **CI-or-module-graph-check** (see OB1) + **type-level** at `emit/3` signature | See OB category for full treatment. |
| SE6 | ❌ | Session surviving past its adapter's `wall_clock_timeout_s` | **runtime-guard** | `AgentWorker` is monitored; timeout triggers Task termination and `:cancel_grace` protocol. |
| SE7 | ❌ | `Haft.Client.write_artifact/2` invoked without `AdapterSession.config_hash` | **type-level** | Signature requires `session :: AdapterSession.t()`; no arity-1 variant exists. |

---

## Adapter / Effect invariants (AD — Layer 4)

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| AD1 | ❌ | Adapter returning an untyped error (free string) | **type-level** | `EffectError.t()` is a closed sum; `@spec` on all adapter callbacks enforces. |
| AD2 | ❌ | Adapter raising for a known failure mode (e.g. timeout) | **runtime-guard** | Adapter impls catch known failures and return `{:error, EffectError.t()}`; Credo rule forbids `raise` in adapter modules except in declared programmer-error cases. |
| AD3 | ❌ | `Haft.Client` silently dropping a write on `:haft_unavailable` | **runtime-guard** | `HaftSupervisor.with_wal/1` always writes to WAL on `:haft_unavailable`; drop requires explicit `:discard` argument (never used in production paths; Credo rule forbids `:discard` outside test code). |
| AD4 | ❌ | Adapter session cancel leaving a partial Haft write | **runtime-guard** | `:cancel_grace` protocol: in-flight Haft call completes or times out within 30s grace; compensating `haft_note(cancelled, cause, partial_refs)` always written. |
| AD5 | ❌ | `JudgeClient` returning a verdict without `cl` | **constructor-level** | Judge output wrapped into `SemanticGateResult.t()` struct with `@enforce_keys [:verdict, :cl, :rationale]`. |
| AD6 | ❌ | `JudgeClient` invoking a gate that has no GoldenSet | **runtime-guard** | `JudgeClient.evaluate/2` checks `SleighConfig.judge_calibration` map first; absent → `{:error, :uncalibrated}`. |

---

## Observation-isolation invariants (OB — Thai-disaster core prevention)

The Thai-disaster attractor is "AI agent writes governance about the
system it's operating in, on the system's own evidence plane." Open-
Sleigh prevents this architecturally.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| OB1 | ❌ | `ObservationsBus` having any compile-time path to `Haft.Client` | **CI-or-module-graph-check** | `mix xref graph --source OpenSleigh.ObservationsBus` asserts zero occurrence of `OpenSleigh.Haft.Client` in the transitive call graph. CI step fails on any path. |
| OB2 | ❌ | Any L5 module writing a Haft artifact about Open-Sleigh's own operation | **CI-or-module-graph-check** | Any `Haft.Client.write_artifact/2` call whose source module is in `lib/open_sleigh/observations/` or `lib/open_sleigh/telemetry/` trees triggers `mix credo` rule `OpenSleigh.Credo.NoSelfObservation`. |
| OB3 | ❌ | `ObservationsBus.emit/3` accepting an argument typed as `Haft.ArtifactRef` | **type-level** | Signature: `value :: number() \| String.t() \| atom()`; maps not admitted. `Haft.ArtifactRef.t()` is an opaque struct; cannot be coerced into the allowed types. |
| OB4 | ❌ | A Haft artifact whose `authoring_source` atom is `:open_sleigh_self` | **constructor-level** | `Evidence.new/5` rejects `:open_sleigh_self` explicitly (reserved atom). `authoring_source` values are validated against a closed whitelist; `:open_sleigh_self` is on the blacklist. |
| OB5 | ❌ | Any module simultaneously importing (via `import`, `alias`, or `use`) BOTH `OpenSleigh.ObservationsBus` AND `OpenSleigh.Haft.Client` | **CI-or-module-graph-check** (v0.5 P5 proxy-module rule) | New in v0.5: catches the proxy-bypass scenario 5.4 Pro flagged. `mix credo` custom rule `OpenSleigh.Credo.NoObservationToHaftProxy` walks each module's `import`/`alias`/`use` headers; any module referencing both names fails CI. This is stricter than OB1 (which only catches the transitive call graph) because it forbids even holding the capability in the same module. |

---

## Continuation-turn invariants (CT — v0.6, Symphony-inherited)

Per `AGENT_PROTOCOL.md §3` continuation-turn model (SPEC.md §5.1
carries an abstract + pointer only).

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| CT1 | ❌ | Continuation turn (`turn_count ≥ 2`) sending the full rendered first-turn prompt again (re-stating the task in thread history) | **constructor-level** | `PromptBuilder.render/3` has two clauses: `turn_count == 1 → first_turn_prompt(...)`, `turn_count ≥ 2 → continuation_guidance(...)`. No code path produces the first-turn prompt with `turn_count > 1`. Credo rule forbids `first_turn_prompt/*` callers outside the `== 1` clause. |
| CT2 | ❌ | Continuation turn opening a new thread instead of reusing the session's live `thread_id` | **runtime-guard** | `Agent.Adapter.run_turn/3` takes an existing `thread :: ThreadHandle.t()` handle, not a fresh thread id. No public API exists for "run turn on a new thread within the same session." Starting a new thread requires closing the session and opening a new one (phase boundary). |
| CT3 | ❌ | `agent.max_turns` in `sleigh.md` set to `0` or negative | **CI-or-module-graph-check** (L6 compile) | `Sleigh.Compiler.compile/1` validates `agent.max_turns >= 1`; violation → `{:error, :max_turns_invalid}`. |
| CT4 | ❌ | Frame or Measure phase configured with `max_turns > 1` | **CI-or-module-graph-check** (L6 compile) | Frame and Measure are single-turn by design (§5.1); compiler rejects per-phase `max_turns` overrides > 1 for these phases; only Execute admits multi-turn. |
| CT5 | ❌ | `AgentWorker` firing a continuation turn after `PhaseOutcome` has been handed to the orchestrator (double-finalize) | **runtime-guard** | `AgentWorker`'s internal state machine enters `Finishing` sub-state after emitting `PhaseOutcome`; the loop exits. No clause re-enters `StreamingTurn` from `Finishing`. |
| CT6 | ❌ | Tool scope mutating mid-thread (e.g., phase-change forcing a tool-set swap without closing the session) | **type-level** | `AdapterSession.scoped_tools` is set at session construction and is `:const` in the struct; no setter. Phase change closes session and opens a new one with new scope. |

## Workspace hook invariants (WH — v0.6, Symphony-inherited with FPF constraints)

Per `WORKSPACE.md §2` workspace hooks. Hooks are trusted configuration
but their filesystem effects must respect `PathGuard`.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| WH1 | ❌ | Hook script writing to a path that (after canonicalisation) resolves outside `workspace_path` | **runtime-guard** | Hook execution wraps `bash -lc` with a post-hook inode sweep: any file created / modified outside `workspace_path` (detected via `find workspace_path -newer <hook_start_time>` combined with `lsof` or kernel audit) is a post-execution fail. MVP-1 honest label: **moderate** — realistically enforceable via `chroot`-like sandboxing only; documented as "trusted configuration" and the agent-authored parts (which DO get full PathGuard) are the primary wall. |
| WH2 | ❌ | Hook timing out past `hooks.timeout_ms` without being killed | **runtime-guard** | `Workspace.run_hook/3` runs via `System.cmd/3` under a `Task.await/2` with explicit timeout; timeout triggers `Process.exit(port, :kill)` and returns `:hook_timeout` error. |
| WH3 | ❌ | `after_create` hook running on workspace **reuse** (existing directory) | **runtime-guard** | `Workspace.create_for_commission/2` returns `{:ok, workspace, :new}` or `{:ok, workspace, :reused}`; hook dispatcher only runs `after_create` when result is `:new`. |
| WH4 | ❌ | `before_remove` hook running on a workspace that doesn't exist | **runtime-guard** | Same dispatcher checks `File.dir?(workspace)` before running `before_remove`. |
| WH5 | ❌ | Hook script using `$HOME` env that points inside `open_sleigh/` (e.g., operator accidentally ran engine as a user whose `$HOME` contains the harness source) | **runtime-guard** | `Session.new/1` canonicalises `$HOME` and rejects sessions where `Path.expand("~")` is inside `open_sleigh/` or `~/.open-sleigh/` parent. Same PathGuard as CL9 clone-into-workspace. |

## Token-accounting isolation invariants (TA — v0.6, extends OB for Symphony-inherited token counting)

Per `HAFT_CONTRACT.md §4` + `AGENT_PROTOCOL.md §4` token accounting.
Costs are observations, not evidence.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| TA1 | ❌ | Token count (`codex_total_tokens` etc.) being written to a Haft artifact as evidence | **CI-or-module-graph-check** | OB5 covers this at the module-graph level (no proxy bridging `ObservationsBus` to `Haft.Client`). Plus Credo rule `OpenSleigh.Credo.NoTokenCountInEvidence` scans `Evidence.new/5` callers to forbid `kind: :token_count` or `authoring_source: :token_counter`. |
| TA2 | ❌ | Double-counting tokens when adapter emits both delta (`last_token_usage`) and absolute (`thread/tokenUsage/updated`) | **runtime-guard** | `TokenAccounting.ingest/2` has clauses for each event shape; `last_token_usage` clauses explicitly discard the payload (per Symphony §13.5 + `HAFT_CONTRACT.md §4`). Absolute updates compare against `last_reported_total_tokens` and add the delta. |
| TA3 | ❌ | `codex_total_tokens_per_commission` observation exceeding a threshold **blocking** a phase advance | **type-level** | Observation indicators NEVER gate transitions (§6d). The gate chain (L2) is structurally separate from the observations bus (L5). Token counts can surface in logs/dashboards but cannot be input to `GateResult.combine/1`. Legacy dashboards may still label this per-ticket. |

## WorkCommission / Projection invariants (WC — commission-first integration)

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| WC1 | ❌ | Open-Sleigh creating or approving a WorkCommission | **CI-or-module-graph-check** | Only Haft exposes commission authoring APIs. Open-Sleigh may call list/claim/preflight/run-event operations, not create/approve. |
| WC2 | ❌ | Execute phase starting without a WorkCommission preflight lease | **runtime-guard** | Orchestrator can create Execute Session only from `start_after_preflight` success. |
| WC3 | ❌ | Execute phase starting when DecisionRecord hash/revision differs from the WorkCommission snapshot | **runtime-guard** | `decision_fresh` gate compares current Haft revision with `decision_revision_hash`; mismatch blocks as stale. |
| WC4 | ❌ | Stale/superseded/deprecated DecisionRecord treated as runnable because a commission was queued earlier | **runtime-guard** | Preflight checks DecisionRecord lifecycle every time before Execute. Commission becomes `:blocked_stale`. |
| WC5 | ❌ | External tracker issue state completing a WorkCommission without Haft evidence | **runtime-guard** | Projection observations record drift/conflict only. Completion requires Haft `complete_or_block` with evidence refs. |
| WC6 | ❌ | ProjectionWriterAgent deciding lifecycle state, severity, owner, deadline, or completion | **runtime-guard** | ProjectionIntent carries deterministic facts; ProjectionValidation rejects invented/missing forbidden claims before connector publish. |
| WC7 | ❌ | External projection credentials required for local execution | **runtime-guard** | `projection_policy: :local_only` is a valid config path; projection adapter failure cannot fail RuntimeRun evidence. |
| WC8 | ❌ | Two batch/YOLO commissions running with overlapping locksets | **runtime-guard** | Scheduler checks lockset intersection before lease grant. |
| WC9 | ❌ | YOLO AutonomyEnvelope expanding itself mid-run | **constructor-level** | AutonomyEnvelope is immutable after approval; out-of-envelope needs move commission to `:needs_human_review`. |
| WC10 | ❌ | Agent uncertainty in Preflight mapped to pass | **runtime-guard** | PreflightReport verdict `:needs_human_review` is sticky unless Haft/human resolves; validators reject pass with unresolved material changes. |
| WC11 | ❌ | RuntimeRun mutates outside WorkCommission Scope but inside `workspace_path` | **runtime-guard** | `AdapterSession.scope` is checked before every mutating adapter call; end-of-run diff validation rejects `:mutation_outside_commission_scope` terminally. |
| WC12 | ❌ | HumanGateApproval reused after CommissionRevisionSnapshot drift | **runtime-guard** | Approval carries `commission_snapshot_hash`; `start_after_preflight` / `PhaseOutcome.new/2` reject approval when current snapshot hash differs. |
| WC13 | ❌ | ImplementationPlan revision changes after lease but before Execute without revalidation | **runtime-guard** | `scope_snapshot_fresh` and `start_after_preflight` compare `implementation_plan_revision`; mismatch releases or blocks the lease. |
| WC14 | ❌ | `external_required` completes local execution with failed publication but no ProjectionDebt | **runtime-guard** | `projection_debt_recorded` gate requires explicit debt state before terminal completion when required external publish did not sync. |
| WC15 | ❌ | Base SHA / admitted repo context changes after queueing without deterministic re-preflight | **runtime-guard** | `scope_snapshot_fresh` compares `base_sha_seen` with commission snapshot; mismatch blocks before Execute. |
| WC16 | ❌ | Tracker terminal state displayed as completion without adjacent Haft evidence state | **runtime-guard** | Status/projection renderers must show external carrier state separately from WorkCommission evidence/completion state. |

## Upstream framing invariants (UP — framing-ownership lock, v0.5)

Per v0.5 P1 resolution (5.4 Pro CRITICAL-1), framing is upstream-only.
Open-Sleigh verifies; it does not author.

| # | Status | Illegal state | Label | Enforcement |
|---|---|---|---|---|
| UP1 | ❌ (**hardened from ⚠️ in v0.4**) | A WorkCommission without `problem_card_ref` entering `:frame` phase | **runtime-guard** at Preflight + **runtime-guard** at Frame entry gate `problem_card_ref_present` | Commission-first mode rejects a WorkCommission with nil/invalid `problem_card_ref`; legacy tracker-first mode rejects a Ticket with nil `problem_card_ref`. **No fallback authoring path exists.** Agent's Frame `PhaseConfig.tools` excludes `haft_problem`, making the authoring tool unreachable from Frame (CL3). |
| UP2 | ❌ | Open-Sleigh emitting a Haft artifact with `authoring_role == :human` | **type-level** | `AuthoringRole` sum enumerates `:frame_verifier | :executor | :measurer | :judge | :human`. Only `HumanGateListener` constructs artifacts with `:human`; it's the sole module that takes an external approval signal as input and cannot be invoked from agent-adapter code paths (enforced by Credo rule `OpenSleigh.Credo.HumanRoleOwnership`). |
| UP3 | ❌ | A `ProblemCardRef` pointing to a Haft artifact authored by Open-Sleigh itself | **runtime-guard** | At Frame entry, the `problem_card_ref_present` gate calls `haft_query(related, artifact_id)` and rejects if `authoring_source == :open_sleigh_self` (cross-validates OB4). |

---

## Cross-cutting: what the catalogue refuses by construction

Beyond specific numbered states, whole classes of failure are refused:

- **Governance about governance.** No artifact authored by Open-Sleigh can describe Open-Sleigh. Covered by OB1–OB5.
- **Unbounded prose.** No narrative field of any L1 type exceeds 1000 chars. PR4, plus bounds on `HumanGateApproval.reason` (500), judge `rationale` (1000). Evidence `kind` is an atom, not a string.
- **Cross-phase state bleed.** Configs are hash-pinned per session (CF7, SE7).
- **Single-writer discipline.** Only `Orchestrator` mutates session state (SE1). Only `Sleigh.Compiler` produces `SleighConfig` (CF8). Only `HumanGateListener` produces `HumanGateApproval` (PR8, UP2).
- **Upstream framing.** Open-Sleigh verifies framing; it never authors it (UP1, UP2, UP3, CL3).

---

## Guardrail strength ratings (honest, v0.5)

The 5.4 Pro review asked for honest strong/moderate/weak ratings on the
four Thai-disaster guardrails. Here they are with the bypass classes that
remain open (ack'd rather than hidden):

| Guardrail | Mechanism | Rating | Acknowledged residual bypass |
|---|---|---|---|
| Workspace-path allowlist | `PathGuard.canonical/1` — `Path.expand` + recursive readlink + inode compare + `.git` remote-URL check (CL5–CL11) | **Strong** (was "Moderate" in v0.4) | Bind-mount across filesystems (CL12) — acknowledged, not prevented. Host-compromise assumption outside Open-Sleigh's threat model. |
| Bounded prose | PR4 `rationale ≤ 1000` + bounds on all narrative fields + closed `Evidence.kind` atom set | **Moderate** | Sharding prose across multiple artifacts each ≤ 1000. Mitigated by `judge_false_pos_rate` on `lade_quadrants_split_ok` catching sharded obligation-language. Not structurally prevented. |
| `sleigh.md` size budget | CF1 (lines) + CF10 (bytes) together; CF9 handles transitive includes if added | **Strong** (was "Moderate" in v0.4) | Base64-encoded bloat within the 50 KB byte cap, or Unicode-dense text packing more semantics per line. Edge-case, caught by `judge_false_pos_rate` on prompt quality. |
| Observation isolation | OB1 (transitive graph) + OB5 (direct proxy rule) + OB3 (type-narrowing on emit/3) | **Strong** (was "Weak" in v0.4) | Indirect coupling via a third system (e.g. the tracker comment channel carrying observation data to Haft). Not structurally prevented at Open-Sleigh's boundary; requires external-system cooperation. |

**Rating method.** "Strong" = primary and secondary bypass classes closed
by the listed mechanisms; residual bypass requires threat-model expansion
(host compromise, cross-system collusion) that's out of scope for this
guardrail. "Moderate" = primary bypass closed, secondary bypass class
open but observability captures the signal. "Weak" = primary bypass open.

---

## Total catalogue (v0.6)

| Category | Count | Type-level | Constructor-level | Runtime-guard | CI/module-graph |
|---|---|---|---|---|---|
| CL (Capability Leak) | 12 | 2 | 0 | 9 | 1 |
| PR (Provenance) | 10 | 0 | 9 | 1 | 0 |
| TR (Transition) | 7 | 5 | 0 | 2 | 0 |
| GK (Gate Kind) | 6 | 3 | 1 | 1 | 1 |
| CF (Config / DSL) | 10 | 0 | 1 | 1 | 8 |
| SE (Session) | 7 | 2 | 0 | 5 | 0 |
| AD (Adapter / Effect) | 6 | 1 | 1 | 4 | 0 |
| OB (Observation Isolation) | 5 | 1 | 1 | 0 | 3 |
| **CT (Continuation Turn)** *new v0.6* | **6** | **1** | **1** | **2** | **2** |
| **WH (Workspace Hooks)** *new v0.6* | **5** | **0** | **0** | **5** | **0** |
| **TA (Token Accounting isolation)** *new v0.6* | **3** | **1** | **0** | **1** | **1** |
| **WC (WorkCommission / Projection)** *new commission-first draft* | **16** | **0** | **1** | **14** | **1** |
| UP (Upstream Framing) | 3 | 1 | 0 | 2 | 0 |
| **Total** | **96** | **17** | **15** | **47** | **17** |

All rows are **pending** because `mix new` has not yet run. As layers
land, each row flips to ✅ conditional on the mechanism being implemented
as specified.

---

## What this catalogue is NOT

- **Not a runtime bug list.** Bugs go in Linear.
- **Not behaviour tests.** Each enforcement mechanism has a corresponding
  test (described in the `Enforcement` column), but the catalogue itself is prescriptive.
- **Not exhaustive for MVP-2.** MVP-2 will add states (CG-frame wellformedness, Pareto integrity, Parity Report freshness). Those arrive with MVP-2 spec.
- **Not a promise of Haskell-grade types.** Elixir's type system catches less than a full dependently-typed language. The four-label taxonomy exists precisely to be honest about which kind of wall stops each illegal state.
