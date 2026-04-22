---
title: "Open-Sleigh Product-Ready TODO"
version: v0.1
date: 2026-04-22
status: living backlog for turning the local MVP into a usable product
owner: enabling system
---

# Open-Sleigh Product-Ready TODO

This document is the execution backlog from the current implementation to
a product that can be used on real projects. It is not a replacement for
the normative specs. It is the work queue that keeps implementation,
evidence, and remaining risk in one place.

## Operating Rule

Work should continue autonomously through safe, reversible engineering
slices. Human approval is needed only for:

- credentials and external account access,
- publishing, billing, legal, privacy, or security-impacting choices,
- destructive operations or compatibility breaks,
- choosing support scope for a new provider or public interface,
- accepting a product trade-off that changes who the product is for.

Everything else should be implemented, tested, and reported as evidence,
not repeatedly re-asked.

## Product-Ready Definition

Open-Sleigh is product-ready for local project work when an operator can:

- configure one repository and one tracker project from `sleigh.md`;
- run `mix open_sleigh.doctor` and get actionable preflight failures;
- start the runtime with real adapters, not mock adapters;
- see current engine state without attaching a debugger;
- observe dispatch failures in the tracker item itself;
- run one ticket from intake through publication or a human gate;
- recover from process restart without losing durable tracker-facing facts;
- understand what remains manual before the first real run.

The product target here is a local CLI/operator product. SaaS hosting,
multi-tenant auth, billing, and a public web dashboard are outside this
definition.

## Current Reality

Evidence gathered on 2026-04-22:

- `mix format --check-formatted` passes.
- `mix compile --warnings-as-errors` passes.
- `mix test` passes.
- `mix credo --strict` passes.
- Mock `open_sleigh.start --once` writes a runtime status snapshot.
- `open_sleigh.status` reads that snapshot in text and JSON modes.
- `open_sleigh.doctor --path sleigh.md.example` correctly reports this
  machine's missing real-runtime inputs: `LINEAR_API_KEY` and `REPO_URL`.

Known gap: there is no real Linear plus repository canary evidence yet in
this repo. That requires external credentials and a test ticket.

## Done

- [x] Config loader for `sleigh.md`.
  - Acceptance: engine, tracker, agent, workspace, hooks, and gate config
    can be read from one config file.
  - Evidence: config-related tests pass.

- [x] Runtime preflight task.
  - Acceptance: `mix open_sleigh.doctor --path sleigh.md.example` validates
    local tools and real-runtime environment variables.
  - Evidence: command reports missing `LINEAR_API_KEY` and `REPO_URL` on
    this machine instead of pretending the runtime is ready.

- [x] Real runtime boot path.
  - Acceptance: `mix open_sleigh.start` can build tracker, agent, workspace,
    hooks, WAL, and orchestrator components from config.
  - Evidence: compile and integration tests pass; mock once-run exercises
    the same Mix task entrypoint.

- [x] Codex app-server protocol foundation.
  - Acceptance: initialize, user input response, command approval,
    file-change approval, apply-patch approval, unsupported tool calls, and
    approval policy are represented.
  - Evidence: protocol tests pass.

- [x] Haft-backed ProblemCard fetch path.
  - Acceptance: upstream problem-card references can be fetched through the
    Haft port and decoded from MCP-style content payloads.
  - Evidence: Haft client and orchestrator tests pass.

- [x] Orchestrator prompt rendering.
  - Acceptance: ticket, ProblemCard, PR, CI, and session placeholders render
    into phase prompts.
  - Evidence: prompt rendering tests pass.

- [x] Human publication gate.
  - Acceptance: branch regex policy drives whether publication can proceed
    automatically or waits for approval.
  - Evidence: publication gate tests pass.

- [x] Terminal tracker transition and terminal comment.
  - Acceptance: completed runs can move the tracker item to the configured
    terminal state and post a final tracker comment.
  - Evidence: external publication tests pass.

- [x] Runtime status snapshot.
  - Acceptance: runtime writes a JSON state file and `mix open_sleigh.status`
    can read it.
  - Evidence: mock once-run plus status command passed locally.

