# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [7.0.0] — 2026-04-28

v7 promotes specs to authoritative artifacts. The product is no longer "decision governance plus task execution"; it is **project harnessability**. A repository becomes harnessable only after it carries a parseable ProjectSpecificationSet (TargetSystemSpec + EnablingSystemSpec + TermMap), and Decisions / WorkCommissions / RuntimeRuns / Evidence flow downstream as consequences of that spec. The product surface model is also clearer: one Haft Core (semantic authority) under three surfaces — Desktop Cockpit (primary human cockpit), MCP Plugin (embedded host-agent surface for Claude Code and Codex), CLI Harness (operator/runtime surface). Surfaces dispatch typed actions; they do not invent semantics.

This is a major release. v6 artifacts (decisions, problems, notes, evidence, commissions) carry forward without migration; the new spec carriers and the SpecOnboardingMethod typed flow are additive. Re-run `haft init` in existing projects to pick up the updated MCP commands and placeholder spec carriers.

### Added

- **Haft v7 specification onboarding spine** — `haft init` now creates parseable placeholder carriers for `.haft/specs/target-system.md`, `.haft/specs/enabling-system.md`, and `.haft/specs/term-map.md` without inventing active product claims. New `haft spec check` runs deterministic L0/L1/L1.5 checks over fenced `yaml spec-section` blocks, term-map entries, optional section fields, and obvious carrier/object authority confusion.
- **Derived spec coverage CLI** — `haft spec coverage` and `haft spec coverage --json` derive per-section states (`uncovered`, `reasoned`, `commissioned`, `implemented`, `verified`, `stale`) from artifact links, WorkCommissions, affected files, and evidence. Output includes `why` and `next_action` rather than a single coverage percentage.
- **Project readiness state `needs_onboard`** — Go core, Desktop Rust shell, and TypeScript UI now distinguish `ready`, `needs_init`, `needs_onboard`, and `missing`. Desktop blocks generic task spawning until a project is ready, while initialized projects with draft or incomplete spec carriers surface onboarding as the primary action.
- **Desktop onboarding cockpit slice** — Settings now exposes typed onboarding actions for initialized projects, including `Open Target Spec`, `Open Enabling Spec`, `Open Term Map`, `Run Spec Check`, and `Refresh Readiness`, with spec-check findings grouped by carrier row.
- **Harness readiness guard with tactical override** — `haft harness run` blocks broad execution for `needs_onboard` projects by default. Operators may pass `--tactical-override-reason` for explicit out-of-spec tactical work; the reason is recorded on selected WorkCommissions through `spec_readiness_override`.
- **`/h-commission` operator command and lifecycle actions** — plugin-mode users now have an explicit entrypoint for the DecisionRecord → WorkCommission authorization step. `/h-commission` creates/reuses WorkCommissions without starting execution and can inspect, cancel, or requeue existing commissions with explicit transition constraints. Codex installs it as an explicit-only `$h-commission` skill; starting Open-Sleigh remains a CLI/Desktop runtime boundary via `haft harness run`.
- **Packaged Open-Sleigh runtime install path** — `task install`, release archives, and `install.sh` now treat Open-Sleigh as a first-class Haft runtime under `~/.haft/runtimes/open-sleigh/current`. `haft harness run` can launch either a repo-local source runtime through `mix` or an installed release runtime through `bin/open_sleigh`, so release users do not need a local `open-sleigh/` checkout for harness runs.
- **SpecPlan draft proposal CLI** — `haft spec plan` groups uncovered or stale spec sections by document kind, spec kind, dependency signature, and affected area, then emits review-only DecisionRecord drafts. The command is explicitly read-only by default; `--accept <proposal-id>` is the one executable proposal action that creates one DecisionRecord with section refs, while `merge`, `split`, and `discard` are typed non-executable actions that report their command gap rather than silently degrading.
- **Desktop harness cockpit detail** — the Harness page now surfaces structured workspace, runtime, evidence, operator-next, and filtered tail facts for a selected WorkCommission instead of forcing operators to inspect raw JSON or logs.
- **Golden v7 E2E proof** — `internal/cli` now has a deterministic smoke that drives a temp project through `go run ./cmd/haft spec check --json`, spec coverage, spec plan, DecisionRecord, WorkCommission preparation, mock RuntimeRun lifecycle, evidence attachment, and verified SpecCoverage edges.
- **ImplementationPlan hybrid core** — new `internal/implementationplan` package models plan id, revision, decision refs, dependency edges, and locksets as pure types. It rejects cycles and impossible dependencies, and is the substrate for harness plan parsing and commission scheduling. Tests cover DAG parse, cycle rejection, dependency satisfaction, and overlapping-lockset conflict.
- **AutonomyEnvelope minimal core** — new `internal/autonomyenvelope` package introduces the first explicit envelope types: allowed repos/paths/actions/modules, concurrency and commission budgets, forbidden one-way-door actions, failure strategy, and expiry/revocation. Commission preflight reads an envelope snapshot when present, blocks out-of-envelope actions deterministically, and cannot skip freshness, scope, or evidence gates. Default behavior remains conservative when no envelope exists.
- **WorkCommission projection intent and projection debt** — new `internal/workcommission/projection.go` models `local_only`, `external_optional`, and `external_required` projection policies. `external_required` becomes an explicit `completed_with_projection_debt` block state when execution evidence passes but external publish is missing or failed; `local_only` is unaffected. Invented status, owner, date, severity, completion, scope, and promise claims are rejected at validation.
- **Spec refs propagated through DecisionRecord and WorkCommission** — spec-linked decisions now carry stable spec refs, and WorkCommissions copy the relevant spec refs and revision/snapshot facts into their commission snapshot. Commissions with missing spec refs are blocked unless an explicit tactical override is recorded; SpecCoverage derives edges from refs instead of fuzzy title matching.
- **Commission snapshot freshness gates** — preflight now blocks stale or drifted commissions before execution starts on decision revision hash, problem revision hash, base SHA, scope hash, and spec refs/revisions where available. A targeted stale-block canary covers the path; hard freshness mismatch is terminal/blocking, not advisory.
- **SpecOnboardingMethod typed core** — new `internal/project/specflow` package models the v7 onboarding flow as pure typed Go values. `Phase`, `Check`, and `WorkflowIntent` form the contract surfaces consume; the canonical phase registry covers the target-system spine (environment/role/boundary) and the enabling-system spine (architecture/work-methods/effect-boundaries/agent-policy/commission-policy/runtime-policy/evidence-policy) in one ordered chain. Each phase composes Checks (`RequireField`, `RequireStatementType`, `RequireClaimLayer`, `RequireValidUntil`, `RequireTermDefined`, `RequireGuardLocation`, `RequireBoundaryPerspectives`) so SoTA fields cannot be omitted. FPF citations stay in agent reasoning and in Core-emitted `context_for_agent` strings; carriers never carry pattern IDs.
- **`haft spec onboard` CLI subcommand** — operator-facing entry point that returns the next typed `WorkflowIntent` for the current project as plain text or `--json`. Flags `--approve`, `--rebaseline`, `--reopen` mutate the SpecSectionBaseline store; `--reason` and `--approved-by` are recorded in the audit trail.
- **`haft_spec_section` MCP tool** — embedded host-agent surface for the same typed onboarding loop, with actions `next_step`, `approve`, `rebaseline`, `reopen`. Returns the same `WorkflowIntent` / baseline-result JSON the CLI emits so plugin-mode and CLI surfaces share one shape.
- **SpecSectionBaseline + drift detection** — SQLite migration v28 adds `spec_section_baselines(project_id, section_id, hash, captured_at, approved_by)`. The `approve` transition snapshots a SHA-256 baseline of the active SpecSection; subsequent edits to the carrier surface as `spec_section_drifted` findings in `haft spec check`. Triage actions: `rebaseline` (intentional evolution; reason required), `reopen` (drop baseline; section returns to onboarding loop). Mirrors the existing decision baseline + drift pattern at the spec level.
- **Spec section staleness detection** — `haft spec check` and the new `haft_query(action="check")` action emit `spec_section_stale` findings for active SpecSections whose `valid_until` is past today. Refresh discipline now lives at the claim level, not only at evidence.
- **`haft_query(action="check")` MCP action** — plugin-mode parity for the CLI `haft check` command. Returns the unified, CI-actionable enforcement report covering decision drift + evidence decay + unassessed decisions + coverage gaps + spec drift + spec stale + spec structural in one structured response. JSON parity with `haft check --json` is enforced by a contract test.
- **`/h-onboard`, `/h-status`, `/h-verify` rewired around the new tools** — `/h-onboard` now drives onboarding through `haft_spec_section(action="next_step")` with mandatory FPF retrieval per phase via `haft_query(action="fpf", ...)` and forbids FPF citations inside `.haft/specs/*` YAML carriers. `/h-status` distinguishes overview (`status`) from CI-actionable enforcement (`check`). `/h-verify` discovery routes through `haft_query(action="check")` first; legacy `haft_refresh(action="scan")` stays for drill-down. Regression tests assert load-bearing clauses across all three prompts.
- **Readiness nudge on MCP reasoning tools** — `haft_problem(frame)`, `haft_solution(explore)`, `haft_decision(decide)`, and `haft_note` now append a soft warning to text results when the project is `needs_onboard`. Warns the operator that decisions made now cannot link to spec refs and downstream WorkCommissions / harness runs will block until specs are in place. Skipped on machine-JSON responses, on tools that already enforce readiness (haft_commission, haft_spec_section, haft_refresh, haft_query), and on `ready` / `needs_init` / `missing` projects.
- **`haft_query(action="resolve_term")` for investigation-first discipline** — new MCP action grounds an umbrella term in the project's bounded context in one round-trip: returns `term_map_entries`, `spec_section_refs` (sections whose `terms` field references the term), `artifact_mentions` (FTS-indexed past Decisions / Problems / Notes), and a typed `resolution` (`resolved` / `ambiguous` / `absent`) with a structured `next_action` hint. `/h-frame`, `/h-decide`, and `/h-note` slash prompts now require the host agent to sweep the bounded context via `resolve_term` BEFORE bouncing back to the operator on vague signals — and even then to ask one concrete question naming the candidates the resolver returned, not "what do you mean?".

