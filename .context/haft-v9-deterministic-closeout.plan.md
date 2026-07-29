# Haft v9 deterministic closeout plan

Status: active
EntityOfConcern: `entity:haft-v9-typed-memory` — Haft v9 typed project memory
Receiving use: close `R7` source acceptance and `R8` installed-runtime
acceptance without Goal automation, concurrent writers, or unreviewed process
signals
Execution owner: one agent turn at a time in this exact Codex task
Publication authority: absent; this plan does not authorize commit, tag, push,
release, or a public maturity claim

## V9 scope freeze — operator accepted 2026-07-28

This file is the sole active WorkPlan-like execution carrier for the v9
closeout. `.context/haft-v9-remaining-execution.plan` and
`.context/haft-v9-typed-memory-e2e-master-plan.md` remain reference and history
carriers; neither may override the order, readiness posture, or acceptance
boundary below.

The operator authorized implementation and updates to this execution carrier.
Commit, push/integration, SpecSection lifecycle, host-process transition, tag,
publication, and release remain separate exact gates.

### Frozen v9 product claim

V9 includes:

- source-native FPF Query;
- structured typed project memory with a fresh-project onboarding path;
- task-level establishment of the first EntityOfConcern without exposing raw
  TypeEnv or MemoryChangeSet assembly;
- specification lifecycle and manual authority boundaries;
- a static source-derived code graph whose governing-decision relations are
  honest about exact bindings and module context;
- complete Codex and Claude init projections;
- upgrade acceptance from the actually published v8.1.0;
- one installed-candidate P14 across CLI, live MCP, and host-process surfaces.

The following are explicitly deferred to v9.x and do not block v9 unless a new
public claim reopens them:

- dense, vector, or hybrid retrieval and superiority studies;
- broader language-adapter coverage;
- LSP or runtime dispatch;
- full FPF RelationSignature lowering;
- a public architecture editor or facet;
- mandatory automatic persistence;
- public `h-plan`, a second graph/runtime authority, or stable Pi parity.

After this freeze, an additional change enters v9 only when it repairs a
violation of the frozen contract or its acceptance oracle. Other improvements
return to v9.x.

### P0 — contract closure before D2/P13

P13 must not begin until all of these are source-complete and focused-green:

- [x] always-advertised `haft_onboard` and `haft_entity` task-level MCP
  contracts, canonical `U.EntityRef`, atomic entity-plus-alias admission, and
  typed recovery before memory enablement;
- [x] complete Codex/Claude init effects, clear TTY rows and receipts, known
  legacy takeover, single-error rendering, and language-neutral onboarding;
- [ ] actual compacted `initialize`/`tools/list` contract, canonical skill
  materialization, current specifications, and no marker-only semantic-sync
  claim;
- [x] canonical project-path handling and segment-safe exact/module governance
  relations across graph, context, coverage, fusion, Explore, and Overseer;
- [x] predecessor restart-checkpoint compatibility that cannot brick public
  status;
- [x] explicit P13 orchestration, zero CI-pinned lint findings, real installer
  exercise, v8.1 lineage and upgrade lane, release notes, and separated
  tag-validation/publication authority;
- [x] all five historical P14 mismatch families classified and covered by a
  current regression;
- [x] the six planned code-graph/agent scenarios materialized beside the
  existing twenty P14 scenarios.

Inventory baseline:
`.context/haft-v9-scope-freeze-inventory-20260728.md`.

### P0 source-acceptance evidence — 2026-07-29

Seven P0 clauses are source-complete and current-green. The compact MCP,
canonical-skill, and marker-honesty parts of the remaining clause are also
green, but the clause stays open because its `current specifications`
condition is false: the active typed-memory DecisionRecord and reviewed term
map still bind merge/split semantics that the frozen v2 SoftwareSystemSpec and
runtime schema exclude. Resolving that contradiction requires the separate
human semantic and SpecSection lifecycle gates. It is not a mechanical
closeout edit.

Current source evidence:

- final isolated source candidate:
  `/private/tmp/haft-v9-source.w7YxTN/haft`,
  `sha256:ad3161af7228e02a494e3ec0cea03e9b0ed0bf00073f204c7fcc66875f01bb77`;
- `go test ./internal/cli -count=1 -timeout=30m` and
  `go test ./... -count=1 -timeout=30m` passed on the final source bytes;
- `go vet ./...`, pinned `golangci-lint v2.11.4`, Pi TypeScript typecheck and
  33/33 tests, actionlint v1.7.7, shell syntax checks, and
  `git diff --check` passed;