- [x] Tracker-facing dispatch failure comments.
  - Acceptance: missing upstream ProblemCard and self-authored upstream
    ProblemCard failures post actionable, idempotent tracker comments and do
    not claim the ticket.
  - Evidence: `mix test test/open_sleigh/orchestrator_dispatch_failure_test.exs`.

- [x] Runtime status recent failure view.
  - Acceptance: status JSON includes recent dispatch/session failure rows and
    text status prints failure count plus concise summaries.
  - Evidence: targeted status and dispatch failure tests pass.

- [x] Tracker-facing session failure comments.
  - Acceptance: adapter/session startup failures post actionable,
    retry-aware tracker comments while retry state remains visible.
  - Evidence: orchestrator retry tests cover adapter start failure comments.

- [x] Stale snapshot and human-gate status details.
  - Acceptance: text status identifies stale snapshots and lists pending
    human gate ticket, session, gate name, and requested-at timestamp.
  - Evidence: status CLI fixture test covers stale and human-waiting output.

- [x] Workspace and repository preflight validation.
  - Acceptance: doctor verifies repository URL shape, local git availability,
    workspace root write access, and publication branch regex before runtime
    start.
  - Evidence: Mix task tests cover passing config, malformed repository URL,
    and unwritable workspace root.

- [x] Runtime log file.
  - Acceptance: config can set a log path; runtime writes structured JSONL
    lifecycle events with event ids.
  - Evidence: mock once-run writes parseable log entries.

- [x] Blocking after-create hook failures.
  - Acceptance: `after_create` hook failure prevents agent session startup
    and flows through existing retry/status/tracker failure reporting.
  - Evidence: AgentWorker test covers non-zero `after_create` exit before
    agent start.

- [x] Hook failure policy surface.
  - Acceptance: `hooks.failure_policy` supports `blocking`, `warning`, and
    `ignore`; doctor rejects invalid policy values.
  - Evidence: AgentWorker and Mix task tests cover blocking, warning, and
    invalid policy behavior.

- [x] Non-destructive workspace cleanup policy.
  - Acceptance: `workspace.cleanup_policy: keep` is explicit, status metadata
    records it, and doctor rejects destructive unsupported policies.
  - Evidence: Mix task tests cover `keep` and rejected destructive policy.

- [x] First-run and troubleshooting guides.
  - Acceptance: operator has a fixed first-run evidence template and a
    troubleshooting guide mapped to doctor/status/tracker failure surfaces.
  - Evidence: `docs/FIRST_RUN.md`, `docs/TROUBLESHOOTING.md`, and
    `docs/runs/README.md`.

- [x] Example tracker ticket templates.
  - Acceptance: operator has a low-risk intake canary template and a
    publication-gated canary template.
  - Evidence: `docs/TICKET_TEMPLATES.md`.

## P0 Critical Path To First Real Use

- [x] Commit the status-snapshot and failure-visibility slice.
  - Why: keep the repo checkpointed before the next runtime behavior change.
  - Acceptance: one cohesive commit with no forbidden provider references in
    the commit message.
  - Evidence: `8a91cec feat: add runtime status and failure visibility`.

- [ ] Run real preflight with operator-provided environment.
  - Why: current runtime is structurally wired, but this machine still lacks
    required real inputs.
  - Acceptance: `mix open_sleigh.doctor --path <real sleigh.md>` returns no
    errors for Linear, repository URL, Codex, Haft, git, and workspace root.
  - Evidence command: `mix open_sleigh.doctor --path <real sleigh.md>`.
  - Human input: `LINEAR_API_KEY`, `REPO_URL`, valid tracker project config.

- [ ] Create a live canary ticket.
  - Why: the product is only useful after a real tracker item can be claimed,
    dispatched, observed, and released or gated.
  - Acceptance: one low-risk ticket contains a valid ProblemCard reference
    and moves through the configured workflow without mock adapters.
  - Evidence command: `mix open_sleigh.start --path <real sleigh.md> --once`.
  - Human input: approval to use one real tracker ticket.