### Changed

- **v7 surface model documented as Desktop Cockpit + MCP Plugin + CLI Harness over one Haft Core** — specs and README now state that Desktop is the primary human cockpit, MCP is the embedded Claude Code/Codex agent surface, and CLI Harness is the operator/runtime surface. UI buttons and plugin commands must compile to typed artifact transitions rather than free prompts.
- **v7 host-agent support narrowed** — Claude Code and Codex are the supported embedded host-agent surfaces. Cursor, Gemini CLI, JetBrains Air, and generic MCP clients are retained only as experimental or legacy protocol carriers.
- **MCP prompt guidance updated for spec-first work** — `/h-onboard`, `/h-status`, and `/h-commission` now describe target/enabling specs, term maps, readiness, commission recovery, and the plugin/runtime boundary; regression tests cover the load-bearing prompt clauses.
- **Harness operator output made compact and actionable** — `haft harness run`, `status`, `watch`, `tail`, and `result` now prefer one human line per meaningful runtime state, terminal next-action hints, evidence summaries, workspace/diff facts, and raw JSON only behind explicit JSON/debug output.
- **Long-lived desktop conversation behavior tightened** — provider/control envelopes are audit-only instead of visible chat messages, and follow-up text on terminal/checkpointed/blocked tasks routes to continuation instead of writing to a dead PTY.
- **Desktop task status normalization shared across cockpit surfaces** — raw task status values now compile through typed `TaskRunState` helpers before input capability, dashboard attention, Jobs columns, status dots, chat streaming, and implementation ladder state are derived.
- **Spec coverage now models runtime carriers** — `haft spec coverage` derives WorkCommission → RuntimeRun → evidence edges from stored runtime events, promotes implemented/verified states from real runtime and evidence signals, and only reports RuntimeRun gaps for malformed carriers instead of emitting a synthetic global unsupported-edge gap.
- **WorkCommission lifecycle semantics centralized across surfaces** — Go core, Desktop view models, CLI harness selectors, Desktop RPC, spec coverage, and Open-Sleigh now share the same lifecycle meanings. `failed` is recoverable and requires operator action, completion states satisfy dependencies, and only completed/projection-debt/cancelled/expired states are terminal.
- **ProjectSpecificationSet typed core** — `internal/project` now treats the canonical spec model (`ProjectSpecificationSet`, `SpecDocument`, `SpecSection`, `TermMapEntry`) as pure typed objects with explicit draft/active/deprecated/superseded/stale/malformed states, instead of loose maps and ad-hoc strings. Markdown remains the carrier and fenced YAML the canonical parse object.
- **Mutating Open-Sleigh tools enforce commission scope before mutation** — adapter and tool runtime paths now require the WorkCommission scope guard up front, so write/edit calls outside `allowed_paths` and writes to `forbidden_paths` are rejected before any file changes. Terminal diff validation is retained as defense-in-depth, not the first hard guard.
- **SpecSection vocabulary aligned via single source** — `project.SpecSectionValidStatementTypes`, `project.SpecSectionValidClaimLayers`, and `project.SpecSectionValidGuardLocations` are now the canonical sets consumed by both the parser-level speccheck and the SpecOnboarding method's Check vocabulary. Eliminates silent drift where `rule`/`promise`/`gate` were valid in one validator but rejected by the other.
- **`haft check` rolls up spec health into the unified exit code** — CI-facing `haft check` now reports stale / drifted / unassessed / coverage-gap findings PLUS the spec health rollup (drift + stale + structural). Single non-zero exit when any kind of debt exists; existing decision/evidence checks contribute the same way they always did.

### Fixed

