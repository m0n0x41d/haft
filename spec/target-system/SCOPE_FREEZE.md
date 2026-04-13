# Scope Freeze

> Reading order: 3 of N. Read after TERM_MAP. 10 minutes.
>
> Rule: if it's not in v6, don't model it in detail. Don't build it. Don't test it.

## v6.0 — Ship (current release)

Everything below is built, tested, and being merged to main.

### Core (proven, stable)
- Artifact graph: ProblemCard, SolutionPortfolio, DecisionRecord, EvidencePack, Note, RefreshReport
- 6 MCP tools: haft_problem, haft_solution, haft_decision, haft_query, haft_refresh, haft_note
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
- MCP Plugin: stable 6-tool interface
- CLI: haft init, serve, sync, agent, board, doctor, fpf search
- Desktop App: Wails v2, pre-alpha

### Skill & Commands
- h-reason skill with 5-mode model (Understand/Explore/Choose/Execute/Verify)
- 12 slash commands: h-frame, h-char, h-explore, h-compare, h-decide, h-verify, h-note, h-status, h-search, h-problems, h-onboard, h-view
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

## v6.2 — Execution Primitives (days 31-60)

Focus: desktop starts proving the loop, not just visualizing.

- [ ] Decision-anchored "Implement" from desktop (spawn agent in worktree with invariants + rationale)
- [ ] Worktree/branch creation
- [ ] Automatic post-run invariant verification
- [ ] Baseline refresh only on successful verification
- [ ] Persisted verification/governance result linked to decision

## v7.0 — Desktop Loop MVP (days 61-90)

Focus: one golden path, not a broad feature wave.

Ship exactly one vertical slice:
- [ ] **Decision → Implement → Verify → Baseline → PR draft/export**
- [ ] If verification fails → reopen/create ProblemCard (not straight to PR)
- [ ] PR output is local-first: draft body + branch compare, not deep provider integration

**NOT in v7:** GitHub/Linear intake, research-before-code, process metrics, autonomous agents.

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
| Formal spec (Quint/TLA+) | Research project. No validated use case yet. |
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