- [x] Post tracker comments for malformed upstream frame dispatch failures.
  - Why: if a ticket is malformed, the operator should see the failure where
    they work, not only in logs.
  - Acceptance: missing ProblemCard and self-authored upstream ProblemCard
    reasons create actionable tracker comments.
  - Evidence: tests cover missing and self-authored upstream ProblemCard.

- [x] Post tracker comments for adapter startup failures.
  - Why: once a session exists, adapter boot failures should still show up in
    the operator's tracker workflow.
  - Acceptance: adapter start-session failure creates an actionable tracker
    comment and a retry-visible status entry.
  - Evidence: orchestrator retry test covers `:thread_start_failed` comment
    and retry-visible status.

- [x] Add recent dispatch/session failure visibility to runtime status.
  - Why: `open_sleigh.status` should answer "why is nothing happening?"
    without log digging.
  - Acceptance: status JSON includes recent dispatch, session, human resume,
    and tracker transition failures; text output prints failure count and
    summaries.
  - Evidence: status CLI fixture and dispatch failure status snapshot tests.

- [x] Add stale snapshot age and richer human-waiting status.
  - Why: after a process dies or waits on an approver, the status command
    should say that directly.
  - Acceptance: text status identifies stale snapshots, pending human gate
    names, requested-at timestamps, and active session ids.
  - Evidence: status CLI fixture covers stale and human-waiting output.

- [ ] Record first real-run evidence.
  - Why: docs and tests are not proof that the real product works.
  - Acceptance: a dated note captures command, config hash, tracker item,
    repository branch, gate outcome, and any manual intervention.
  - Evidence artifact: `docs/runs/<date>-first-real-run.md`.

## P1 Reliability And Operator UX

- [x] Structured failure comments.
  - Acceptance: every tracker-facing failure comment has reason, operator
    action, retry behavior, and correlation id.
  - Evidence: dispatch and session failure tests assert reason, action, retry,
    and marker/session identifiers.

- [x] Retry visibility.
  - Acceptance: retry attempts expose reason, next retry time, and attempt
    number in status JSON.
  - Evidence: orchestrator retry tests cover attempt, delay, due time, and
    error in status.

- [x] Stale runtime detection.
  - Acceptance: `open_sleigh.status` reports stale status snapshots and gives
    the exact path being read.
  - Evidence: status CLI tests with old timestamp fixture.

- [x] Runtime log file.
  - Acceptance: config can set a log path; runtime writes structured events
    with correlation ids.
  - Evidence: mock once-run writes parseable log entries.

- [x] Workspace repository validation.
  - Acceptance: doctor verifies the repository URL, local git availability,
    workspace root write access, and branch naming policy before runtime
    start.
  - Evidence: doctor tests with failing repo, unwritable workspace root, and
    passing repo fixtures.

- [x] Workspace cleanup policy.
  - Acceptance: completed, failed, and human-waiting sessions have explicit
    cleanup behavior.
  - Evidence: config and doctor enforce non-destructive `keep`; destructive
    delete policies require future human approval.

- [x] Hook hardening.
  - Acceptance: hook failures are typed as blocking, warning, or ignored by
    config; status and tracker comments expose blocking failures.
  - Evidence: `hooks.failure_policy` runtime and doctor tests.

- [x] Terminal transition fallback.
  - Acceptance: if tracker transition fails, the engine records the failure
    and does not report the session as cleanly completed.
  - Evidence: `test/open_sleigh/orchestrator_transition_fallback_test.exs`.

- [x] ProblemCard contract fixture suite.
  - Acceptance: valid, missing, malformed, self-authored, and stale upstream
    cards are covered by fixtures.
  - Evidence: `test/open_sleigh/haft/client_test.exs`.

## P2 Gate Quality

- [x] Real semantic judge adapter.
  - Acceptance: semantic gates can call a configured model provider through a
    narrow adapter and return deterministic verdict shapes.
  - Evidence: `OpenSleigh.Judge.AgentInvoker` plus
    `test/open_sleigh/judge/agent_invoker_test.exs`.