- direct C4 result
  `/private/tmp/haft-v9-source.w7YxTN/c4/run-nanh_hfa/results.json` is green:
  exactly 12 compact MCP tools, closed nested schemas, pre-basis and
  `known_absent` no-write recovery, Codex/Claude real-TTY and flag equivalence,
  idempotency, exact legacy takeover, fail-before-write collision handling,
  single-error rendering, outside-marker preservation, and non-mutating
  predecessor-checkpoint diagnostics.
- MethodRun
  `mpull-20260728-implement-the-operator-approved-haft-v9-scope-fr-17122bd7`
  is closed with both hard gates satisfied, no waivers, and verification
  result `partial`.

This temporary binary is source-probe evidence only. It is not installed and
does not satisfy D3, P14, or any installed-runtime claim. P0 therefore remains
open at the specification contradiction, and D2 through D7 remain open.

## Baseline

- Exact project TypeEnv:
  `sha256:6dc594a9d5470701b583a6e0893cf75d89629a27673d7aecd34b0993979c6aaf`.
- ProjectTypeEnvHead revision: `2`.
- Project graph revision: `8`.
- Last passing source carrier:
  `.context/p13/p13-acceptance-20260727T063937.898131000Z-d4f14caf7b0e53e7.json`.
- That carrier passed all eleven suites, preserved one start/end identity, and
  recorded no waivers. It becomes historical as soon as the goal-free resume
  repair changes source bytes.
- Reproducible diagnostic binaries for that historical identity:
  - `haft`: `sha256:aa790b543e34f1792380d64a6bdc416a6c0abf278dfc69028ebe9122976c329c`;
  - checkpoint:
    `sha256:4d2e648d4b7f315c7874c18e842fa104e06a39f217cc357d0b3bc0674d41b686`;
  - supervisor:
    `sha256:9a0097df21a698e315b639a30fcafdff4c804e8c4a1368e353280bebb45e344d`.
- Root cause that changes the baseline: P14 binds resumption to
  `GoalObjectiveDigest` and `GoalResumeCount=1`, while Goal automation produced
  overlapping turns in one checkout and has been removed.

## Execution invariants

1. There is one writer and at most one heavy verification process.
2. No source file changes while a P13 run is active.
3. Every generated carrier is no-clobber and digest-addressed.
4. A failed step is diagnosed at its own boundary. It is not retried by
   repeating the whole plan.
5. A source-byte change invalidates P13/P14 evidence for older bytes.
6. The default restart path is graceful-only. `SIGTERM` remains an explicit
   per-attempt option; `SIGKILL` is inexpressible.
7. No application or task-runtime signal is sent before the Human Gate in
   `D5`.
8. Goal automation is not recreated. Exact-task continuation is explicit and
   acquires one durable resume lease.

## PlanItems

### D0 — settle the inherited writer

Acceptance:

- no `go test`, `go build`, P13, P14, checkpoint, or supervisor process from an
  earlier turn remains;
- all completed inherited outputs are classified as current or historical by
  source identity.

Return condition:

- if another writer appears, stop local mutation until it finishes; do not
  signal it.

### D1 — replace Goal coupling with exact-task resume intent

Method:

- replace Goal-objective fields with a checkpoint-generated exact-task resume
  intent digest;
- make `resume` prove exact task, exact repository, exact checkpoint intent,
  and single-writer transition;
- project `ExactTaskResumeCount=1` into P14 evidence;
- keep the deep-link, fallback wake, live-MCP challenge, and cleanup receipts
  unchanged in authority.

Acceptance:

- wrong task, repository, resume intent, duplicate writer, or wrong checkpoint
  fails closed;
- no field or test implies that Goal automation is required;
- focused restart and P14 carrier tests pass.

Return condition:

- if Codex cannot continue an opened exact task without a Goal, stop at the
  live rehearsal and report that exact host limitation. Do not restore Goal
  automation as a workaround.

### D2 — freeze and run one P13

Inputs:

- all D1 source and documentation edits complete;
- no other writer or heavy test process;
- frozen P13 environment: `GOMAXPROCS=1`, `GOFLAGS=-p=1`.

Method:

- capture one freeze-input carrier;
- verify it independently;
- execute exactly one consolidated P13;
- run the separate read-only freshness verifier once.

Acceptance:

- schema `haft.p13.acceptance-evidence/v3`;
- status `passed`;
- start and end identity equal;
- eleven suites pass;
- twelve bounded race packages run sequentially;
- no waivers.

