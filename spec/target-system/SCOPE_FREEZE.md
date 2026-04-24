# Scope Freeze

> Reading order: 3 of N. Read after TERM_MAP. 10 minutes.
>
> Rule: if it's not in v6, don't model it in detail. Don't build it. Don't test it.

## Strategic Product Pivot

The product center has moved from "decision governance plus task execution" to
**project harnessability**.

Haft must not merely run agents on a repository. Haft must make the repository
ready for rigorous AI-assisted engineering:

```text
Add project
  -> Init .haft and host-agent MCP config
  -> Onboard TargetSystemSpec
  -> Onboard EnablingSystemSpec
  -> Validate TermMap and SpecCoverage
  -> Create Decisions from spec sections
  -> Create WorkCommissions from decisions
  -> Run harness runtime
  -> Attach Evidence and update SpecCoverage
```

Large formal specs are not deferred research. They are the primary harness
material. The implementation must be staged, but the target product model is
spec-first.

## v6.0 — Ship (current release)

Everything below is built, tested, and being merged to main.

### Core (proven, stable)
- Artifact graph: ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, Note, RefreshReport
- 7 MCP tools: haft_problem, haft_solution, haft_decision, haft_commission, haft_query, haft_refresh, haft_note
- Problem close action (mark ProblemCard as addressed)
- FPF spec search/index (~800 sections, route-aware tiered retrieval)
- Evidence engine: R_eff, WLNK, CL penalties, evidence decay, valid_until, verdict
- Pareto front computation with constraint-aware elimination
- Knowledge graph: decisions→code→modules→dependencies (17 tests)
- Codebase analysis: module detection (Go, JS/TS, Python, Rust, C/C++), import parsing, symbol hashing
- Decision coverage reporting: governed / partial / blind modules
- Cross-project recall: ~/.haft/index.db with CL2/CL1 penalties
- Claims with verify_after dates, pending verification surfacing
- Auto-baseline after decide (when affected_files present)
- Invariant injection into agent prompts (from knowledge graph)
- Invariant verification against live dependency graph
- haft sync: team workflow via git-tracked .haft/*.md → local SQLite

### Desktop App (pre-alpha, functional)
- Dashboard with governance findings
- Problem board with drill-in
- Decision detail with evidence F/G/R decomposition
- Portfolio comparison with Pareto front visualization
- Task spawning (Claude Code / Codex agents)
- Agent chat view
- Terminal panel
- Multi-project management
- Search (Cmd+K)

### Surfaces
- MCP Plugin: stable 7-tool interface
- CLI: haft init, serve, sync, agent, board, doctor, fpf search, commission
- Desktop App: Wails v2, pre-alpha

### Skill & Commands
- h-reason skill with 5-mode model (Understand/Explore/Choose/Execute/Verify)
- 13 slash commands: h-frame, h-char, h-explore, h-compare, h-decide, h-verify, h-note, h-status, h-search, h-problems, h-onboard, h-view, h-commission
- Probe-or-commit readiness gate
- Language precision triggers in skill instructions
- Audience projections: engineer, manager, audit, compare, delegated-agent, change-rationale

## v6.1 — Harden the Contract (days 0-30)

Focus: trust and momentum for existing MCP/CLI users. No new surfaces.

- [ ] Fix remaining P2-P4 bugs from code review
- [ ] Core packages have zero desktop/ dependencies (clean boundary)
- [ ] Integration tests for knowledge graph on real project data
- [ ] Evidence decomposition tests (F/G/R computation)
- [ ] `haft check` CLI command for CI (verify decisions fresh, evidence current)
- [ ] `.haft/workflow.md` — simple hybrid markdown+structured, injected into agent prompts
- [ ] Problem type field on ProblemCard (optimization/diagnosis/search/synthesis)
- [ ] **Decision integrity enforcement (G1/G2/G4):**
  - [ ] G1: reject duplicate active decisions per problem
  - [ ] G2: require structured parity plan in standard/deep compare
  - [ ] G4: warn on subjective comparison dimensions (maintainable, simple, scalable)
- [ ] Rename/install/docs polish for migration friction

## v6.2 — Dashboard + Implement + Adopt (days 31-60)

Focus: one proved execution loop. BDD scenarios in `spec/target-system/EXECUTION_CONTRACT.md`.

### Dashboard
- [ ] Unified dashboard replacing separate Problems/Decisions pages
- [ ] Active decisions with **Implement** button
- [ ] Governance findings with **Adopt** / **Waive** / **Reopen** inline
- [ ] Implement guards: G1 blocks, G2/G4 warn, invariant-less warns

### Implement
- [ ] Worktree/branch creation
- [ ] Agent session with invariants + rationale + workflow.md + knowledge graph invariants
- [ ] Checkpointed mode (default)
- [ ] Post-run invariant verification
- [ ] Baseline on pass, verification evidence CL3
- [ ] Failure → "Needs attention" (fix/reopen/dismiss)
- [ ] "Create PR" on pass

### Adopt
- [ ] Agent thread with decision context + drift/stale report
- [ ] Resolution: re-baseline / reopen / waive / deprecate / measure
- [ ] Agent never auto-resolves
- [ ] RefreshReport recorded

### Deferred to v7 (per 5.4 review)
- Full project harnessability onboarding
- Spec parser/checker and SpecCoverage graph
- Automation triggers (problem factory — don't mix with execution)
- Broad autonomous execution over many commissions
- DecisionRecord→WorkCommission→RuntimeRun pipeline with auto-advance (scaling factory)

**Terminology:** "problem" = ProblemCard only. Dashboard page = "Dashboard", not "Problem Board".

## v7.0 — Project Harnessability MVP (next major slice)

Focus: one golden path that proves Haft can make a real project harnessable,
not merely run an agent task.

Ship exactly one vertical slice:
- [ ] Add existing project from Desktop and detect readiness (`ready`, `needs_init`, `needs_onboard`, `missing`)
- [ ] `haft init` / Desktop init creates `.haft/`, workflow policy, and host-agent MCP config
- [ ] Onboarding agent drafts `TargetSystemSpec` with stable SpecSection ids
- [ ] Onboarding agent drafts `EnablingSystemSpec` only after target spec passes structural validation
- [ ] `haft spec check` validates required sections, term map, statement types, and target/enabling split
- [ ] `haft spec plan` proposes DecisionRecord drafts linked to spec sections
- [ ] Create one WorkCommission from a spec-linked decision
- [ ] Harness runtime executes that commission and writes evidence
- [ ] SpecCoverage updates the relevant section from `commissioned` to `verified`

**NOT in v7:** GitHub/Linear as primary intake, process metrics, broad
autonomous campaigns, perfect semantic validation, function-level coverage, or
manager dashboards. Optional ExternalProjection design is allowed, but trackers
remain carriers, not sources of truth.

## v7.1 — Desktop Cockpit for Spec-First Work

Focus: make the deep spec workflow operationally usable.

- [ ] Project readiness panel: init/spec/coverage/runtime state
- [ ] Target spec workspace: section tree, missing required sections, term gaps
- [ ] Enabling spec workspace: repo architecture, tests, agent policy, runtime policy
- [ ] Spec coverage view: uncovered/reasoned/commissioned/verified/stale
- [ ] Decision planning view: accept/merge/split/discard spec-derived decision drafts
- [ ] Runtime cockpit: runnable/running/blocked/completed commissions, evidence, apply/cancel/requeue
- [ ] Long-lived conversations bound to project/spec/decision context, not terminal one-shot tasks

## v7.2 — Spec Enforcement Hardening

Focus: make formal specs executable enough to justify autonomy.

- [ ] Strict markdown parser for `spec-section` YAML blocks
- [ ] TermMap parser and ambiguous-term checker
- [ ] SpecSection revision/hash snapshots
- [ ] SpecCoverage graph persisted as edges, derived as view state
- [ ] Preflight checks spec section revisions for spec-linked commissions
- [ ] Behavior/interface/spec test links as evidence carriers
- [ ] `haft check` includes spec readiness and stale section checks

## vNext — Haft Harness Runtime (Open-Sleigh subsystem)

Focus: make execution scalable inside the **Haft product** without collapsing
DecisionRecord, work authorization, runtime attempt, and external tracker
carrier into one object. Open-Sleigh is the current runtime implementation of
this subsystem, not a peer product.

### Commissioned execution
- [ ] `WorkCommission`: create/queue/start/cancel/expire lifecycle
- [ ] Mandatory freshness gate before execution
- [ ] `RuntimeRun`: one Open-Sleigh attempt per commission
- [ ] `ImplementationPlan`: DAG of WorkCommissions with dependencies/locksets
- [ ] `AutonomyEnvelope`: explicit YOLO/batch bounds

### Projection
- [ ] Local-only execution works with no Linear/Jira/GitHub configured
- [ ] Optional `ExternalProjection` records desired/observed external state
- [ ] ProjectionWriterAgent writes manager-facing text from deterministic intent
- [ ] ProjectionValidation blocks invented progress, missing links, and forbidden claims
- [ ] Manual tracker status changes record drift/conflict, not semantic completion

### Harness runtime
- [ ] Open-Sleigh consumes Haft WorkCommissions, not raw tracker tickets
- [ ] First phase is Preflight; Execute starts only after Haft admits execution
- [ ] Batch scheduler respects dependencies, locksets, leases, and envelope limits
- [ ] External projection publish failure does not invalidate execution evidence
- [ ] Product/operator surface stays `haft harness`; runtime codename remains internal

**Not in this slice:** project-management features such as sprints, estimation
games, Gantt charts, manager dashboards inside Haft, or making Jira/Linear
required infrastructure.

## v8 — Governor Signals (post day 90)

Focus: detect-only first. Autonomous actuation later.

Phase A (detect-only):
- [ ] Stale decision detection loop
- [ ] Drift detection loop
- [ ] Dependency findings loop
- [ ] Dashboard alerts and notifications

Phase B (actuation — only after Phase A trust earned):
- [ ] Spawn verification agent for expired decisions
- [ ] Auto-update non-breaking drift
- [ ] Research-before-code lane
- [ ] Process quality metrics

**Key principle:** do not automate execution until decision quality is enforced, not merely encouraged.

## Later / Maybe

Things we know about but explicitly defer:

| Feature | Why deferred |
|---------|-------------|
| Server mode (PostgreSQL) | Needs real multi-user demand signal. Solo/team-via-git is sufficient now. |
| Formal methods spec (Quint/TLA+) | Different from Haft's parseable project specs. Useful later for selected high-risk modules, not the default carrier. |
| FSRS spaced repetition for reviews | Needs server mode. |
| Full autonomy budgets (FPF E.16) | Redundant with host agent permissions for now. |
| Web UI | Desktop app is the visual surface. No browser version. |
| Mobile app | Not our market. |
| Slack/Discord bot | Not our market. Surface sprawl. |
| Full campaign orchestration | Build journal primitives first (v7), full orchestration later. |
| Z3 SMT parity solver | Solves wrong problem (input quality, not computation). |
| Self-improving skill library | Needs role/context/assurance model first. |

## Never

Things that are out of identity scope:

| Feature | Why never |
|---------|-----------|
| General personal assistant | Different target system entirely |
| Consumer/omnichannel surface | Different market |
| Competing with Claude Code on code editing | Plugin rides on host agent execution |
| FPF pattern browser | FPF is engine, not interface |
| Project management (sprints, tickets, Gantt) | Different product category |
| AI model training/fine-tuning | Infrastructure, not our layer |