- [x] Golden set for semantic gates.
  - Acceptance: representative pass/fail examples exist for each enabled
    semantic gate.
  - Evidence: `test/open_sleigh/judge/golden_sets_test.exs`.

- [x] Enable semantic gates in the example config.
  - Acceptance: `sleigh.md.example` shows production-intended settings after
    the judge path is calibrated.
  - Evidence: `sleigh.md.example`, compiler tests, doctor tests, and gate
    report tests.

- [x] Gate drift report.
  - Acceptance: gate behavior can be compared across judge versions without
    changing runtime code.
  - Evidence: `mix open_sleigh.gate_report` and Mix task tests.

## P3 Packaging And Documentation

- [x] Installable release.
  - Acceptance: operator can run Open-Sleigh without understanding Mix internals.
  - Evidence: escript CLI entrypoint, Mix release config, and CLI tests.

- [x] Config schema validation report.
  - Acceptance: config errors show field path, expected shape, actual value,
    and fix hint.
  - Evidence: doctor JSON schema-report test.

- [x] First-run guide.
  - Acceptance: README has a short path from clone to real canary run.
  - Evidence: README links to `docs/FIRST_RUN.md`; real command execution
    still requires live credentials and a live canary ticket.

- [x] Troubleshooting guide.
  - Acceptance: common failures have symptoms, cause, command, and fix.
  - Evidence: `docs/TROUBLESHOOTING.md` maps doctor/status/tracker failure
    surfaces to fixes.

- [x] Example tracker ticket templates.
  - Acceptance: operator can create a valid intake ticket and a valid
    publication-gated ticket from examples.
  - Evidence: `docs/TICKET_TEMPLATES.md`.

## P4 Provider Expansion

- [ ] Codex adapter live hardening.
  - Acceptance: one real project run completes through the Codex adapter
    before another adapter is added.
  - Evidence: first real-run note and no unresolved P0 runtime failures.

- [x] Claude Code adapter decision record.
  - Acceptance: support is intentionally accepted or deferred with parity
    criteria, maintenance cost, and revert rule.
  - Evidence: decision record in `specs/target-system/ADAPTER_PARITY.md`.

- [x] Claude Code adapter skeleton.
  - Acceptance: adapter implements the same `Agent.Adapter` boundary without
    weakening the core state model.
  - Evidence: adapter contract tests pass for both adapters.

- [x] Provider parity test matrix.
  - Acceptance: common session lifecycle, approval, file change, command
    request, failure, and cancellation behaviors are tested across providers.
  - Evidence: `test/open_sleigh/agent/adapter_parity_test.exs` covers the
    provider-independent boundary; live provider quality remains gated by the
    first real-run evidence.

## P5 Optional Product Surface

- [x] Read-only local HTTP status API.
  - Acceptance: status endpoint mirrors status JSON and never exposes Haft
    artifact bodies or secrets.
  - Evidence: `OpenSleigh.StatusHTTP`, `OpenSleigh.StatusHTTPServer`, and
    status HTTP tests.

- [x] Minimal browser dashboard.
  - Acceptance: operator can inspect sessions, failures, pending gates, and
    recent tracker comments locally.
  - Evidence: dashboard handler test plus Playwright smoke snapshot against
    `http://127.0.0.1:48767/dashboard`.

- [x] Notification adapter.
  - Acceptance: pending human gates and blocking failures can notify a local
    or team channel through an abstract notification port.
  - Evidence: notification adapter behavior and local JSONL sink tests.

## Human-Only Inputs Remaining

- `LINEAR_API_KEY` for real tracker access.
- `REPO_URL` for the repository that Open-Sleigh should work on.
- A live tracker project/team/status mapping.
- One intentionally low-risk live canary ticket.
- Confirmation that adding Claude Code support is desired after the Codex
  path has real-run evidence.

## Next Autonomous Slice

All remaining open work requires live operator inputs: `LINEAR_API_KEY`,
`REPO_URL`, valid tracker workflow mapping, and one low-risk live canary
ticket. The local engineering backlog is closed for MVP-1 product use.