- **Desktop harness IPC action shape** — Tauri commission actions now translate camelCase UI arguments into the snake_case CLI RPC payloads expected by the Go handlers, including the new `harness_tail` command.
- **Multi-turn desktop continuation cleanup** — third and later follow-ups now preserve durable conversation turns while stripping continuation control prompts and audit-only provider envelopes from both Rust seed blocks and TypeScript transcript rendering.
- **WorkCommission lifecycle action ordering** — lifecycle record/start/run/complete actions now reject out-of-order updates instead of appending impossible runtime events to queued or preflight-only commission state.
- **Project readiness false-ready projection** — Desktop readiness now combines reported status with project existence, `.haft`, and spec-carrier facts so missing or onboarding projects cannot render as runnable from an optimistic carrier status.
- **WorkCommission attention recovery hints** — queued, ready, preflighting, and running commissions that need operator attention now keep `requeue` available when recoverable; expired commissions remain limited to inspect/cancel lifecycle actions.
- **Harness workspace apply scope enforcement** — `haft harness apply`, auto-delivery, CLI result output, and Desktop harness RPC now require explicit `scope.allowed_paths`, honor `scope.forbidden_paths`, and surface typed disabled-apply reasons for forbidden, unknown, or out-of-scope workspace diffs. `affected_files` and `lockset` remain scope facts but no longer authorize mutation by themselves.
- **Open-Sleigh legacy canary compatibility** — tracker-first legacy tickets no longer fail the new commission-mutation guard, while real WorkCommission runs still require scoped material changes. The mock adapter now emits deterministic measure evidence so legacy canaries exercise the evidence path instead of timing out.
- **`haft_commission` MCP schema rejected by host LLM API** — top-level `allOf` block of conditional `if/then` requirements (commission_id required for show/requeue/cancel; reason required for requeue/cancel) passed Go-side schema construction but the host LLM API rejects top-level `allOf`/`oneOf`/`anyOf` and took the entire haft MCP server offline (`/mcp` reported `1 MCP server failed`; `/h-verify` and `/h-onboard` returned HTTP 400). Per-action conditional requirements were already enforced at the handler boundary in `internal/cli/serve_commission.go`; the schema only declares `action` as universally required now. Two regression tests prevent the same shape from creeping back into any tool: one specific to `haft_commission`, one iterating every advertised tool's `inputSchema`.
- **`haft_solution` compare variant identifier discoverability and silent set override** ([#71](https://github.com/m0n0x41d/haft/issues/71)) — the explore → compare round trip silently lost data in three independent ways:
  - **Generated variant ids never surfaced.** `materializeVariantIDs` produced `V1`/`V2`/... when callers omitted `id`, but the explore response only showed them in body prose. ChatGPT/Codex agents skipped the prose and sent free-form titles to compare, which then errored as "outside the declared compare set" with no list of correct ids. The explore response now appends a deterministic `Variants:` index (`V1 — <title>` rows) and a usage hint listing every payload field that must use those exact ids. Comparison error messages also append `; expected one of: ["V1", "V2", ...]` so the agent can self-correct without re-running explore.
  - **`parseJSONArg` swallowed JSON shape errors.** Callers used `_, _ = parseJSONArg(...)` for `dominated_variants`, `pareto_tradeoffs`, and `incomparable`. A malformed value (wrong shape, not `[]any`) produced an empty payload, and validation reported "missing variant" errors that pointed nowhere. `parseJSONArg` now returns `(present, error)` and all three call sites propagate.
  - **Caller-supplied `non_dominated_set` silently overridden when computed Pareto carried no dominance signal.** When every dimension scored with text outside the canonical ordinal vocabulary (e.g., `"medium-high"`, `"good"`) `compareDimensionValues` returned `dimensionComparisonUnresolved` for every pair and the conservative computed front collapsed to the entire compare set. The Compare path then overwrote the human's manual ranking with this noise. The new honesty fallback detects "zero comparable pairs across all dimensions" via `paretoFrontResult.comparablePairCount`, retains the caller's `non_dominated_set` as authority for explanation coverage, keeps `ComputedParetoFront` conservative (full set) for transparency, and emits a typed warning naming the indecisive dimensions. When ANY pair is comparable, the computed front continues to override the caller's set as before.

### Chore

- **Repo-root-safe frontend unit tests** — the desktop frontend package now exposes `npm --prefix desktop/frontend test` for Node contract tests over readiness, IPC argument shape, transcript filtering, and cockpit view data.
- **Dogfood spec readiness state recorded** — `spec/enabling-system/DOGFOOD_SPEC_READINESS.md` documents the current Haft-on-Haft readiness as `needs_onboard` (placeholder `.haft/specs/*` carriers, `.haft/` ignored at repo root), so dashboards do not show fake active spec authority for this repo.

## [6.2.1] — 2026-04-22

### Fixed

- **`haft init --codex` no longer installs deprecated Codex prompts** — Codex initialization now migrates the bundled `h-*` entrypoints into Codex skills under `.agents/skills`, removes stale Haft prompts from `~/.codex/prompts` when Air is not also requested, and writes `agents/openai.yaml` so only `$h-reason` allows implicit invocation. The phase/action skills (`$h-frame`, `$h-char`, `$h-explore`, `$h-compare`, `$h-decide`, `$h-view`, `$h-verify`, `$h-status`, `$h-search`, `$h-note`, `$h-onboard`, `$h-problems`) are explicit-only. Air keeps its existing prompt bootstrap path.

### Chore

- **FPF corpus/index refresh** — bumped the embedded FPF corpus snapshot and regenerated `internal/cli/fpf.db`.

## [6.2.0] — 2026-04-20

### Fixed

- **MCP `haft_solution` and `haft_problem` schemas missing `parity_plan`** ([#62](https://github.com/m0n0x41d/haft/issues/62)) — deep-mode comparison validates a structured parity plan (`baseline_set`, `window`, `budget`, `missing_data_policy` per FPF G.9:4.2) but the advertised MCP tool schema in `internal/fpf/server.go` did not expose any parameter that accepted those four keys. Deep mode was unreachable from MCP clients (Claude Code, Cursor, Gemini CLI, Codex). The standalone schema and dispatcher already handled `parity_plan` correctly — only the MCP-advertised schema was missing it. Added structured `parity_plan` object parameter to both `haft_solution(action="compare")` and `haft_problem(action="characterize")`. Two regression tests assert it stays exposed.
- **Artifact ID collisions across branches** ([#63](https://github.com/m0n0x41d/haft/issues/63)) — `GenerateID` previously rendered a per-day sequential counter (`dec-20260418-001`), so two branches creating decisions on the same day produced identical filenames in `.haft/` and the merge had to be resolved by hand. Switched to a 32-bit `crypto/rand` hex suffix (`dec-20260418-a3f7c198`). ~4.3B values per kind per day; birthday-paradox collision probability stays below 10⁻⁶ for the first few thousand IDs. The `seq` parameter is preserved for call-site compatibility but no longer rendered into the ID. Three regression tests cover the new format and uniqueness guarantees.
- **Tauri IPC shape mismatch on every mutation** — the desktop backend's `rpc_mutation!` macro declared every Tauri command as `(project_root: String, payload: Value)`, but the frontend sends flat command-specific fields (`{ path }`, `{ decision_id, reason }`, `{ input }`, …) and never passed `projectRoot` explicitly. Every user-facing UI action (switch project, dashboard load, create decision, implement, etc.) failed at Tauri's IPC argument validation before reaching the CLI subprocess. Rewrote the macros to accept each command's real field shape and made `projectRoot` optional (subprocess inherits `HAFT_PROJECT_ROOT` from the Tauri host environment). Four Tauri command registrations renamed to match the frontend (`characterize_problem`, `adopt_problem_candidate`, `dismiss_problem_candidate`, `assess_comparison_readiness`) plus new `add_project_smart`.
- **Cross-project decision recall keyed on user-supplied title** — the global decision index was storing `selected_title` as `decision_id`, so two decisions with the same chosen option in one project would collide and overwrite each other in `~/.haft/index.db`. Now uses the canonical artifact ID returned by `artifact.Decide()`. Requires plumbing `dispatchTool` to return the created artifact ref alongside the response string.
- **FPF hint map drift risk** — previous implementation hardcoded pattern IDs per phase in a Go map that could silently diverge from pattern files. Hints now generate from embedded markdown; renaming or removing a pattern ID is detected at build time via smoke test.
- **Baseline directory crash** — `hashFile()` now detects directories and returns skip error instead of attempting to read directory as file. `affected_files` containing directory paths (e.g. `src/infra/auth/`) are skipped gracefully instead of failing the entire baseline operation.
- **Test alignment** — baseline and verification tests updated to match graceful skip behavior.

### Changed

- **Desktop frontend migrated from Wails v2 to Tauri v2** — Rust shell + React 19 + TypeScript; faster launch, smaller binary, native per-platform packaging. `haft desktop` launcher finds the installed app or falls back to `desktop-tauri/target/release/bundle/` in dev.
- **`parity_plan` JSON Schema unified** — both transports now share a single `artifact.ParityPlanJSONSchema` helper instead of two parallel maps in `tools/haft.go` and `fpf/server.go`.
- **Pattern attribution cleaned up** — patterns sourced from Levenchuk material (slideument + semiotics slideument) relabeled from generic "Haft operational pattern" to specific source references (FRAME-06/07, CHR-02/06/07/08, CMP-07, EXP-07, VER-09, X-TERM-QUALITY, X-GLOSSARY, X-BITTER-LESSON).
- **FRAME-07 Goldilocks** — fabricated "10-20% beyond current capability" replaced with zone-of-proximal-development framing per slideument slide 7.
- **FRAME-08 unified with X-STATEMENT-TYPE** — question 3 of the Reading Checklist delegates to the X-STATEMENT-TYPE taxonomy (rule/promise/explanation/gate/evidence) instead of duplicating a parallel list.
- **CHR-11 source clarity** — explicit note distinguishing the slideument slide 35 didactic 5-step compression from the canonical FPF-Spec A.6.P:4 four-layer structure (Stable lens → Kind-explicit relation tokens → Slot-explicit qualified relation records → Change-class lexicon → Lexical guardrails). Each didactic step carries a canonical A.6.P:4.x reference.
- **h-reason SKILL.md trimmed** — 400 → 359 lines. Removed Concept Index (duplicated routes matchers), merged RAG search reference into FPF spec lookup, compacted Feature Maturity table into a status-keyed list. Micro-patterns preserved as direct-response floor.
- **Removed "Mandatory FPF retrieval (MUST execute before reasoning)" section** from h-reason SKILL.md — contradicted the interaction-mode protocol and doubled the auto-hint cost.
- **Hint query keywords dynamized** — per-phase example retrieval keywords now derived from the first N matchers of the corresponding `phase-*` route in `fpf-routes.json` instead of a hardcoded Go map. Matcher rename propagates to hint automatically.
- **`NextSequence` deprecated** — `GenerateID` no longer needs sequence lookup since the switch to crypto/rand suffixes. All five artifact creators (Decide, FrameProblem, ExploreSolution, CreateNote, CreateRefreshReport) now skip the wasted DB query and pass 0. The function itself stays in the `Store` and `ArtifactStore` interface for one release; planned for removal in 6.3.
- **Single `haft run` pipeline** — removed `--steps` and `--plan` as separate modes. One pipeline: Plan → Execute → Review → Baseline. `--auto` controls whether to pause.

### Added

- **Dashboard + Implement + Adopt flows (desktop)** — one proved operator loop from decision to PR. The Dashboard replaces the separate Problems / Decisions pages: active decisions with Implement button, governance findings with Adopt / Waive / Reopen buttons, and recent activity in one surface. Clicking Implement creates a worktree, spawns an agent with invariants + rationale + `.haft/workflow.md` + knowledge-graph invariants injected, verifies on completion, baselines affected files on success (CL3 evidence recorded), and generates a PR body from decision rationale. Adopt on drift / stale findings creates an agent thread with full context; agent never auto-resolves — presents options, user chooses. Resolutions recorded as RefreshReport. Implement guards: G1 blocks (multiple active decisions), G2/G4 warn (missing parity plan, subjective dimensions), no-invariants warns. Irreversible actions (Implement, Create PR, Reopen, Supersede, Deprecate) require confirmation dialogs showing affected artifacts. Governance scanner auto-refreshes on a timer.
- **`haft run` — full implementation pipeline from CLI** — reads a DecisionRecord, plans tasks via an agent, executes each with build verification, runs final invariant review, baselines on success. One command: `haft run dec-001`. Two modes: interactive (pauses between tasks) and `--auto` (full pipeline). `-c` for extra context files, `-p` for extra instructions. Task plan persisted as `.haft/plans/{ref}.md`, human-editable before execution. Per-task `go build` verification; on failure, a fix agent is spawned automatically. Final review runs invariants + build + tests; on failure, a fix agent is spawned and review re-runs.
- **Haft Design System (desktop Tauri frontend)** — lifted the design-system kit from the `haft-design-system` bundle into production: eight typed primitives (`Eyebrow`, `Button`, `Badge`, `Card`, `Input`, `StatCard`, `MonoId`, `Pill`) under `desktop/frontend/src/components/primitives/` consuming the existing Tailwind `@theme` tokens; shell components (`RailBtn`, `SidebarTask`, `StatusDot`) extracted from `App.tsx` into `components/shell/`; `ComparisonTable` component (border-first Pareto-front grid with accent highlighting and recommendation banner) replaces the legacy inline `<table>`. Dashboard, Decisions (with new `DecayWindow` progress bar computed from `created_at` + `valid_until`), Jobs, and Portfolios pages migrated to primitives.
- **`governance_mode` field on DecisionRecord** — declares whether `affected_files` are governed at the file level (`exact`) or widened to module-level scope (`module`, default — preserves pre-6.2.x behavior). Exact mode skips the silent directory inflation in baseline / drift detection. Honors FPF X-SCOPE: every claim has explicit where + under what + when.
- **FPF semiotic patterns** — 7 new patterns distilled from Levenchuk's semiotics slideument: FRAME-08 Reading Checklist (6 pre-reasoning questions), FRAME-09 Strict Distinction Quad (Role/Capability/Method/Work), CHR-10 Boundary Norm Square (L/A/D/E), CHR-11 Relational Precision Restoration Pipeline (A.6.P), CHR-12 Umbrella-word Family (quality / action / service / sameness / wholeness specializations), X-STATEMENT-TYPE (classify every load-bearing sentence), X-FANOUT-AUDIT (sweep all carriers on concept rename).
- **Compiled FPF pattern index** — 65 pattern chunks indexed alongside 4625 FPF spec chunks. Phase-keyed routes (frame / characterize / explore / compare / decide / verify / cross-cutting) in `fpf-routes.json`. 7 pattern files under `internal/fpf/patterns/`.
- **Auto-injected FPF hints in reasoning tool responses** — `haft_problem`, `haft_solution`, `haft_decision` responses include compact pattern ID citations for the current phase with retrieval guidance. Hints derive from embedded pattern files at runtime via `//go:embed` — renaming a pattern heading propagates automatically.
- **Core pattern markers** — `**Core:** true | <phase>` frontmatter in pattern markdown selectively surfaces top patterns per phase in auto-injected hints. Supports cross-phase citation (e.g. CHR-01 core in both frame and characterize).
- **FPF Micro-Patterns baseline in h-reason SKILL.md** — compressed always-in-context versions of core patterns for direct-response mode where no tool is called and hints don't inject.
- **`Valid-until` self-application on FPF pattern files** — each pattern file under `internal/fpf/patterns/` now declares a `**Valid-until:** YYYY-MM-DD` review date. New `TestPatternFilesNotPastValidUntil` fails when any pattern file is past its date, forcing periodic review of attribution, Core markers, and content currency. Initial dates set to 2026-10-18 (six-month review cadence).
- **`internal/embedding` package** — designated home for provider-bound embedding implementations. Hosts the OpenAI semantic embedder (`embedding.NewOpenAI`) extracted from `internal/fpf/semantic_embedder.go`. The fpf package now keeps only the abstract `SemanticEmbedder` interface and `SemanticEmbedderDescriptor` type — no provider, openai-go, or agent imports.
- **Transport-parity golden test** (`internal/cli/serve_parity_test.go`) — documents the action enum drift between standalone (`internal/tools/haft.go`) and MCP (`internal/cli/serve.go` switch dispatch) for each tool. Documented drift today (haft_problem.adopt, haft_decision.apply, haft_refresh.drift/reconcile, haft_query.board/list/coverage) is captured in the test's `knownTransportDrift` map; new drift fails the test. Detection layer for the unified-contract refactor planned for 6.3.
- **Layered architecture boundary tests** (`internal/artifact/core_boundary_test.go`) — replaced the desktop-only denylist with `TestPureCoreDoesNotDependOnSurfaceOrFlow` (asserts pure-Core packages — including `internal/fpf` — never import flow/surface/provider/external) and `TestEmbeddingPackageIsFlowLayerOnly` (asserts no Core package imports `internal/embedding`).
- **Cross-project recall regression tests** — verifies `haft_decision(action="decide")` returns the canonical artifact ID; two decisions with the same `selected_title` in one project now produce distinct global-index entries.
- **`haft init` smoke tests** (`internal/cli/init_smoke_test.go`) — assert MCP config production for Claude Code + Cursor stays correct: bare `haft` command (no absolute binary path), legacy `quint-code` key migrated out, idempotent re-runs, `HAFT_PROJECT_ROOT` env var plumbed through.

### Chore

- **Dead code sweep** — removed unused `loadSemanticRoutes` helper, unused `getBinaryPath` in `internal/cli/init.go`, unused `normalizeDecisionPredictions` in `internal/artifact/claim_status.go`, unused `testSteppingStoneVariant` test helper, and unused ANSI color constants `aBlue`/`aMagenta` in `internal/cli/run_ui.go`.
- **Modernize lint hints applied**: `strings.SplitSeq` in `patterns.go`, `min()` builtin replacing manual length cap, tagged switch over `Status` in `present/format.go`.
- **`task install` cleans stale GOBIN binary** — `rm -f $GOBIN/haft` runs first so PATH resolution picks up the freshly-built `BIN_DIR/haft` instead of an older `go install`-produced binary.
- **FPF spec submodule** bumped from 08e8e6f to 585938a.
- **Desktop Cargo.lock** — `haft-desktop` adds `dirs` dependency.

## [6.1.0] — 2026-04-14

### Added

- **`haft check` CLI command** — CI-friendly governance verification. Runs stale scan, drift scan, unassessed decisions, coverage gaps. Exit 0 = clean, exit 1 = findings. `--json` flag for structured output.
- **Full governance state in `/h-verify`** — scan now surfaces pending problems (backlog/in-progress count), addressed problems without linked decisions, and invariant violations from knowledge graph. Single entry point for "what needs attention."
- **`.haft/workflow.md` support** — hybrid markdown+YAML project policy file. Parsed at serve/agent startup. Intent + Defaults injected into agent prompts. `haft init` creates commented example.
- **Problem typing on ProblemCard** — `problem_type` field: optimization, diagnosis, search, synthesis. Accepted on frame, stored in DB, shown in `/h-status` and `/h-problems`.
- **Derived decision health model** — replaces single "phase" with two independent axes: Maturity (Unassessed / Pending / Shipped) and Freshness (Healthy / Stale / AT RISK). Freshness evaluated only for Shipped decisions. Never stored — computed at query time.
- **Claim-scoped evidence supersession** — new measurement supersedes only previous measurements for the same `(claim_ref, observable)`, not all measurements on the decision. Prevents unrelated evidence from being retired.
- **Claim-scoped R_eff** — `R_eff(decision) = min(R_eff(claim_i))` where each claim's R_eff is computed from its own evidence. More precise than decision-level aggregation.
- **F_eff / G_eff decomposition** — Formality (F0–F3) and Groundedness (CL-derived) exposed as view concerns alongside R_eff for evidence diagnosis.
- **Deep onboard for legacy projects** — `/h-onboard` now runs module coverage analysis and deep scans blind modules: reads code, identifies responsibilities, invariants, implicit decisions, risks. Supports parallel subagent execution when available.

### Changed

- **"No evidence = Unassessed"** — decisions without evidence are shown separately from healthy decisions, not treated as fresh. UI surfaces coverage gaps.
- **Verdict vocabulary normalized** — measurement result aliases (`accepted`/`partial`/`failed`) mapped to canonical evidence verdicts (`supports`/`weakens`/`refutes`) at storage boundary.
- **CL0 + supports = inadmissible** — evidence from opposed context with verdict `supports` is rejected at ingest, not merely penalized.
- **G1 enforced: one active decision per problem** — `Decide()` rejects if another active DecisionRecord exists for the same problem_ref.
- **G2: parity plan warnings** — `haft_solution(action="compare")` in standard/deep mode warns if parity plan is empty or unstructured.
- **G4: subjective dimension warnings** — compare warns on dimensions like "maintainable", "simple", "scalable" — asks to decompose into measurables or tag as observation-only.
- **Core boundary enforced** — integration tests verify Core packages (`internal/artifact`, `graph`, `fpf`, `reff`, `codebase`) have zero `desktop/` imports.

### Fixed

- **Desktop: oversized task output tails bounded** — prevents UI freeze on large agent outputs.
- **Knowledge graph integration tests** — FindDecisionsForFile, FindInvariantsForFile, ComputeImpactSet tested on seeded DB with real project data.

## [6.0.0] — 2026-04-13

### Breaking Changes

- **Product renamed from quint-code to Haft** — binary, MCP tools (`quint_*` → `haft_*`), slash commands (`/q-*` → `/h-*`), skill names, and docs all use `haft` naming. Existing MCP configs, skill references, and slash commands from v5.x will not work without updating.
- **Decision data model replaced** — claim-aware decision kernel with structured claims, predictions, and claim-bound evidence replaces markdown-only reconstruction. Existing decision artifacts require migration.
- **Reasoning model changed** — 5-mode activity model (Understand / Explore / Choose / Execute / Verify) replaces the artifact-centric 6-step protocol. Skill instructions, prompts, and agent behavior follow the new model.
- **`/h-verify` replaces `/h-refresh`** — `/h-refresh` is deprecated and auto-cleaned on install. Use `/h-verify` for discovery (scan + drift + pending verify_after) and triage.

Note: older changelog entries keep historical `quint-code`, `quint_*`, and `/q-*` names where they describe behavior, commands, or releases from that era.

### Added

- **Desktop app (pre-alpha)** — Wails v2 desktop application with dashboard, problem board, decision detail with evidence F/G/R decomposition, portfolio comparison with Pareto front visualization, task spawning (Claude Code / Codex), agent chat view, terminal panel (Cmd+\`), multi-project management, and search (Cmd+K). Dark theme following the design system. Pre-alpha: not recommended for production use.
- **Standalone Haft runtime** — local-first `haft agent` / TUI flow with persisted sessions, checkpointed vs autonomous execution, permission and question dialogs, model/session pickers, compaction, spawned subagents, and a typed JSON-RPC protocol between UI and runtime.
- **Knowledge graph** — `internal/graph` package providing unified query interface over existing artifact, module, and dependency tables. Queries: FindDecisionsForFile, FindInvariantsForFile, FindModuleForFile, TransitiveDependents, ComputeImpactSet. All cycle-safe with depth limiting. 17 tests.
- **Invariant injection into agent prompts** — when implementing a decision, agents receive invariants from ALL decisions governing the affected files, not just the current decision's own invariants. Invariants tagged with source decision ID.
- **Invariant verification** — automated checking of "no dependency from X to Y" and "no circular dependencies" patterns against the live module dependency graph. Returns holds/violated/unknown per invariant.
- **Governance invariant alerts** — governance scanner now runs invariant verification on decisions with drift findings, creating problem candidates for violations.
- **Probe-or-commit readiness gate** — AssessComparisonReadiness evaluates portfolio comparison quality: variant count, dimension coverage, score fill rate, constraint presence, parity plan. Returns commit/probe/widen/reroute with specific recommendations. Shown in desktop Portfolios page.
- **Evidence F/G/R decomposition** — decision detail page shows per-evidence formality level (F0-F3), congruence level (CL0-CL3), verdict badges, freshness indicators, and coverage gaps (claims without evidence).
- **Auto-run toggle for agent tasks** — per-task toggle between checkpointed (agent pauses) and auto-run (agent proceeds without intervention) modes. Persisted across app restart.
- **`haft sync` for team workflow** — syncs `.haft/` markdown files into local SQLite database after `git pull`. Enables team collaboration where `.haft/*.md` in git is the shared source of truth and each engineer has their own local database.
- **Probe-or-commit behavioral gate** — Choose mode now includes a readiness checklist before comparison: dimension coverage, variant diversity, and whether a specific next investigation could change the ranking. Returns commit / probe / widen / reroute.
- **Language precision triggers** — Understand and Choose modes catch ambiguous terms (service, process, quality, component) and subjective comparison dimensions (maintainable, simple, scalable) before they corrupt downstream reasoning.
- **`verify_after` field on claims** — `DecisionClaim` and `PredictionInput` now accept `verify_after` (RFC3339 or YYYY-MM-DD). Claims with past verify_after dates that remain unverified are surfaced by `haft_refresh(scan)` as `pending_verification` stale items with observable and threshold details. MCP schema updated.
- **Constraint-aware Pareto computation** — `computeParetoFront()` now eliminates variants that are strictly worst on all comparable peers for any constraint dimension before dominance computation. Constraint violations are reported as warnings. Variants with missing constraint data are preserved conservatively.
- **Standalone agent refresh tool parity** — `HaftRefreshTool` now exposes all 6 actions (scan, drift, waive, reopen, supersede, deprecate) matching the MCP server schema. Previously only scan/drift were available to the standalone agent.
- **Explicit reroute map** — legitimate upstream transitions documented: Choose → Understand (comparison reveals bad framing), Explore → Understand (wrong problem type), Execute → Choose, Verify → any earlier mode.
- **Claim-aware decision kernel** — decisions now persist canonical structured claims, predictions, claim-bound evidence, live measurement status, and deterministic Pareto/coverage state instead of relying on markdown-only reconstruction.
- **Deterministic projections** — projection views now render the same artifact graph for different audiences, including engineer, manager, audit, compare, delegated-agent brief, and change-rationale handoff surfaces.
- **Route-aware FPF retrieval** — indexed section summaries, route expansion, explain/full controls, golden-query evaluation, tree drill-down, and experimental semantic retrieval over the embedded FPF corpus.
- **Broader codebase awareness** — C/C++ module and include detection, symbol hashing, richer module/dependency scanning, and module-governance reporting in status/coverage flows.
- **Expanded client integrations** — `haft init` now installs MCP/command surfaces for Claude Code, Cursor, Gemini CLI, Codex CLI/App, and Air while keeping the same local binary/runtime.
- **`haft_problem(action="close")`** — marks a ProblemCard as `addressed`. Previously required manual frontmatter editing. Exposed in MCP schema for both plugin and standalone modes. ([#43](https://github.com/m0n0x41d/quint-code/issues/43))
- **Auto-baseline after `decide`** — when `affected_files` are provided, file hashes are snapshotted immediately after the decision is recorded. No more manual `haft_decision(action="baseline")` calls. ([#43](https://github.com/m0n0x41d/quint-code/issues/43))

### Changed

- **Core architecture refactored into explicit layers** — artifact build/store logic, presentation formatting, protocol transport, agent runtime, and TUI shell now live as clearer functional boundaries with purer `Build*`/formatting paths and thinner orchestration shells.
- **Agent execution moved beyond slash-command steering** — the repo now supports both MCP/plugin workflows and a standalone agent/TUI loop, with persisted execution mode aliases and compatibility bridges for older symbiotic/collaborative terminology.
- **Provider/model support expanded** — the registry and CLI now support multi-provider model discovery/switching with GPT-5.4-class defaults/fallbacks instead of the older 5.3-era baseline.
- **FPF search quality improved materially** — deterministic route lookup, better weighting/sanitization, explicit section summaries, and MCP-accessible spec search replaced the older narrower retrieval path.
- **`haft init --codex` TOML generation fixed** — idempotent section replacement instead of append, prevents duplicate key errors on repeated init.

### Fixed

- **`haft serve` / plugin mode now matches the core claim model** — served MCP schema and handlers understand predictions, strict decision/measurement arrays, claim refs/scope, and projection views instead of lagging behind the direct runtime.
- **Slash-command guidance no longer points users at stale `/q-*` actions** — note validation, nav strips, MCP presentation text, and h-reason docs now consistently steer users through the `h-*` surface, with `/h-view` as the advanced projection entry point.
- **Large pasted prompts no longer explode the TUI** — oversized pasted text is collapsed to `[N rows inserted]` placeholders in the input/queue/transcript UI, while the raw content is preserved and expanded only at submit time.
- **Queued follow-up messages preserve real prompt state** — multiline text, attachments, and hidden collapsed-paste payloads now survive queueing, replay, and draft restore paths without truncation or accidental `trim()` damage.
- **Decision/evidence integrity issues tightened** — malformed compare/measure payloads now fail loudly, Pareto fronts are computed deterministically, and claim/evidence bindings keep canonical scope instead of silently degrading.
- **Governance shutdown no longer panics on double-close** — `sync.Once` prevents channel double-close during fast project switching.
- **SwitchProject validates new project before teardown** — pre-checks DB accessibility, preventing zombie state if the target project is broken.
- **Task auto_run field restored from database** — was persisted but silently lost on restart.
- **WAL mode + busy_timeout on all SQLite connections** — prevents SQLITE_BUSY during concurrent governance scanner and UI queries.
- **Null safety across all Go→JSON view projections** — nil slices now serialize as `[]` not `null`, preventing frontend TypeError crashes on 30+ array fields.
- **Task runner race conditions fixed** — state copied under mutex before use outside lock in shutdown, cancel, and finalize paths.
- **Atomic file writes for config and registry** — temp file + rename prevents corruption from concurrent access.
- **Task timeout enforcement** — agent processes killed after configurable timeout (default 300 min), preventing zombie processes.
- **Artifact Create uses single transaction** — artifact insert and link inserts wrapped in one transaction, preventing partial state on link failure.
- **tableHasColumn PRAGMA cached** — eliminated 2 PRAGMA queries per evidence operation.
- **Large agent output truncated** — outputs over 500 lines show last 200 with "Show full output" button, preventing WebView freezing.
- **Search race condition fixed** — stale results from earlier queries no longer briefly flash.

## [5.3.1] — 2026-03-25

### Fixed

- **NavStrip no longer triggers agent auto-execution** — "Next:" label replaced with "Available:" + explicit guard line ("do not auto-execute"). Slash commands (`/q-explore`, `/q-decide`) replace tool call syntax (`quint_solution(action="explore", ...)`), so agents read them as user actions, not callable functions.
- **NavStrip is mode-aware** — available actions now reflect the current depth mode. Tactical shows `/q-explore | /q-decide` (short cycle). Standard without characterization shows `/q-char | /q-explore` — making `/q-char` visible as the gateway to the full cycle. Standard with characterization shows only `/q-explore`. EXPLORING in tactical shows `/q-decide | /q-compare (upgrade)` instead of always suggesting compare.
- **`quint_solution(action="compare")` rejected valid dimensions** — compare handler used raw type assertions instead of `parseStringArrayFromArgs` helper. When MCP clients serialized `dimensions` or `non_dominated_set` as JSON strings (common without schema loaded), the assertion silently failed, producing a misleading "at least one comparison dimension is required" error. Same fix applied to `scores` (new `parseNestedStringMapFromArgs` helper) and measure handler arrays (`criteria_met`, `criteria_not_met`, `measurements`).
- **"No baseline" scan confused with "not implemented"** — `CheckDrift` now checks git history for affected files when no baseline exists. Distinguishes "files changed since decision (likely implemented, needs baseline+measure)" from "files unchanged (not yet implemented)". Prevents agents from misreporting implemented decisions as not started.

### Added

- **NavStrip interpretation in q-reason skill** — new section documenting that "Available:" is a menu for the user, not instructions for the agent. Clarifies that tactical mode has fewer steps but the same human consent gates, and only Path 3 (explicit delegation) overrides the guard.
- **Proactive check for "no baseline" in q-reason skill** — instructs agents to not assume "no baseline" means "not implemented" and to check git history before reporting status.

## [5.3.0] — 2026-03-24

### Added

- **Interactive terminal dashboard (`quint-code board`)** — Bubbletea v2 TUI with four tabs: Overview (health, activity, depth distribution, coverage, contexts, evidence), Problems (backlog with drill-in), Decisions (list with R_eff/drift, drill-in with glamour markdown), Modules (coverage tree). Live refresh every 3s. Connected tab borders, alternating row colors, adaptive dark/light theme, dynamic help bar. Exit code 1 with `--check` flag for CI/hooks.
- **Decision mode computed from artifact chain** — `inferModeFromChain` derives mode from linked problems (characterization) and portfolios (comparison). Agent-declared mode can only escalate, not downgrade. Fixes misclassification where full-cycle decisions were recorded as tactical.
- **FTS5 search keyword enrichment** — `search_keywords` column on artifacts, indexed by FTS5. Agent generates synonyms and related terms at write time. Accepted on `quint_note` and `quint_decision`. Migration 15 rebuilds FTS5 index.
- **C/C++ header-only module detection** — `-I` include directories from `compile_commands.json` are registered as modules (FileCount=0), so dependency edges to `include/` directories are no longer dropped by `ScanDependencies`.

### Fixed

- **`/q-refresh scan` now rescans modules** — module structure updates alongside drift and stale checks, keeping dependency graph fresh without requiring a separate `coverage` action.
- **C/C++ symlink-safe include resolution** — `resolveInclude` canonicalizes both `projectRoot` and `-I` paths with `EvalSymlinks` before computing relative paths. Fixes silent edge loss on macOS symlinked checkouts.
- **Notes excluded from drift detection** — notes are observations, not implementations. ScanStale no longer flags notes with affected_files as "no baseline."
- **Module scanner excludes `.claude` and `.context` directories** — Claude Code worktrees and reference repos no longer inflate module count.
- **q-reason skill context-aware entry** — skill no longer always falls through into full FPF cycle. Three paths: think-and-respond (no artifacts), prepare-and-wait (human drives), full autonomous cycle (agent drives). Default is prepare-and-wait.

## [5.2.0] — 2026-03-23

### Added

- **C/C++ module detection** — `compile_commands.json` as primary source (searches project root, `build/`, `cmake-build-*/`). Falls back to directory-based heuristic with `Makefile`/`CMakeLists.txt`/`meson.build` markers. Graceful fallback if `compile_commands.json` paths don't resolve.
- **C/C++ import parsing** — extracts `#include "..."` edges (skips `<...>` system includes). Resolves include paths using `-I` flags from `compile_commands.json`. Falls back to relative and project-root resolution.
- **C/C++ extensions** — `.c`, `.h`, `.cpp`, `.cc`, `.cxx`, `.hpp`, `.hxx` registered in language registry.

### Fixed

- **`quint_solution(action="explore")` rejected valid variants** — MCP clients that serialize the `variants` array as a JSON string (instead of a parsed array) caused silent parsing failure, resulting in a misleading "genuinely distinct options" error with 0 variants. Same fix applied to all array fields across note/problem/decision handlers. ([#33](https://github.com/m0n0x41d/quint-code/issues/33))
- **Status always rescans modules** — `quint_query(action="status")` now runs a fresh module scan instead of showing stale cached data. Previously required `action="coverage"` to trigger rescan.
- **Symlink-safe path resolution** — C/C++ module detection uses `filepath.EvalSymlinks` on project root and source paths for reliable matching on macOS and symlinked project directories.

## [5.1.0] — 2026-03-20

### Added — Computed Features

- **R_eff computation** — effective reliability = min(evidence_scores) with CL penalties (CL3=0, CL2=0.1, CL1=0.4, CL0=0.9). Expired evidence scores 0.1. Computed on every access.
- **Evidence decay → stale detection** — decisions with R_eff < 0.5 auto-surface in stale scan. R_eff < 0.3 = "AT RISK" label.
- **Graduated epistemic debt** — stale items sorted by severity (days overdue), debt magnitude displayed.
- **Diversity check** — Jaccard similarity on variant titles+descriptions. Warns at >50% word overlap.
- **Archive recall** — FTS5 search at frame/explore time surfaces related active artifacts as "Related History".
- **Characterization cross-check** — compare warns on dimension mismatch, asymmetric scoring, parity rules.
- **Parity checklist** — auto-generated per-dimension parity questions from characterization.
- **Goldilocks signals** — readiness score (section completeness) + complexity counts (constraints, targets, dimensions) in problem selection.
- **Problem lifecycle** — three-way split: Backlog (no work) → In Progress (has portfolio) → Addressed (has decision).
- **Proactive evidence prompts** — after frame/explore in standard+ mode, tool prompts agent to collect and attach evidence.
- **Periodic refresh prompt** — if >5 days since last scan, any tool response reminds agent to run refresh.
- **Lemniscate feedback** — failed/partial measurement suggests reopen with concrete command.

### Added — Codebase Awareness

- **File drift detection (Level A)** — `baseline` action snapshots SHA-256 hashes of affected files. `/q-refresh` detects drift (modified, file missing, no baseline). Self-correcting: unbaselined decisions surface in `/q-status`.
- **Module detection (Level B)** — detects modules/packages across Go (`go/parser`), JS/TS (`package.json` + `index.ts` barrel files), Python (`__init__.py`), Rust (`Cargo.toml` + `mod.rs`). Interface-based architecture — one implementation per language behind `ModuleDetector`/`ImportParser` interfaces.
- **Dependency graph (Level C)** — parses imports to build module dependency graph. Go uses `go/parser` stdlib (100% accuracy), JS/TS/Python/Rust use regex. Impact propagation: when module A drifts, drift report shows dependent modules and their decisions.
- **Decision coverage report** — `quint_query(action="coverage")` shows governed/partial/blind modules. R_eff-aware: only `DecisionRecord` artifacts count as governance, `partial` status for modules where all decisions have degraded evidence (R_eff < 0.5).
- **`.quintignore` support** — project-specific exclusions for module scanning. Also respects `.gitignore` (local + global) via `go-gitignore` library.
- **Module coverage in `/q-status`** — coverage section appended to status dashboard when modules are scanned.
- **Module-aware onboarding** — `/q-onboard` now includes module coverage analysis step, prioritizes blind modules.

### Added — Unified Storage & Cross-Project Recall

- **Unified storage** — database moved from `.quint/quint.db` (in-repo) to `~/.quint-code/projects/{id}/quint.db` (home dir). Markdown projections remain in `.quint/` for code review. No binary files in git.
- **Project identity** — `.quint/project.yaml` with immutable generated ID (`qnt_` + 8 hex). Created on `init`, committed to git, same for all developers.
- **Cross-project decision recall** — `~/.quint-code/index.db` stores decision summaries across all projects. When framing a new problem (`/q-frame`), related decisions from other projects surface with CL2/CL1 tags.
- **CL matching** — same primary language = CL2 (similar context), different language = CL1 (different context). Auto-detected from codebase modules.
- **Serve guard** — if old `.quint/quint.db` exists but project not migrated, MCP blocks all tool calls with migration instructions.
- **`QUINT_SERVER_ORIGIN`** — new env var in MCP config. `local` (default) for solo dev. URL value accepted for future remote server mode (not implemented yet).
- **Context facts** — `context_facts` table auto-populated on startup with project fingerprint (languages, module count, decision count, domains).

### Added — Decision Quality & Integrity

- **Adversarial verification gate** — `/q-decide` runs a verification check before recording. Tactical: one-line counter-argument. Standard/deep: 5 probes (deductive consequences, strongest counter-argument, self-evidence check, tail failure scenarios, WLNK challenge). Grounded in FPF A.12 + Verbalized Sampling research.
- **Evidence supersession** — when `Measure()` records a new measurement, previous measurements on the same artifact are marked `verdict='superseded'` and excluded from R_eff. Prevents old partial measurements from dragging R_eff down permanently.
- **Inductive measurement gate** — `Measure()` warns if no baseline exists for the decision's affected files. Measurements without baseline record at CL1 (self-evidence), not CL3. R_eff honestly reflects verification quality.
- **R_eff shared package** — `internal/reff/` extracts `ScoreEvidence`, `VerdictToScore`, `CLPenalty` as shared pure functions. Single source of truth for both `artifact` and `codebase` packages.
- **Note-decision dedup** — containment-based overlap check at write time. If >70% of a note's title words appear in an active decision title, the note is rejected. 50-70% = warning. Also checks note-vs-note duplicates.
- **Reconcile action** — `quint_refresh(action="reconcile")` batch-scans all active notes against all active decisions for overlaps. One Go-side pass, no per-note agent calls.
- **Shipped vs Pending** — `/q-status` splits decisions into "Shipped" (has measurement) and "Pending Implementation" (no measurement).
- **Post-implementation ritual** — SKILL.md teaches agent to baseline + verify + measure after implementing a decision.

### Added — Developer Experience

- **Structured logging** — middleware in `serve.go` auto-logs every MCP tool call entry/exit with tool name, action, duration_ms, status. Domain logging for artifact create/baseline/drift and codebase scan operations.
- **Codex skill support** — `quint-code init --codex` installs `/q-reason` skill to `~/.agents/skills/q-reason/SKILL.md`.
- **Pre-commit hook** — `.githooks/pre-commit` mirrors CI pipeline exactly: `go mod tidy`, `golangci-lint`, `go test -race`, `go build`.

### Added — Product Features

- **FPF E.9 Decision Records** — four-component structure: Problem Frame, Decision/Contract, Rationale, Consequences. Decide response shows full DRR inline.
- **Indicator roles** — characterization dimensions tagged as constraint (hard limit), target (optimize), or observation (Anti-Goodhart).
- **Per-dimension measurement freshness** — valid_until on individual comparison dimensions. Compare warns on expired measurements.
- **Note auto-lifecycle** — notes auto-expire at 90 days. Detectable by scan. Waive/deprecate/supersede supported.
- **Generalized refresh** — waive/supersede/deprecate work on ALL artifact types (notes, problems, decisions, portfolios), not just decisions.
- **Multi-problem decisions** — `problem_refs` array parameter: one decision can address multiple problems.
- **Audit trail** — every tool call logged to audit_log table (fire-and-forget).
- **SoTA survey prompt** — explore in standard/deep mode reminds to check existing solutions before deciding.
- **Status caps** — dashboard sections capped (decisions=5, stale=5, problems=5, addressed=3) with overflow indicator.
- **List action** — `quint_query(action="list", kind="DecisionRecord")` for full artifact listing without caps.
- **Evidence display in problems** — /q-problems shows evidence count and verdict summary per problem.

### Fixed

- **CL=0 silent upgrade** — CL=0 (opposed context) no longer defaulted to CL=3. Uses -1 sentinel for "not provided".
- **NextSequence race condition** — uses MAX(id) instead of COUNT to avoid TOCTOU duplicate IDs.
- **Swallowed errors** — store.Update and store.AddLink errors in refresh operations now logged via logger.Warn.
- **FTS5 special characters** — comprehensive stripping of +, -, :, ~, single quote alongside existing chars.
- **MCP server stability** — panic recovery in request handler, 1MB stdin buffer (was 64KB), lifecycle logging (start/stop/EOF), stdout write error handling.
- **MCP init config** — uses QUINT_PROJECT_ROOT env instead of cwd. Removed stale nested .mcp.json.
- **Codex/Air project config** — `init --codex` / `init --air` now write MCP settings to project-local `.codex/config.toml` instead of shared `~/.codex/config.toml`.
- **writeFileQuiet** — uses logger.Warn instead of fmt.Fprintf(stderr).
- **MCP JSON string arrays** — `parseStringArray` now handles arrays sent as JSON strings by MCP clients (e.g., `"[\"a\"]"` instead of `["a"]`).
- **Coverage governance honesty** — only `DecisionRecord` artifacts count as governance. Notes no longer inflate coverage percentage.
- **Root module coverage** — root modules (Path: "") now match all files in the project, not just root-level files. Fixes JS/TS and Rust single-package coverage.
- **Measurement CL scoring** — measurements without baseline record at CL1 (0.4 penalty), not CL3. Prevents unverified measurements from inflating R_eff.
- **Coverage R_eff consistency** — unknown verdict in coverage computation now scores 0.5 (weakening), matching artifact package. Was incorrectly 0.0.
- **Status notes filter** — `/q-status` recent notes section filters out deprecated/superseded notes.
- **Evidence supersession in R_eff** — `ComputeWLNKSummary` excludes superseded evidence items from R_eff calculation.
- **FTS5 sanitization** — cross-project recall query sanitizer now strips periods, commas, semicolons, dashes, and other punctuation that caused FTS5 syntax errors.

### Changed

- **Apply deprecated** — decide response includes full DRR body. Apply action returns body directly (backward compat). `/q-apply` slash command removed.
- **Refresh UX** — tool description, schema, and slash command updated: "manage artifact lifecycle" not "detect stale decisions". `artifact_ref` parameter added (alongside `decision_ref` for compat).
- **Nav strip** — shows tactical decide option after frame. No apply prescription after decide.
- **Storage location** — database moved from `.quint/quint.db` to `~/.quint-code/projects/{id}/quint.db`. Requires re-running `quint-code init` to migrate.
- **Coverage always rescans** — `quint_query(action="coverage")` always runs fresh module scan instead of caching for 7 days.

### Removed

- `/q-apply` slash command
- Apply prescription from nav strip and decide response

## [5.0.0] — 2026-03-16

### Breaking Changes

Complete product redesign. v5 is not backward-compatible with v4.

- All v4 MCP tools removed
- All v4 slash commands removed
- Hypothesis/phase-based model replaced with problem/solution/decision model
- Phase FSM, role system, L0/L1/L2 user-facing layers removed
- .quint/ directory structure changed

### Added

- 6 MCP tools: quint_note, quint_problem, quint_solution, quint_decision, quint_refresh, quint_query
- 11 slash commands
- /q-reason skill with diagnostic framing protocol
- Artifact system with dual-write storage (DB primary, files secondary)
- Navigation strip in every tool response
- Note validation (rationale, conflicts, scope)
- Decision modes (note, tactical, standard, deep)
- Impact measurement and evidence attachment
- Versioned characterization
- All-artifact stale scan with lineage on reopen
- FPF spec search (4243 sections embedded)
- goreleaser for cross-platform builds

### Architecture Decisions

ADR-1 through ADR-19 documented in `.context/v5-architecture-decisions.md`