Return condition:

- one failed suite returns only to its owning code boundary; after a repair,
  capture a new identity and run one new P13. Never run two P13 processes
  concurrently.

## P14 — installed candidate and live host acceptance

This section (`D3` through `D6`) is the canonical P14 request/oracle source.
The executable matrix has exactly 26 top-level scenarios: the existing twenty
runtime scenarios plus six code-graph/agent scenarios. `fresh_initialization`
retains its six nested init subcases; those subcases are not additional
top-level scenarios.

### D3 — prepare the exact P14 candidate

Method:

- build `haft`, checkpoint, and supervisor twice with identical parameters;
- require pairwise SHA-256 equality;
- materialize the three root-bound fixture carriers from the exact passing P13;
- generate and seal one prepared 26-scenario request;
- install the exact candidate;
- execute installed CLI and isolated init matrices without restarting Codex.

Acceptance:

- candidate/source/P13/fixture digests agree;
- installed executable digest equals the reproducible candidate digest;
- every installed-CLI scenario is captured as observation, not inferred from
  its prepared oracle;
- selected project remains read-only where the scenario requires isolation.

Return condition:

- any digest or root mismatch returns to the smallest producing step; source
  mismatch returns to D2.

### D4 — prepare one live restart attempt

Method:

- capture the exact Codex application generation, exact task-runtime
  generation, task ID, project, installed candidate, carriers, MethodRun, and
  resume intent;
- run failure-before-quit rehearsal;
- prove that failed preflight leaves both processes alive;
- prepare but do not submit the successful attempt.

Acceptance:

- checkpoint state is `prepared`;
- failure rehearsal is fail-closed;
- no signal or restart has occurred;
- the attempt is bound to one candidate and attempt number `1`.

### D5 — Human Gate: real host transition

Only this operation stops for the operator.

The brief must state:

- exact application and task-runtime PIDs/generations;
- graceful-only path and its timeout;
- whether `--allow-term` is proposed for either exact process;
- what stays alive after each failure;
- rollback/return condition;
- the weakest link: exact-task continuation depends on observed Codex host
  behavior after the old task runtime exits.

No comparison or Pareto front is implied: this is authority for one bounded
process transition, not a product-option choice.

### D6 — execute and verify live P14

After D5 authorization:

- submit one checkpoint attempt;
- open `codex://threads/<exact-task-id>`;
- in the explicitly continued exact task, acquire the one resume lease;
- bind the newly observed installed `haft serve` process;
- execute the frozen live MCP packet in declared order;
- ingest the task-history capture;
- run checkpoint verification and observe launchd-label removal;
- build the final installed-observation carrier.

Acceptance:

- checkpoint state `verified`;
- old application and task runtime are absent;
- new application and live MCP generations are newer than the checkpoint;
- exact-task resume count is `1`;
- fallback wake count is `0` or `1`;
- every frozen CLI/MCP/process scenario passes;
- no private nonce is copied into the public evidence carrier.

Return condition:

- a process-only failure remains in D6 for diagnosis;
- a source change returns to D2;
- a candidate mismatch returns to D3;
- no automatic second restart is scheduled.

### D7 — reconcile and close

- update `.context/haft-v9-remaining-execution.plan` from observed evidence;
- update the P14 checklist and README truth labels;
- run bounded spec/carrier/currentness checks and `git diff --check`;
- close the active MethodRun with exact evidence;
- report engineering readiness separately from packaging and release.

Acceptance:

- R7 and R8 claims cite current carriers;
- no draft/runtime/release distinction is collapsed;
- no commit, tag, push, or release occurs without a separate operator request.

## Current position

- [x] D0 inherited build/materialization writer settled.
- [x] D1 goal-free exact-task resume and repo-root-confined seal input.
- [ ] P0 contract closure: seven clauses are green; the `current
  specifications` clause remains at the separate human semantic/lifecycle
  gate described above.
- [x] source-stage MethodRun closed honestly as `partial`; this does not close
  P0 or D7.
- [ ] D2 one final P13 for the repaired source.
- [ ] D3 exact installed candidate and CLI capture.
- [ ] D4 prepared live attempt.
- [ ] D5 bounded Human Gate.
- [ ] D6 live P14.
- [ ] D7 reconciliation and closeout.

## Variance question

At every handoff ask only: does the observed artifact or process generation
match the exact baseline produced by the immediately preceding PlanItem? If
not, return to that producer; do not reinterpret an older passing artifact as
current evidence.
