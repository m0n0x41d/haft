# Haft v9 deterministic closeout plan

Status: active
EntityOfConcern: `entity:haft-v9-typed-memory` — Haft v9 typed project memory
Receiving use: close `R7` source acceptance and `R8` installed-runtime
acceptance without Goal automation, concurrent writers, or unreviewed process
signals
Execution owner: one agent turn at a time in this exact Codex task
Publication authority: absent; this plan does not authorize commit, tag, push,
release, or a public maturity claim

Current execution entry: use
`## 2026-08-02 — final path to READY_FOR_MANUAL_E2E` at the end of this file.
That section supersedes older uses of “current”, exact FPF coordinates, dirty
tree counts, installed-binary observations, and remaining-step summaries. It
does not supersede the frozen v9 product scope or any authority boundary.

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

### Historical P0 source-acceptance evidence — 2026-07-29, before the FPF 2ada refresh

Seven P0 clauses are source-complete and current-green. The compact MCP,
canonical-skill, and marker-honesty parts of the remaining clause are also
green, but the clause stays open because its `current specifications`
condition is false: the active typed-memory DecisionRecord and reviewed term
map still bind merge/split semantics that the frozen v2 SoftwareSystemSpec and
runtime schema exclude. Resolving that contradiction requires the separate
human semantic and SpecSection lifecycle gates. It is not a mechanical
closeout edit.

Historical source evidence:

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

### Current FPF refresh addendum — 2026-07-29

After the historical probe above, `data/FPF` advanced from `0990ff1` to
`2ada413629b846ef308222d16489a82cb5b40a71`. The aligned committed-index
candidate has spec digest
`sha256:00e8213ed4f2ab548ea16118b0559d72c1fc9c9baedd025891eeed160d5143af`,
database digest
`sha256:2df40bb9d4e754ee8e9d47ced1eb4926fbb941ac42880dc0a365c60fa4b04bc4`,
compiler edition `fpf-base-typeenv.cov2.v5`, and Base TypeEnv
`typeenv:sha256:effff65cae9eaf1aba287245df79c460fbeaee5f666dcaa7992bfeb251c1e35e`.

The former `0990ff1` V4 Base remains an exact replay artifact. Local-Practice
`1.4.0` is a non-binding candidate on the new Base; no `ProjectTypeEnvHead`
selection occurred.

This source change makes the earlier probe and every earlier P13/P14 carrier
historical for the refreshed bytes. Focused source, index, archive, candidate,
and token-gate checks are bounded source evidence only. D2 through D7 remain
open. No install, restart, P14, RC, tag, publication, or release authority
follows from this addendum.

An exact `task fpf-index` rerun is byte-idempotent at database digest
`sha256:2df40bb9d4e754ee8e9d47ced1eb4926fbb941ac42880dc0a365c60fa4b04bc4`
and retains the V4-to-V5 compatibility assessment. The refreshed embedded
query also restores the real “target system” concern to the source-owned
`SYSTEM-IN-CONTEXT` card with C.26, C.32.PAD, and E.18.NET witnesses.
`cmd/indexer`, `internal/fpf`, `internal/fpf/typeenvsql`, `internal/cli`, and
the token gate passed on these source bytes. These remain source-worktree
checks, not fresh P13 or installed P14 evidence.

Current semantic gates remain separate: RoleAssignment/performed-work
attribution, the active C.2.1 ClaimGraph invariant, and the pre-existing
merge/split versus alias-only contradiction.

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
- [ ] P0 contract closure: the FPF `2ada413` source/index integration and
  focused checks are complete, but fresh D2/P13 evidence is still open. The
  `current specifications` clause remains at separate human
  semantic/lifecycle gates for RoleAssignment/work attribution, the C.2.1
  ClaimGraph invariant, and merge/split versus alias-only semantics.
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

## Final source and commit-readiness addendum — 2026-07-29

This addendum supersedes older uses of “current” for the delegated source and
commit task. It does not close D2 through D7 and does not turn source evidence
into installed-runtime or release evidence.

### Exact source identities and commits

- starting parent: `432eb8572974b022a63e98972c2222d3d364c6b6`;
- materialized source commits, in order:
  `7e4ac03b`, `aff54c13`, `0de1cc17`, `fdd6ad96`, `5c989004`,
  `72d6e310`, `545053c2`, `c948e747`, `f2937047`, and `c6422f11`;
- `data/FPF`: clean detached source at
  `2ada413629b846ef308222d16489a82cb5b40a71`, equal to live
  `origin/main`;
- `FPF-Spec.md`: 11,789,814 bytes, SHA-256
  `00e8213ed4f2ab548ea16118b0559d72c1fc9c9baedd025891eeed160d5143af`;
- `internal/cli/fpf.db`: 73,265,152 bytes, 7,968 source units, schema 11,
  SHA-256
  `2df40bb9d4e754ee8e9d47ced1eb4926fbb941ac42880dc0a365c60fa4b04bc4`;
- Base compiler and ref: `fpf-base-typeenv.cov2.v5` and
  `typeenv:sha256:effff65cae9eaf1aba287245df79c460fbeaee5f666dcaa7992bfeb251c1e35e`;
- retained compatibility: `compared` against exact V4 Base
  `typeenv:sha256:28c7650b8933cbf6feb5d87965d48b4a8c7b80ae71c9c0ca4990d8ae7b6a36b6`,
  with 22 changes and no self-comparison;
- immutable V4 archive: 139,574 bytes, SHA-256
  `28c7650b8933cbf6feb5d87965d48b4a8c7b80ae71c9c0ca4990d8ae7b6a36b6`;
- Local-Practice candidate `1.4.0`: 81,823 bytes, SHA-256
  `8c4544660446dff9de3edb8ac93b6ccd9378e69e2909bb57c47d107a0900a2b7`,
  still non-binding;
- temporary, uninstalled source binary:
  `/tmp/haft-v9-candidate.0Vw6IR/haft`, SHA-256
  `b24c5237a41e71ce5ab5daa527a3c884854e8d4fc0ffd4365a3b35dfe9b7ca9a`.

### Exact verification result

- `gofmt` over changed Go files, `go mod tidy`, `git diff --check`,
  `go mod tidy -diff`, `go mod verify`, and `go list ./...`: PASS;
  module verification reported `all modules verified`;
- focused indexer, FPF, TypeEnv-SQL, embedding, config, CLI, and direct
  provider-normalization suites: PASS;
- predecessor compiler and envelope reads now share one read-only `sql.Tx`;
  deterministic path-replacement tests prove one opened snapshot, unknown
  compiler fail-closure, exactly-once close, and joined operation/close errors;
- optional candidate source-ground spoofing and fallback optional-batch
  retention have bounded regression coverage, including dedupe, ordering,
  caps, and combined omission/truncation metadata;
- pre/post `task fpf-index` database SHA-256 values were identical; nested
  `task fpf-verify` and a separate `task fpf-verify` passed. The handoff's
  alternate raw command had DB/spec arguments reversed and failed before
  verification; the live `Taskfile.yaml` order is authoritative;
- actionlint v1.7.7 and `go vet ./...`: PASS;
- the first golangci-lint v2.11.4 run found one test-only helper left in
  production; the helper moved to `_test.go`, focused tests passed again, and
  the final exact linter run reported `0 issues`;
- literal
  `GOMAXPROCS=2 GOFLAGS=-p=1 go test -count=1 ./...` reached the Go test
  binary's default 10-minute package deadline and failed
  `internal/cli` at 601.320 seconds while test 955 of 985 had been running for
  one second. `h-diagnose` probes refuted deadlock and order dependence: that
  test passed alone in 1.30 seconds, five repetitions passed, and neighboring
  order probes were stable;
- the established finite-budget normal run
  `GOMAXPROCS=2 GOFLAGS=-p=1 go test -count=1 -timeout=30m ./...` passed
  the complete package graph; `internal/cli` passed in 747.861 seconds. The
  stronger claim that the stock 10-minute command is green remains false;
- Open-Sleigh `mix format --check-formatted`,
  `mix compile --warnings-as-errors`, and `mix test`: PASS; 509 tests and
  2 properties passed, 3 integration-tagged tests were excluded by the suite;
- temporary candidate `version`, `carrier check`, and `doctor`: PASS.
  Doctor reported 7 passes, 0 failures, and one warning about pre-existing
  installed `haft serve` processes; no process was signaled;
- focused current race runs for the indexer snapshot, optional FPF source
  fallback, and embedding-provider validation: PASS. The earlier exact
  isolated DB race pass remains relevant because that package was unchanged.
  Full package-isolated hosted race proof is still absent; the earlier CLI
  race run remains a partial timeout, not a pass;
- final audit: 0 staged junk, 0 unexpected untracked files, 0 added maintainer
  paths, 0 shaped secrets, 0 live production callers of deleted standalone
  packages, and 0 SQLite journal/WAL/SHM leftovers.

### Cleanup, ports, and release-note truth

- commits `72d6e310` and `c948e747` remove the retired standalone
  agent/provider/tooling islands and obsolete repository hooks while retaining
  the current terminal presentation paths with live production callers;
- published-main safeguards absent from the divergent v9 ancestry were ported
  for recursive directory scopes, commission lifecycle fail-closure,
  passed-running-preflight replay, and recursive Open-Sleigh struct
  serialization. The Linux ARM64 Beam safeguard was already present and was
  not reimplemented;
- current Unreleased has exactly one each of `Changed`, `Added`, `Fixed`, and
  `Removed`; 457 bold bullet labels are unique. Numeric and hash claims match
  the tree. Race wording now says release aligns with CI's existing
  package-isolation policy and cites no uncarried timing measurements;
- root MethodRun
  `mpull-20260729-update-haft-to-the-newly-pulled-fpf-revision-2ad-613b5564`
  closed through the temporary source candidate with both hard gates
  satisfied, no waivers, and verification `partial`. The partial result keeps
  the stock test-budget mismatch and later release gates visible.

### Open semantic and release gates

- FPF `2ada413` still exposes unresolved source/spec relations around
  `BoundedContext`, the performer of Work, episteme slot assignments, and the
  breadth of the C.2.1 ClaimGraph invariant;
- the active DecisionRecord still includes merge/split semantics while the
  frozen runtime/spec contract supports context-bound `admit_alias` and
  `supersede_alias`. Selecting, narrowing, or superseding that binding remains
  a separate human semantic and SpecSection lifecycle gate;
- `docs` is clean at recorded gitlink
  `2053e86cb791cfb89c83b24d3756da39c76c472c`, but live `origin/main` is
  `cb4034307bd2bb3c909afa53f4065a41928b36f1`; the checkout is ahead by one and
  no advertised remote ref contains `2053e86`. Reproducible checkout and D7
  publication remain blocked until that commit is pushed under separate
  authority;
- fresh consolidated P13, installation, restart, installed P14, tag,
  publication, and release authority remain absent. No install, restart,
  signal, P14 run, push, tag, or release occurred in this source task.

Source implementation and atomic local commits are complete. The v9 release
verdict remains **NO-GO**.

---

## 2026-08-02 — final path to READY_FOR_MANUAL_E2E

Status: **ACTIVE EXECUTION ENTRY**.

This section is the current executable plan. It operationalizes P0 and D2-D7
for the source bytes that exist after the FPF refresh to `9a9a42e`. Older
sections remain historical evidence and rationale. When an older exact hash,
count, process observation, or “current position” conflicts with this section,
use this section and re-observe reality before acting.

### Operator objective

Bring Haft v9 to the state in which the only remaining engineering validation
is Ivan's manual E2E on:

1. one genuinely fresh project;
2. one project upgraded from the published v8.1.0 line;
3. one existing pre-release Haft project with a live project ledger.

The overnight result must not be “code seems ready”. It must be one exact,
clean candidate whose source, installed CLI, live MCP, Codex host, Claude host,
P13 evidence, P14 evidence, CI/race evidence, documentation, and handoff packet
all refer to the same candidate identity.

### FPF WorkPlan boundary

This is an A.15.2 `U.WorkPlan`: intended work, dependencies, acceptance
targets, stop conditions, and later variance checks. The document does not
perform Work, pass a test, approve a specification, create authority, or prove
release readiness. Every completed checkbox needs the evidence named below.

### Final result kind

The only successful final posture of this plan is:

```text
READY_FOR_MANUAL_E2E
```

That posture means all conditions below are true:

- [ ] no known source, fixture, parser, Query, token, lint, test, race, archive,
      migration, or runtime failure remains;
- [ ] P0's current specification contradiction is resolved through a real
      human decision and lifecycle act, not by silently editing prose;
- [ ] Git contains one clean immutable candidate commit on `dev`;
- [ ] `origin/dev` points to that exact commit and PR #99 has green current
      checks, including all twelve hosted race shards;
- [ ] one fresh P13 v3 carrier passes and remains fresh for that exact commit;
- [ ] the installed binary digest equals the exact candidate digest;
- [ ] the selected project ledger is schema 56 and passes integrity/currentness
      checks without another migration;
- [ ] one and only one accepted current-project MCP generation supplies the
      live observation used by P14;
- [ ] all 26 P14 scenarios pass across installed CLI, actual Codex MCP,
      Codex host/process, and actual Claude host;
- [ ] D7 truth carriers cite the exact evidence and contain no stale readiness
      claim;
- [ ] `.context/haft-v9-manual-e2e-handoff.md` gives Ivan short copy/paste
      scenarios for fresh, v8.1, and existing projects;
- [ ] no merge to `main`, tag, GitHub Release, or public release claim has been
      performed.

If any condition is false, the final posture is not
`READY_FOR_MANUAL_E2E`.

### Current observed baseline

Observed after Ivan restarted Codex on 2026-08-02:

- repository: `/Users/ivanzakutnii/Repos/active_projects/fpf/haft`;
- branch and HEAD: `dev`,
  `84157cfc4339fd5129a400314cc70c989bf2ede4`;
- remote: `origin/dev` currently equals that HEAD;
- worktree: 26 modified tracked paths, 0 untracked paths, nothing staged;
- current FPF revision:
  `9a9a42e4d154021ca3f7415e0009a4214832f65f`;
- current FPF spec digest:
  `sha256:135b2bd2ac115eddddd0508f7340431e66359ae2faf394065d1f3e4411023171`;
- source units: `8054`;
- current compiled base ref:
  `typeenv:sha256:ebb72c7705768c0de1d7d23e4cd064c42c7673a0418abc435362a62b98dbecb8`;
- current embedded DB digest:
  `sha256:7310141a2d8d9b91e6dc9282213858f15afa9c5f7bd9fab386972db7a693fa48`;
- refresh classification: `review_ready`, with a live token-fixture finding;
- installed executable:
  `/Users/ivanzakutnii/.local/bin/haft`;
- installed executable digest:
  `sha256:829e173085f3a512240143dc4a3a10cb0360675df87c9a21c1caa346e9806717`;
- installed version reports HEAD `84157cfc...` and `modified: true`; it is a
  dirty-source build, not an immutable candidate;
- project ledger contains exactly schema versions 1 through 56;
- `haft doctor` reports 7 passes, 0 failures, and one warning: multiple
  current-project `haft serve` processes;
- post-restart live `haft_query(action="status")` timed out after 60 seconds;
  this is a runtime failure observation, not source failure proof;
- current P13 manifest still pins schema 54, writer generation 54, FPF
  `0990ff1`, COV2 v4, and older project-head coordinates;
- P14 contract declares 26 scenarios but remains
  `prepared_not_executed`, with exact materialization `post_p13_pending`;
- PR #99 exists, but the current head has no hosted check rollup.

Re-observe this baseline at execution start. Do not preserve a stale value
merely because this plan records it.

### Scope freeze

In scope:

- repair the three known FPF review debts and any directly exposed regression;
- eliminate the multi-process startup-indexing herd before freezing a
  candidate;
- resolve P0's actual semantic/specification contradiction;
- refresh P13/P14 acceptance inputs to the exact current candidate;
- make all source, race, install, migration, archive, and runtime gates green;
- create intentional commits and push `dev` only when separately authorized;
- install and verify one exact candidate only when separately authorized;
- execute the exact D5 host transition only after its runtime-specific Human
  Gate;
- reconcile D7 and prepare the manual E2E handoff.

Out of scope:

- new v9 features unrelated to the frozen contract;
- dense/vector/hybrid retrieval claims;
- a new public merge/split surface unless Ivan explicitly selects that option;
- exposing TypeEnv, schema selection, internal coordinates, or raw
  MemoryChangeSet assembly to an end user;
- merging to `main`, tagging `v9.0.0`, publishing a GitHub Release, or claiming
  RC/release status;
- performing Ivan's manual E2E on his real old/new projects.

### Authority map

This WorkPlan itself supplies no execution or mutation authority.

| Effect | Required authority |
|---|---|
| read source, inspect state, run tests, build temporary candidates | ordinary authorized engineering work |
| repair code/tests/docs inside the accepted v9 closeout | the operator's separate implementation request |
| narrow/supersede the merge/split part of the active decision | direct operator selection from Gate S1 |
| approve/rebaseline/reopen affected SpecSections | exact SpecSection lifecycle request after S1 |
| commit and push exact semantic blocks to `dev` | direct commit/push request; never infer it from this plan |
| alter the live project ledger | separate exact migration request; current ledger is already schema 56, so no migration is presently planned |
| install the candidate | direct install request |
| stop/restart Codex or a live MCP process | exact D5 Human Gate with observed process generations |
| merge, tag, publish, or release | separate later release request after manual E2E |

Do not stop already-authorized reversible source work for a missing later
authority. Continue until the exact affected gate.

### Execution invariants

1. One writer and at most one heavy verification process.
2. Before every long command, prove no earlier `go test`, race, P13, P14,
   checkpoint, supervisor, or build writer is still active.
3. Never run P13 while source bytes can change.
4. Any release-relevant source-byte change invalidates older P13/P14 evidence.
5. Diagnose a failure at its owning boundary. Do not rerun the whole pipeline
   until that boundary is repaired.
6. Do not update a golden expectation merely to make a test pass. First prove
   the intended product behavior still holds.
7. Preserve unrelated user changes and submodule boundaries.
8. Use private `mktemp -d` workspaces for destructive-looking experiments.
   Never target the repository root, home directory, or a broad unresolved
   variable with removal.
9. Every generated evidence carrier is no-clobber and digest-addressed.
10. A clean source check is not installed-runtime evidence. A live response is
    not source acceptance. A plan is not performed Work.
11. Internal schema/TypeEnv terms may appear in maintainer evidence only. They
    must never become an end-user prompt or onboarding choice.
12. Update this section's execution ledger as work completes; record exact
    command, result, carrier path, digest, and candidate SHA.
13. Multiple `haft serve` processes for one project are a supported host
    topology. They may share one complete index, but they must never create
    concurrent rebuilds for the same project and source identity.

### Dependency order

```text
M0 current baseline
  -> M1 FPF review debt
  -> M1A startup-index single-flight
  -> M2 semantic/spec closure
  -> M3 acceptance refreeze
  -> M4 source qualification
  -> M5 clean immutable candidate + hosted CI/race
  -> M6 one fresh P13
  -> M7 exact install + installed CLI capture
  -> M8 D4/D5/D6 live P14
  -> M9 D7 reconciliation + manual E2E packet
  -> READY_FOR_MANUAL_E2E
```

No later step may borrow a pass from an earlier source identity.

## M0 — establish one current baseline

### Work

- [x] prove no inherited test/build/P13/P14/checkpoint writer remains;
- [x] capture `git status --short --branch`, HEAD, `origin/dev`, submodule
      revisions, installed executable path/digest, ledger schema list, doctor
      output, and current `haft serve` process generations;
- [x] copy the live project DB to a private backup and hash the copy before any
      later installed-runtime work;
- [x] verify the live DB has one row for every schema version 1 through 56,
      `PRAGMA integrity_check` returns `ok`, and no version above 56 exists;
- [x] record whether the live MCP status call succeeds, errors, or times out;
      do not retry repeatedly and create more processes;
- [x] record the exact current worktree path set and ownership.

### Suggested read-only commands

```bash
git status --short --branch
git rev-parse HEAD
git ls-remote origin refs/heads/dev refs/heads/main
git submodule status
haft version
haft doctor
shasum -a 256 /Users/ivanzakutnii/.local/bin/haft
sqlite3 /Users/ivanzakutnii/.haft/projects/qnt_e3149c17/haft.db \
  'select version from schema_version order by version;'
sqlite3 /Users/ivanzakutnii/.haft/projects/qnt_e3149c17/haft.db \
  'pragma integrity_check;'
ps -axo pid,lstart,command | rg 'haft serve'
```

### Acceptance

- one written baseline names exact bytes and process generations;
- the live DB is proved schema-56 and internally coherent;
- no migration is scheduled merely because an older observation said 55;
- a timed-out MCP remains an explicit M7/M8 runtime blocker.

### Return

If a writer is active, wait for or identify its owner. Do not signal an
unowned process. If DB integrity fails, stop live mutation and preserve the
backup; source work may continue.

## M1 — close the fresh-FPF review debt

Three known findings must be handled as product-behavior questions, not blind
golden updates.

### M1.1 Russian token/truncation fixture

Observed difference:

```text
russian-plan-work-evidence omitted_at_least = 47
frozen expectation = 49
```

Work:

- [x] reproduce with `bash scripts/fpf_query_token_gate.sh`;
- [x] compare exact candidate IDs, roles, ordering, match grounds, truncation
      basis, and token savings with the previous fixture;
- [x] prove the query still distinguishes WorkPlan, performed Work, and
      evidence and still returns the intended source-owned entry points;
- [x] if only the lower-bound count changed because current source membership
      changed, update the reviewed fixture to 47 and record why 47 is the
      correct lower bound;
- [x] if candidate identity or meaning drifted, repair Query behavior before
      updating the fixture;
- [x] require the frozen token-saving threshold to remain satisfied.

Acceptance:

- token gate passes from a clean cache and a warm cache;
- no candidate identity or semantic loss is hidden by the numeric update;
- the refresh report no longer contains `token_gate_failed`.

### M1.2 target-system Query regression

Observed failure:

```text
TestSQLiteQueryIndex_SourceNativeTiersAndExactHydration
query: "What is the target system here?"
missing: readme:practical_use_card:system-recognition
unexpected surviving result: spec:toc_row:a-2-9
```

Work:

- [x] inspect the current `SYSTEM-RECOGNITION` practical-use card and full
      direct pattern source;
- [x] compare current source fields and indexed terms with the Query probes;
- [x] determine whether source evolution changed the correct answer or exposed
      a ranking/indexing regression;
- [x] preserve the product claim: a plain target-system concern must recover a
      recognizable system-recognition entry rather than an incidental mention
      of “target” and “system” in another pattern;
- [x] fix the smallest source adapter, grammar, or retrieval boundary;
- [x] add a regression that would fail if A.2.9 wins only through incidental
      keyword overlap.

Acceptance:

```bash
go test ./internal/fpf \
  -run '^TestSQLiteQueryIndex_SourceNativeTiersAndExactHydration$' \
  -count=1 -v
```

passes and the exact intended source-owned system-recognition candidate is
present.

### M1.3 source-conformance snapshot

Observed difference:

```text
current FPF digest:
sha256:135b2bd2ac115eddddd0508f7340431e66359ae2faf394065d1f3e4411023171
reviewed snapshot:
sha256:7a1e56595a5bb850d6db571f1bd42bccec4a5a3859f480aadebae4e532557da9
```

Work:

- [x] inspect every source span used by the category-error conformance oracle;
- [x] classify semantic change versus byte-only/source-layout change;
- [x] when semantics are unchanged, repin the exact digest with a review note;
- [x] when semantics changed, adapt the implementation/specification and add
      regression evidence before repinning.

Acceptance:

```bash
go test ./internal/typedmemory \
  -run '^TestSourceConformanceCategoryErrorSourceSnapshotIsCurrent$' \
  -count=1 -v
```

passes on the reviewed current source.

### M1.4 refresh closure

Run:

```bash
go test ./cmd/fpf-refresh ./internal/fpfrefresh -count=1
task fpf-verify
bash scripts/fpf_query_token_gate.sh
git diff --check
```

Then run a read-only refresh check. Accept `no_change`, or an internally
coherent candidate with zero unresolved release-blocking Query/token/parser
findings. Do not re-apply an identical source merely to obtain a nicer status.

### M1 return

Any failure returns to M1's owning substep. Do not open P13, P14, or install
work while an M1 finding remains.

## M1A — eliminate the multi-process startup-indexing herd

### Confirmed failure mechanism

Current source has all four parts of the defect:

- `internal/cli/serve.go` starts a background `EnsureIndex` on every
  `haft serve` process;
- `internal/codeintel/flow.go` serializes only goroutines inside one process
  through package-level `indexMu`;
- Codex legitimately creates separate stdio servers for separate tasks and
  subagents, so one project may have several `haft serve` processes;
- `internal/codebase/incremental.go` gives each rebuild its own parser worker
  pool. Concurrent processes therefore multiply CPU and later contend while
  publishing to the same SQLite ledger.

Observed runtime evidence already includes four simultaneous indexers,
Tree-sitter and `go/parser` stacks, aggregate consumption of roughly ten CPU
cores, and downstream `SQLITE_BUSY`. Preserve that observation in the
execution ledger, but reproduce it with deterministic instrumentation rather
than treating CPU percentage alone as the acceptance oracle.

### Required product behavior

For one canonical project and one source identity:

1. any number of `haft serve` processes may start;
2. at most one process may execute the expensive index rebuild;
3. a contender must not start parser workers while another process owns that
   project's rebuild lease;
4. after obtaining the lease, the leader must recompute freshness before
   parsing. If the first process already published the current fingerprint,
   the next process returns `fresh_after_wait` without rebuilding;
5. startup is non-blocking: a startup contender records `deferred_busy` and
   continues serving from the last complete epoch when one exists;
6. a request that needs a fresh index may wait only within its context/deadline.
   On expiry it serves the previous complete epoch with an explicit degraded
   result, or returns a structured unavailable result when no complete epoch
   exists. It never launches a parallel rebuild;
7. an owner crash releases the operating-system lease. The next contender
   rechecks freshness and either skips or completes the rebuild;
8. two different projects remain free to index concurrently.

Sequential rebuilds are allowed only when the source fingerprint changed
between lease observations. "Single-flight" means one active rebuild per
project, not one rebuild forever.

### M1A.1 minimal computational core

Implement a closed coordination result instead of extending the ambiguous
`(bool, error)` contract with more flags. The pure policy must be able to
represent exactly these outcomes:

- current index already fresh;
- this caller rebuilt and published;
- another caller finished, then this caller found the index fresh;
- rebuild currently owned elsewhere and this caller deferred;
- the last complete epoch was retained after a bounded failure;
- no complete epoch is available.

Keep fingerprint comparison and the decision to rebuild in the functional
core. Keep file locking, clock/deadline handling, SQLite, logging, parsing, and
publication in the imperative shell. Do not make `serve.go` itself the
coordinator: every query path that calls `EnsureIndex` must cross the same
boundary.

### M1A.2 project-scoped operating-system lease

- [x] add one code-index rebuild coordinator keyed by canonical project
      identity and the exact ledger/index location;
- [x] place its lock carrier in Haft-owned per-project runtime state, not in
      the user's source tree, and derive it from canonical coordinates rather
      than an unchecked path supplied by the model;
- [x] use an OS advisory lock so process exit releases ownership; do not use a
      stale PID file as authority;
- [x] follow the repository's existing build-tagged Unix/Windows lock pattern;
- [x] open/create the carrier without following symlinks, require a regular
      operator-owned private file, and preserve safe permissions;
- [x] define one lock order between the in-process mutex, inter-process lease,
      fingerprint read, parse, and atomic publication;
- [x] hold the project lease for the complete expensive candidate build and
      publication, then release it on every return path;
- [x] recompute source and stored fingerprints only after lease acquisition;
- [x] preserve the previous complete epoch on parse, resolution, cancellation,
      lock, or publication failure.

Reuse a small existing safe-lock primitive only if its abstraction really
matches this lease. Do not couple code indexing to FPF refresh, onboarding,
spec migration, or embedding-sidecar locks merely because those packages also
call `flock`.

### M1A.3 startup and query integration

- [x] inject the project coordinator when constructing the code-intel service
      from the resolved serve/ledger binding;
- [x] route startup, explore, context, impact, node, flow, related, and every
      other `EnsureIndex` caller through the same coordinator;
- [x] let the background startup attempt use the non-blocking defer policy;
- [x] let request-time attempts use a context-bounded wait policy;
- [x] emit one concise structured log event naming project, outcome, wait
      duration, source fingerprint, and published epoch; never print lock
      internals or TypeEnv UX;
- [x] remove any log path that reports an ordinary follower as a rebuild
      failure;
- [x] keep SQLite busy handling as a secondary defensive boundary, not the
      single-flight mechanism.

Update `haft doctor` in the same block. More than one current-project server is
normal for multi-task hosts and must not be diagnosed as a defect by process
count alone. Doctor should show the bounded process set and warn only about a
specific actionable condition such as a stale executable, orphaned transport,
unsafe lock carrier, rebuild lease held beyond its declared bound, or a failed
index publication.

### M1A.4 deterministic verification

Add tests at the lowest boundary that can prove each claim:

- [x] pure policy table covers every closed coordination result and forbids a
      follower from entering parse/publication;
- [x] same-process concurrent calls still produce one rebuild;
- [x] a multi-process integration fixture starts at least four contenders
      against one temporary project and one ledger, releases them at one
      barrier, and proves exactly one parser/build entry and one publication
      for the unchanged source identity;
- [x] all followers return fresh/deferred results without parser workers and
      without `SQLITE_BUSY`;
- [x] a leader paused after lease acquisition proves that followers do not
      parse; after release they recheck and skip;
- [x] a leader terminated before publication proves automatic lease release,
      preservation of the previous complete epoch, and one successful retry;
- [x] a source mutation while a follower waits permits the necessary second
      *sequential* rebuild and never an overlapping rebuild;
- [x] two distinct project identities can rebuild concurrently;
- [x] cancellation/deadline, symlink carrier, unsafe mode/owner, and Windows
      fallback behavior are covered;
- [x] a real stdio harness starts several `haft serve` processes for the same
      project, obtains responsive MCP status from each, and observes no herd or
      SQLite publication contention;
- [x] `go test -race` covers the coordinator and code-intel packages.

The multi-process assertion must use test-only barrier/event carriers or an
equivalent deterministic observer. Do not make a wall-clock sleep, sampled CPU
percentage, or absence of a log line the sole proof.

Suggested focused qualification after implementation:

```bash
go test ./internal/codeintel ./internal/codebase ./internal/cli -count=1
go test -race ./internal/codeintel ./internal/codebase ./internal/cli -count=1
```

Also run the new real multi-process test by its exact name with `-count=20`
before the full suite. Record peak simultaneous rebuild count, publications,
follower outcomes, and any SQLite busy errors.

### M1A acceptance

- one project/source identity has measured peak active rebuild count `1` with
  at least four simultaneous `haft serve` contenders;
- aggregate parser concurrency is bounded by one configured worker pool;
- followers remain responsive and perform no parsing;
- no `SQLITE_BUSY` is produced by code-index publication in the fixture;
- a crashed owner is recoverable without a stale-lock cleanup ritual;
- distinct projects retain parallelism;
- `haft doctor` no longer calls legitimate per-task servers duplicates;
- README/CHANGELOG describe automatic shared indexing without exposing lock
  mechanics as user setup;
- all focused tests and the race run pass.

### M1A return

Any failure returns to the coordinator, lease carrier, freshness recheck, or
atomic publication boundary that owns it. Do not mask the defect by disabling
startup indexing, reducing the global worker count, increasing SQLite's busy
timeout, or telling users to keep only one Codex task open. M2 and every
candidate-freeze step remain blocked until M1A passes.

## M2 — close P0 semantic and specification truth

### Exact current contradiction

The active typed-memory decision promises explicit append-only merge/split
history. The v2 public runtime and SoftwareSystemSpec expose only
context-bound `admit_alias` and `supersede_alias`. Both cannot be the current
v9 product contract.

The fresh FPF source also requires a bounded re-review of:

- actual performer and `RoleAssignment` claims for performed Work;
- the breadth of the C.2.1 ClaimGraph requirement;
- merge/split versus alias-only identity behavior.

### Gate S1 — human product/specification choice

Affected operation: P0 closure and every later P13/release claim.

Real options:

1. **Narrow v9 to alias-only; defer public merge/split to v9.x
   (advisory recommendation).**
   - Changes: supersede or narrow only the merge/split portion of the active
     decision; align term map, specifications, tests, P13 claims, and release
     notes with `admit_alias|supersede_alias`.
   - Unchanged: internal historical storage and old records remain readable;
     no public merge/split route is invented; all other frozen v9 features
     remain in scope.
   - Immediate result: smallest path to an honest v9 contract.
   - Weakest link: future merge/split needs a new reviewed contract and
     migration rather than being silently implied by v9.

2. **Implement and expose reviewed merge/split in v9.**
   - Changes: public domain algebra, exact authority path, CLI/MCP UX,
     migrations, replay/conflict semantics, P13/P14 scenarios, documentation,
     and manual E2E scope.
   - Unchanged: no implicit/rank-based merge is permitted.
   - Immediate result: materially larger v9 scope.
   - Weakest link: this violates the accepted scope freeze and creates a new
     high-risk release surface late in closeout.

3. **Defer v9 release without changing either contract.**
   - Changes: none.
   - Immediate result: P0, P13, and release remain blocked.
   - Weakest link: no return date or completed v9 candidate.

No useful Pareto comparison exists beyond the already frozen selection policy:
preserve semantic honesty and minimize late irreversible scope. Option 1 is the
only option on the present critical path.

The plan and this recommendation are not the choice. Record Ivan's natural
language selection as the direct operator request, then route the exact
decision effect through `h-decide`. Do not interpret silence as Option 1.

### M2 specification work after S1

- [ ] query/inspect the exact current FPF source for the three semantic topics;
- [ ] build one bounded semantic fanout list across the active decision,
      SoftwareSystemSpec, TargetSystemSpec, term map, skills, P13 claims, P14
      oracles, README, CHANGELOG, and release notes;
- [ ] apply only the selected semantic change;
- [ ] use `h-spec` for affected SpecSection lifecycle work;
- [ ] obtain the exact human lifecycle act where required;
- [ ] preserve historical decision text as historical rather than rewriting
      what was previously selected;
- [ ] prove no end-user surface asks about TypeEnv, schema, or internal memory
      model selection.

### M2 acceptance

- the current decision, current specs, runtime algebra, public tools, skills,
  and release claim say the same thing;
- RoleAssignment/Work wording attributes action only to an actual system under
  an obtaining assignment;
- ClaimGraph requirements are no broader than current source support;
- P0's `current specifications` checkbox can be checked with exact carrier
  evidence;
- lifecycle state is current, not merely edited Markdown.

### M2 return

If S1 is unanswered, continue every unrelated source/test task that does not
depend on the choice, but do not claim P0 closure or start final P13.

## M3 — refreeze P13 and P14 inputs

The existing manifest is structurally valid but semantically stale.

### M3.1 derive, never guess, the new P13 identity

- [ ] update `required_schema_version` from 54 to the source-current 56;
- [ ] derive writer generation separately. Migrations 55/56 do not by
      themselves imply writer generation 56; retain 54 only after reading the
      exact writer capability row and source contract;
- [ ] replace FPF `0990ff1`/COV2 v4 coordinates with the exact current
      `9a9a42e`/COV2 v5 coordinates;
- [ ] update spec, DB, compiler, local-practice, and integration-lock digests;
- [ ] recapture the exact selected project-head, profile, receipt, graph, and
      predecessor coordinates from a consistent schema-56 snapshot;
- [ ] update stale schema-54 and old-FPF prose in the P13 README and validators;
- [ ] keep the empty excluded-Go-package requirement;
- [ ] update all exact error messages and tests that intentionally bind the
      schema/source identity.

Do not change writer generation, predecessor identity, or selected head merely
to make a manifest validator pass.

### M3.2 use an isolated frozen P13 home

Do not mutate the live ledger to prepare source acceptance. It is already
schema 56. Create a private isolated `HOME`, snapshot the live DB into it,
verify the copy, and use that copy for freeze capture and P13. Preserve the
exact project ID and project-root binding expected by the harness.

The isolated basis must contain every path required by
`internal/p13acceptance/README.md`, including ignored `.agents`, `.context`,
project carriers, and the copied project DB.

### M3.3 structural and freeze checks

Run:

```bash
go test -count=1 ./internal/p13acceptance \
  -run '^TestP13ManifestStructureAndAnchors$'

go test -count=1 ./internal/p14acceptance \
  -run '^TestP14RequestOracleContract$'

HAFT_P13_CAPTURE_FREEZE_INPUT=1 \
  go test -count=1 -v ./internal/p13acceptance \
  -run '^TestP13CaptureFreezeInputCandidate$'

HAFT_P13_VERIFY_FREEZE_INPUT=.context/p13/<CURRENT_FREEZE_CARRIER>.json \
  go test -count=1 -v ./internal/p13acceptance \
  -run '^TestP13VerifyFreezeInputCandidate$'
```

Copy only the verified `freeze_input` object into the pending manifest, then
repeat the structural and freeze verification.

### M3 acceptance

- manifest status is `frozen_for_execution`;
- schema is exactly 56 and writer generation is independently correct;
- FPF/source/index/local-practice coordinates equal the current publication;
- freeze carrier and manifest identities match;
- P14 still has exactly 26 top-level scenarios and
  `post_p13_pending` materialization;
- no P13 or P14 pass is claimed yet.

## M4 — make the complete source tree green

### M4.1 mechanical integrity

- [ ] `gofmt` every changed Go file;
- [ ] `go mod tidy -diff`;
- [ ] `go mod verify`;
- [ ] `go list ./...`;
- [ ] `git diff --check`;
- [ ] verify no SQLite journal/WAL/SHM artifact is left in the tree;
- [ ] verify no unexpected generated or secret-shaped file is present.

### M4.2 exact source gates

Run sequentially:

```bash
task fpf-verify
bash scripts/fpf_query_token_gate.sh
go test -count=1 -timeout=90m ./...
go vet ./...
golangci-lint run \
  --timeout=10m \
  --max-same-issues=0 \
  --max-issues-per-linter=0
npm --prefix packages/haft-pi run typecheck
npm --prefix packages/haft-pi test
actionlint .github/workflows/*.yml
bash -n install.sh scripts/*.sh scripts/release/*.sh
bash scripts/release/smoke-archive.sh \
  <exact-v9-candidate-archive> <version> <short-sha>
task test:race
git diff --check
```

The Open-Sleigh/Elixir/OTP gates were removed from this current command set by
`dec-20260805-haft-v9-remove-built-in-execution-contour-a1eb55b6`. Historical
execution-ledger entries below remain provenance for older source identities;
they do not qualify the v9 boundary. The archive smoke is now the negative
gate: it must reject every Open-Sleigh, ERTS, Elixir, or BEAM payload.

`task test:race` is the bounded P13 critical profile: require all 37 exact
tests across its 12 manifest-owned packages to pass. Do not run a full local
race before P13. `task test:race-full` is an explicit, very slow reproduction
of the hosted CI contour only. CI owns the twelve-shard aggregate; release
requires the successful unexpired aggregate for its exact candidate SHA rather
than executing those shards again.

### M4.3 fresh and upgrade/archive probes

Before P13, exercise temporary isolated projects with the candidate built from
the current worktree:

- [ ] fresh `haft init --codex --local`;
- [ ] fresh `--core-only`, `--claude`, `--codex`,
      `--codex --mcp-only`, `--agents`, and `--all`;
- [ ] published v8.1 fixture forward-upgrade;
- [ ] v8.1 backup/restore and failed-upgrade recovery;
- [ ] existing schema-55 to schema-56 migration on a copy;
- [ ] repeated init idempotency and preservation of foreign/outside-marker
      content;
- [ ] default memory and supported singleton profile bootstrap without an
      internal schema question;
- [ ] archive smoke through `scripts/release/smoke-archive.sh`.

These are source/candidate probes, not Ivan's later manual E2E.

### M4 acceptance

Every command passes on one unchanged source identity. Any skipped test must be
an explicitly intended suite exclusion already declared by the repository;
new waivers are not accepted.

### M4 return

Repair the smallest owning boundary, rerun its focused test, then rerun M4 from
the first gate whose dependency bytes changed. Do not start P13 on red M4.

## M5 — create, commit, push, and qualify one immutable candidate

### M5.1 final diff audit

- [ ] inspect every changed path and assign it to a semantic commit block;
- [ ] verify `data/FPF`, `internal/cli/fpf.db`, integration lock, and
      local-practice candidate are one coherent publication;
- [ ] verify README, CHANGELOG, `.github/release-notes/9.0.0.md`, and this plan
      describe current behavior and do not claim runtime proof;
- [ ] verify the docs gitlink is reachable from its advertised remote;
- [ ] verify no user change is omitted or accidentally absorbed.

### M5.2 intended commit blocks

Use exact-path staging, not blind `git add -A`. Recompute these blocks from the
actual diff:

1. fresh-FPF refresh behavior, parser/query repair, tests, FPF submodule,
   embedded DB, integration lock, local-practice rebase, README/CHANGELOG;
2. P0 semantic/spec closure and current acceptance-contract refreeze;
3. closeout/release documentation and manual-E2E preparation.

Each commit must:

- use Ivan's configured Git name/email;
- contain no OpenAI/Codex attribution or task reference;
- pass `git diff --cached --check`;
- preserve submodule identity;
- have its owning focused tests green.

Do not commit or push until the current operator request explicitly authorizes
those effects.

### M5.3 clean candidate

After commits:

```bash
git status --short --branch
git diff --exit-code
git diff --cached --exit-code
git fsck --no-dangling
```

The exact candidate identity is the full clean HEAD SHA. Record:

- candidate SHA;
- tree SHA;
- FPF revision;
- embedded DB digest;
- executable digest from two deterministic builds;
- P13 manifest digest;
- submodule identities.

### M5.4 push and hosted qualification

When push authority exists:

```bash
git push origin dev
```

Require `origin/dev` to equal the candidate. PR #99 must receive current checks.
If the PR push does not trigger CI, dispatch the non-P13 CI workflow on `dev`
and record why:

```bash
gh workflow run ci.yml --ref dev -f run_p13=false
```

Wait for:

- Test & Build;
- all twelve race shards and the aggregate;
- exact lint;
- FPF verification and token gate.

No historical workflow run may satisfy this gate.

### M5 acceptance

- clean local candidate equals `origin/dev`;
- all hosted checks are green for that SHA;
- no source change occurs after the candidate is named.

## M6 — execute exactly one final P13

### Preflight

- [ ] M0-M5 all pass;
- [ ] no other heavy process or writer exists;
- [ ] candidate SHA and freeze carrier are immutable;
- [ ] isolated P13 `HOME` and repository basis match the candidate;
- [ ] `GOMAXPROCS=1` and `GOFLAGS=-p=1`;
- [ ] no prior P13 process is running.

### Execute

```bash
HAFT_P13_RUN_CONSOLIDATED=1 \
  go test -count=1 -timeout=12h -v ./internal/p13acceptance \
  -run '^TestP13ConsolidatedAcceptance$'
```

Do not reuse an older suite unless the v3 reuse contract independently accepts
the exact dependency digest. Prefer a full run for the final candidate.

Immediately verify the emitted carrier:

```bash
HAFT_P13_VERIFY_ACCEPTANCE_EVIDENCE=.context/p13/<P13_EVIDENCE>.json \
HAFT_P13_VERIFY_ACCEPTANCE_DIGEST=sha256:<P13_CARRIER_DIGEST> \
  go test -count=1 -v ./internal/p13acceptance \
  -run '^TestP13VerifyAcceptanceEvidenceFresh$'
```

### Acceptance

- schema `haft.p13.acceptance-evidence/v3`;
- status `passed`;
- start/end identity equal the candidate;
- all required suites and gates pass;
- twelve bounded race packages accounted for;
- no waiver;
- freshness verifier passes;
- carrier path and digest recorded in this plan.

### Return

If one suite fails, preserve the failed carrier, diagnose only that boundary,
repair it, create a new clean candidate, invalidate the old evidence, and run
one new final P13. Never run two P13 processes concurrently.

## M7 — build and install the exact P14 candidate

### M7.1 reproducible build

Build `haft`, checkpoint, and supervisor twice with identical parameters in
separate private directories. Require pairwise SHA-256 equality.

Record the exact candidate executable digest. It must not report
`modified: true`.

### M7.2 materialize and seal P14 input

Follow `internal/p14acceptance/README.md` using the fresh P13 carrier:

1. materialize runtime fixtures with
   `TestP14MaterializeRuntimeFixtureCarriers`;
2. exercise FPF Query and memory-read generators against the candidate;
3. generate the prepared input with
   `TestP14GeneratePreparedRequestOracleInput`;
4. seal it with `TestP14SealPreparedRequestOracleCarrier`;
5. require all 26 scenario builders and binding groups.

Every fixture must bind the candidate, P13, selected project, FPF, graph,
skills, instructions, MCP config, and exact future physical roots.

### M7.3 live ledger and install gate

The current live ledger is already schema 56. Do not migrate it again.

Before install:

- [ ] compare the live DB schema list and integrity result with M0;
- [ ] preserve the M0 backup and current hash;
- [ ] prove no pending migration exists for the candidate;
- [ ] obtain direct install authority.

Install only the exact reproducible candidate. Then require:

```bash
command -v haft
haft version
shasum -a 256 /Users/ivanzakutnii/.local/bin/haft
haft doctor
```

The installed digest must equal the candidate digest.

### M7.4 installed CLI capture

Execute:

```bash
HAFT_P14_CAPTURE_INSTALLED_CLI=.context/p14/<SEALED_PREPARED_CARRIER>.json \
HAFT_P14_INSTALLED_CLI_CANDIDATE=/Users/ivanzakutnii/.local/bin/haft \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14CaptureInstalledCLI$'
```

Require all installed-CLI observations and before/after no-write digests.

### M7 acceptance

- installed path/digest equals the clean candidate;
- project DB remains coherent schema 56;
- installed CLI capture passes;
- no Codex/Claude restart or live-P14 claim has occurred yet.

## M8 — prepare, authorize, and execute live P14

### M8.1 repair the process-generation boundary

The current observation contains many current-project `haft serve` processes
and a timed-out status call. Do not kill them by pattern.

- [ ] map every PID to parent process, executable, CWD, start time, and host;
- [ ] distinguish currently connected transports from orphans;
- [ ] let the P14 checkpoint own the exact transition;
- [ ] prove a failure-before-quit rehearsal leaves the current host alive;
- [ ] prepare one candidate-bound attempt, number 1.

### M8.2 Gate D5 — exact host transition

At runtime, publish a Human Gate Brief containing:

- exact old Codex application PID/start generation;
- exact task-runtime PID/start generation;
- exact currently connected MCP PID/start generation;
- candidate path and digest;
- graceful-only quit path and timeout;
- whether `--allow-term` is proposed for either exact process;
- what remains alive after each failure;
- rollback/return condition;
- weakest link: continuation depends on observed host behavior after the old
  exact task runtime exits.

The brief is not authority. Ivan's direct natural-language approval of that
exact attempt is required. Generic permission in this WorkPlan is insufficient.
Never use `SIGKILL`.

If Ivan is unavailable, finish every other preparable item and stop only at
this gate. Do not fabricate `READY_FOR_MANUAL_E2E`.

### M8.3 execute Codex P14

After D5 approval, execute sections 3-5 of
`.context/haft-v9-p14-live-rehearsal-checklist.md` exactly:

- submit one checkpoint attempt;
- reopen the exact task;
- acquire one resume lease;
- bind the one new installed `haft serve` process;
- call live status and fulfill the private challenge;
- generate the exact Codex MCP request packet;
- execute its calls in the declared order in the resumed task;
- ingest the actual task capture;
- verify cleanup and launchd-label removal.

Acceptance includes:

- old application/task generations absent;
- new application/MCP generations newer;
- exact-task resume count 1;
- fallback wake count 0 or 1;
- no extra prompt-turn tool calls;
- one exact proactive EntityOfConcern establishment only when the sealed
  receiving use requires durability;
- no internal schema/TypeEnv fields exposed in that task-level call;
- live status succeeds through the actual resumed transport.

### M8.4 execute actual Claude proof

Use the exact same installed candidate and fresh P13/P14 basis. Follow the
Claude proof request in `internal/p14acceptance/README.md` and run
`TestP14CaptureActualClaudeHostProof`.

Require:

- actual Claude process/session generation newer than its captured baseline;
- exact installed executable and project;
- real Claude transcript/history, not a Codex import or fixture;
- no hung or extra calls;
- current skills/instructions/MCP schema.

### M8.5 finalize P14

Create the finalization request and run:

```bash
HAFT_P14_FINALIZE_INSTALLED_OBSERVATION=.context/p14/<FINALIZATION_REQUEST>.json \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14FinalizeInstalledObservationCarrier$'
```

### M8 acceptance

- final installed-observation carrier passes all 26 scenarios;
- installed CLI, actual Codex MCP, Codex host, and Claude host are all present;
- every runtime binding points to the exact candidate and fresh P13;
- private nonce/receipt material is absent from public evidence;
- `haft doctor` reports no failed check and no ambiguous current-project MCP
  generation used by the evidence;
- no source bytes changed.

## M9 — D7 reconciliation and manual-E2E handoff

### M9.1 reconcile project truth

- [ ] update this plan's execution ledger with exact P13/P14/CI/race evidence;
- [ ] update the older current-position text or clearly mark it historical;
- [ ] update README readiness labels;
- [ ] update CHANGELOG and `.github/release-notes/9.0.0.md`;
- [ ] update the P14 checklist status from prepared to the exact observed
      disposition;
- [ ] run bounded spec/carrier/currentness checks;
- [ ] close any active MethodRun with exact changed files, gates, and evidence;
- [ ] run `git diff --check`;
- [ ] if D7 changed release-relevant source bytes, return to M5 and invalidate
      P13/P14. Prefer evidence-only carrier updates that the acceptance identity
      already includes and plan their timing before M6.

### M9.2 non-publishing package checks

- [ ] validate public release carriers for the candidate tree;
- [ ] build local versioned archives with release-equivalent flags;
- [ ] run `scripts/release/smoke-archive.sh`;
- [ ] verify checksums;
- [ ] prove fresh install from the archive;
- [ ] prove the v8.1 upgrade/restore fixture from the archive;
- [ ] record that the hosted release workflow still requires current
      `origin/main` and is intentionally deferred until after manual E2E.

Do not merge, tag, or publish to make the release workflow green.

### M9.3 create Ivan's manual packet

Create `.context/haft-v9-manual-e2e-handoff.md` with:

1. exact candidate SHA, binary path/digest, FPF revision, P13/P14 carrier paths
   and digests, and tested host generations;
2. a ten-minute fresh-project scenario;
3. a v8.1 backup/upgrade/rollback scenario;
4. an existing-project re-init/currentness scenario;
5. exact expected human-visible output and failure symptoms;
6. copy/paste commands to gather logs without mutating or exposing secrets;
7. a result table with `PASS`, `FAIL`, and notes;
8. the rule that any source fix after a manual failure invalidates current
   P13/P14 and returns to the owning step.

The fresh-project manual scenario must prove:

- `haft init`/the chosen host projection installs project memory by default;
- no user is asked to choose or understand TypeEnv, schema, or memory model;
- onboarding reports a readable ready/review state;
- FPF Query retrieves current source;
- the agent uses memory proactively when a concrete cross-session/handoff/
  audit receiving use exists;
- the first EntityOfConcern uses the task-level surface without raw internal
  coordinates;
- ordinary ephemeral reasoning does not persist noise;
- init is idempotent and preserves project-owned content.

The v8.1 manual scenario must prove:

- backup exists before upgrade;
- published v8.1 project identity and artifacts remain readable;
- explicit migration is recoverable and idempotent;
- re-init installs current host surfaces and default memory without a schema
  questionnaire;
- old decisions/notes/specs remain readable with honest historical posture;
- rollback instructions restore the backed-up v8.1 state.

The existing-project scenario must prove:

- no duplicate project identity or entity is created;
- existing profile/default memory is reused or repaired;
- host restart connects one current binary/process generation;
- `haft doctor` and live status are readable;
- source refresh warnings are prominent but supported fresh FPF remains
  embedded;
- code graph and structured memory stay separate, useful orientation surfaces.

### M9 final acceptance

Only after all prior conditions pass, write exactly:

```text
Engineering posture: READY_FOR_MANUAL_E2E
Release posture: NOT_RELEASED
Remaining engineering validation: Ivan's manual fresh/v8.1/existing-project E2E
Remaining release acts: merge main, validate, annotated tag, publish
```

## After Ivan's manual E2E

This plan does not authorize these actions.

If manual E2E passes:

1. reconcile any evidence-only notes without changing candidate bytes;
2. obtain merge authority and merge the exact `dev` candidate to `main`;
3. run the non-publishing release validation workflow on exact `main`;
4. obtain tag authority and create annotated `v9.0.0`;
5. require successful tag-validation artifacts and checksums;
6. obtain publication authority;
7. publish the sealed GitHub Release.

If manual E2E exposes a source defect:

1. record the exact scenario and observation;
2. diagnose and repair the owning boundary;
3. invalidate the old candidate, P13, P14, CI, archive, and manual packet;
4. return to the earliest affected M-step;
5. never patch the release artifact independently of source.

## Execution ledger

Update this table during performed Work. A checkbox or prose statement without
an evidence reference is not completion.

### 2026-08-03 — implemented v9 source-blocker closeout

The operator-approved source repairs are complete. This entry records source
evidence only; it does not claim P13, installed-runtime P14, clean-candidate,
hosted-CI, SpecSection lifecycle, publication, or release authority.

- Project-profile bootstrap now bounds detector evidence to 64 representative
  paths while preserving the exact total and truncation signal. A valid
  software repository no longer becomes a second document scope merely
  because it contains ordinary Markdown/PDF/DOCX artifacts; explicit document
  manifests remain independent document-system evidence. The reported UUID
  `019fb9c0-8c3e-7282-be6f-615698e685e6` was a Codex thread ID for
  `licensing_platform`, not a Haft project or scope ID. The exact repository
  probe now returns one supported `software` scope with 18 observed signals.
- Status no longer performs a repo-wide tree-sitter scan per missing binding.
  One immutable published symbol corpus is reused for the request, startup
  indexing is coordinated across processes, status has a 45-second request
  bound with a 2-second corpus-read budget, and an unavailable drift
  projection degrades only that projection rather than hanging the whole
  cockpit. The local candidate returned status in approximately 23.4 seconds
  against the formerly hanging repository.
- The v9 public identity-change contract is alias-only. Public entity
  merge/split is absent from the closed v9 algebra and is recorded as v9.1
  roadmap scope in `.context/haft-version-9-1-roadmap.md`.
- FPF-facing Work semantics now name the actual performing System and the
  covering RoleAssignment, use the canonical performed-under-assignment
  relation, and preserve the exact ClaimGraph constitution boundary.
- `haft spec status` now reports workflow disposition separately from section
  health. Six sections still require an explicit human lifecycle act before a
  new baseline may be claimed:
  `SS.constraints.authority.001`, `SS.functional.profile.001`,
  `SS.interfaces.hosts.001`, `SS.interfaces.profile.001`,
  `SS.procedural.profile-onboarding.001`, and
  `SS.procedural.typeenv.001`.
- Source identity at that checkpoint was FPF revision
  `9dd9215969126625d449a40e8ca4d1df9ac903f8`, embedded DB
  `sha256:9c72d5b34dc9816dd6e57eed27dc26b6ff5a9df6cad496e5ce062ed409fda3f8`
  with 8,092 units, and Base TypeEnv
  `typeenv:sha256:13e53b3cfeb7402bcbe1673df5a74a0b96173db064a891140a3249f76ae6a61d`.
- The race aggregate at that checkpoint,
  `/tmp/haft-v9-race-20260803-v2.json`, passed all
  12 shards and all 239 commands with zero failed packages. Its SHA-256 is
  `24cb5b2ab2bfc5ee4075ac9c812d8dccaf559524b60f3462a34abe0d91caa0a7`
  and its plan digest is
  `sha256:db86664695564843dba627bca4f4f198ced5f0d9269fff96c43f70acc5cafcdc`.
  The same plan digest as the failed oversubscribed attempt proves that the
  repair changed runner capacity, not test scope. Automatic concurrency on
  this 12-CPU host is now four race commands, each retaining `-cpu=2` and
  `-parallel=2`.
- Pinned `golangci-lint v2.11.4` reported zero issues; `go vet ./...`,
  `task fpf-verify`, `go mod tidy -diff`, `go mod verify`, `go list ./...`,
  actionlint v1.7.7, shell syntax, and `git diff --check` passed. The full
  ordinary suite passed before the final race-runner-only adjustment; focused
  ordinary tests for `cmd/race-shard` and `internal/cli` changes and the full
  final-byte race aggregate passed afterward. A redundant second full ordinary
  run was stopped at the operator's request and is not cited as evidence.
- The locally built product candidate at that checkpoint was
  `/tmp/haft-v9-candidate.6vyGXt/haft`, SHA-256
  `b0aa717b3b64d7f39b6f03fbaea097568b756301f2eac33a9e32d9cb5fec010b`;
  the final subsequent edits affect only `cmd/race-shard` and test code, not
  `cmd/haft` product bytes. It is not installed-runtime evidence.

Known source defects in the approved four-point repair plan plus the added
project-profile regression are closed. Remaining v9 gates are effects rather
than another code-fix loop: select the exact project TypeEnv and run P13;
perform the six explicit SpecSection lifecycle acts; authorize installation
and exact host transition; then run installed P14 and manual E2E.

### 2026-08-04 — latest FPF integration and exact-snapshot M4 qualification

This entry supersedes the current FPF and local M4 coordinates above without
rewriting their historical evidence. It records source and disposable-probe
evidence only. It does not claim a selected `ProjectTypeEnvHead`, P13,
installed-runtime P14, a clean commit, hosted CI, SpecSection lifecycle acts,
manual E2E, publication, or release authority.

- FPF refresh now accepts the latest structurally coherent upstream
  publication instead of allowing source-sensitive compatibility fixtures to
  veto it. The resulting exact publication is FPF revision
  `8b727cba9e893a467b82aab9da84fb7d6d945480`, specification digest
  `sha256:c03441b51561922ec7bdbcb0c76bb3d26bd5c904ac9c68364625cbe454af3a42`,
  embedded DB
  `sha256:e10be4501c3f2f94754fae64a72642dcbbbb30433204617979fb89348df946e5`
  with 8,092 units and zero diagnostics, Base TypeEnv
  `typeenv:sha256:36e74f905065438532da7d486099c6a745dc82190f46c9f8958d13e3c44d2786`,
  and compiler schema `fpf-base-typeenv.cov2.v5`. The retained Local-Practice
  `1.5.0` digest is
  `sha256:1776b46c80229925c251623195680bf45f3c89c76f2791a4d215272a34ec2248`.
  The refreshed integration lock pins generator identity
  `fpf-refresh-inputs/v1:sha256:6ec021d2cad0e4f5146680c4493e2d951b07a01d51d6c400d5ebe701695ca10e`.
  All 39 semantic deltas were reviewed as source changes rather than hidden as
  a mechanical commit bump.
- Published v8.1 skill and Codex TOML carriers are recognized by their exact
  namespace and structure during v9 upgrade, including the v8.1 `h-decide`
  policy. The new one-scope profile-relation repair path remains fail-closed,
  predecessor-pinned, CAS-protected, effect-specific, and separate from
  specification authority. The release archive metadata, root instruction
  carriers, release notes, and the no-legacy-config archive smoke oracle were
  brought into the same public contract.
- The frozen qualification snapshot is
  `/private/tmp/haft-v9-final-snapshot3.zfMzrC/repo` at HEAD
  `84157cfc4339fd5129a400314cc70c989bf2ede4`, tracked-diff digest
  `sha256:fde267ca32ce22d1884a1de6a61e61bedcbac56eddfa5c2d4500a3add66af3f6`,
  and untracked-source digest
  `sha256:f3ef1608065a56a67cab605348bce2e75b45954bb62fc65b4855e9f43934f19f`.
  These three coordinates matched the working tree immediately before and
  after qualification. A prior run against a moving working tree ended at
  227/239 because another session changed `internal/onboarding/contract.go`
  during compilation; it is excluded from qualification rather than reported
  as either a product pass or a product defect.
- On the frozen snapshot, the full ordinary Go suite passed, including
  `internal/cli` in 804.862 seconds and `internal/fpfrefresh` in 322.783
  seconds. `go mod tidy -diff`, `go mod verify`, `go list ./...`,
  `go vet ./...`, pinned `golangci-lint v2.11.4` with zero findings,
  `git diff --check`, FPF integration verification, the real and synthetic
  token gates, Pi typecheck plus 34/34 tests, OpenSleigh format/compile plus
  509 passing tests with three explicit exclusions, actionlint v1.7.7, and
  shell syntax all passed.
- Two builds of each Go executable were byte-identical. The disposable
  source-probe `haft` digest is
  `sha256:420959e667fa12a14b046c50349dccfb9d26725e59cfb01c15ef7832ecec65bc`;
  it reports `9.0.0-source-probe`, commit `dirty-84157cf`, and is not a clean
  release candidate or installed-runtime evidence. The disposable archive
  `/private/tmp/haft-v9-final-probes.FkvVoZ/haft-darwin-arm64-source-probe.tar.gz`
  has digest
  `sha256:014c3d27ee0c05b263026e5ceda3b30f879d3852d9682ddf3bc8bc91be5fcc24`
  and passed the archive smoke. The seven-variant fresh-init matrix, supported
  singleton bootstrap, published-v8.1-to-v9 upgrade with restore and
  fail-before-migration collision checks, and exact schema-55-to-56 core
  migration all passed in disposable projects.
- The authoritative final race aggregate is
  `/private/tmp/haft-v9-final-probes.FkvVoZ/race-v9-final.json`, SHA-256
  `444775f001bc2e10cf1cc18d0cf570ac5e9fa3ff7b3f07d76366ccaeb53f32ae`.
  It ran from 2026-08-04T15:47:33Z through 17:21:53Z and passed all 12
  uniquely indexed shards, all 239 command observations, and all 1,913 work
  items with zero failed or interrupted commands. Its exact plan digest is
  `sha256:a940ba0851ab62db4391e658c27c951a6d95eaff39683d4d6044ef7d9df2d38d`;
  the aggregate plan payload byte-for-byte matches the saved pre-run plan.
- A fresh non-binding transition review was prepared after qualification at
  `.haft/project-typeenv-genesis-review.json`, digest
  `sha256:90a9402935c9b62df3ca9dbbf9560365683f1c07cbbe10608ac1102587676b52`,
  valid through 2026-08-05T17:29:21Z. Live readback after preparation proves
  that `ProjectTypeEnvHead` remains at revision 2 selecting composite
  `typeenv:sha256:6dc594a9d5470701b583a6e0893cf75d89629a27673d7aecd34b0993979c6aaf`
  and the typed-memory graph remains at revision 8. The reviewed successor is
  composite
  `typeenv:sha256:3ccfcdbc97f1a6f4a8241f03357ebcacf827868a35343cb19f88fd0ec07da615`
  on Base `36e74f…`, stage
  `project-typeenv-stage:sha256:76af2f6621b69490ded6f672654ab7baac94c9e512a05d26c11a43db135cf130`.
  Existing assertions revalidate cleanly and all six installed projection
  profiles are compatible. Of 253 rule comparisons, 241 are unchanged and 12
  kind-classification signatures remain honest `compiler_gap` results because
  the compiler does not implement their semantic order; there are zero
  additive, widened, narrowed, or removed results. This is the review's
  weakest link, not a hidden compatibility pass. Before preparation, a
  consistent disposable live-ledger backup was verified with `quick_check`
  and `integrity_check`; its path is
  `/private/tmp/haft-v9-typeenv-prepare.VZqTvQ/haft-before-prepare.db` and its
  digest is
  `sha256:f3f54c55683bf88ca64ad3ecfa20794c1ca677a4a4c209d0a1a8ec098a541184`.
- This ledger update is an evidence-only carrier edit made after the frozen
  race. It changes the final P13 source identity and therefore must be included
  in the later P13 freeze; it does not change the Go product bytes qualified by
  the race. Exact carrier/source checks are rerun after this edit, while no
  second full race is implied or claimed.

| Step | Status | Exact candidate/source | Evidence | Return/blocker |
|---|---|---|---|---|
| M0 baseline | passed | HEAD `84157cfc4339fd5129a400314cc70c989bf2ede4`; FPF `9a9a42e4d154021ca3f7415e0009a4214832f65f` | 2026-08-02 read-only baseline; live schema rows 1-56 and `integrity_check=ok`; private backup `/private/tmp/haft-v9-m0.ERBK58/haft.db` digest `sha256:4a758dc4285ccaf12606957c8c97a6c451fbf5c2fd1929e961b47bbf526bcff6`; live MCP status timed out; 27 pre-existing modified tracked paths recorded as user-owned | Installed-runtime timeout remains for M7/M8; no migration scheduled |
| M1 FPF review debt | passed | FPF `8b727cba9e893a467b82aab9da84fb7d6d945480`; DB `sha256:e10be4501c3f2f94754fae64a72642dcbbbb30433204617979fb89348df946e5`; generator `fpf-refresh-inputs/v1:sha256:6ec021d2cad0e4f5146680c4493e2d951b07a01d51d6c400d5ebe701695ca10e` | structurally coherent refresh passed with 8,092 units and zero diagnostics; all 39 semantic deltas reviewed; exact verifier and real/synthetic token gates passed | none |
| M1A startup-index single-flight | passed | current dirty source after M1; canonical project ID/root/exact ledger coordinates | deterministic 4-process barrier: peak active rebuilds `1`, parser/build entries `1`, publications `1`, followers `fresh_after_wait=3`, `SQLITE_BUSY=0`; exact multi-process test passed at `-count=20`; owner-kill retained the old epoch and one retry published old+1; mutation produced exactly two sequential epochs with no overlap; distinct projects reached peak `2`; four real stdio servers returned MCP status with one new epoch; full `internal/codebase` and `internal/codeintel` tests passed; coordinator/codebase race passed in 152.601s/20.685s and affected CLI race passed in 51.712s; unsafe carrier tests and build-tagged unsupported-platform fallback are present; doctor accepts several current-project servers; README/CHANGELOG updated | broader `internal/cli` default-timeout run is not M4-green: the stale current-source expectation was repaired, then the package reached its 10-minute timeout in unrelated TypeEnv-heavy tests; do not reuse that run as full qualification |
| M2 semantic/spec closure | source-complete; lifecycle gate open | FPF `8b727cba9e893a467b82aab9da84fb7d6d945480`; alias-only identity plus exact one-scope profile-relation repair | 2026-08-03 and 2026-08-04 source closeouts above; six section health states named explicitly | six exact SpecSection lifecycle acts |
| M3 acceptance refreeze | source-complete; exact review prepared; binding gate open | DB `sha256:e10be4501c3f2f94754fae64a72642dcbbbb30433204617979fb89348df946e5`; Base `typeenv:sha256:36e74f905065438532da7d486099c6a745dc82190f46c9f8958d13e3c44d2786`; Local-Practice 1.5.0 `sha256:1776b46c80229925c251623195680bf45f3c89c76f2791a4d215272a34ec2248`; candidate composite `typeenv:sha256:3ccfcdbc97f1a6f4a8241f03357ebcacf827868a35343cb19f88fd0ec07da615` | review carrier `sha256:90a9402935c9b62df3ca9dbbf9560365683f1c07cbbe10608ac1102587676b52` is selectable through 2026-08-05T17:29:21Z; current head remains revision 2 and graph 8; P13 remains `pending_selected_basis` | exact ProjectTypeEnvHead selection authority |
| M4 source qualification | passed for exact pre-ledger product snapshot | HEAD `84157cfc4339fd5129a400314cc70c989bf2ede4`; tracked `sha256:fde267ca32ce22d1884a1de6a61e61bedcbac56eddfa5c2d4500a3add66af3f6`; untracked `sha256:f3ef1608065a56a67cab605348bce2e75b45954bb62fc65b4855e9f43934f19f` | full ordinary/static/lint/source/language/archive/upgrade matrix passed; race aggregate `sha256:444775f001bc2e10cf1cc18d0cf570ac5e9fa3ff7b3f07d76366ccaeb53f32ae`: 12/12 shards, 239/239 commands, 1,913/1,913 work items | evidence-only ledger edit requires exact carrier checks and later inclusion in P13 freeze; P13 remains separate |
| M5 clean candidate + hosted CI/race | partial: exact local race passed | dirty source-probe `sha256:420959e667fa12a14b046c50349dccfb9d26725e59cfb01c15ef7832ecec65bc`; race plan `sha256:a940ba0851ab62db4391e658c27c951a6d95eaff39683d4d6044ef7d9df2d38d` | final local race and disposable archive/init/upgrade evidence above | clean commit/push and hosted-CI authority |
| M6 P13 | blocked before execution | manifest `pending_selected_basis` | no P13 was run or claimed | exact ProjectTypeEnvHead selection authority |
| M7 install + CLI capture | pending | — | — | install authority |
| M8 live P14 | pending | — | — | exact D5 process gate |
| M9 D7 + manual packet | pending | — | — | — |

### 2026-08-04 — automatic compatible-successor policy and live dogfood

This entry supersedes the manual ProjectTypeEnv-selection blocker in the
2026-08-04 M3/M6 rows above without rewriting that historical checkpoint. The
operator bound one project-wide v9 rule: every project, including Haft's own
dogfood, automatically activates every exact proven-compatible bundled
ProjectTypeEnv successor and never asks a human to approve that compatible
transition. Incompatible, incomplete, stale, or underdetermined transitions
remain fail-closed with no head write. This entry records implementation and
source-built dogfood evidence only; it is not installed-runtime P14, manual
E2E, commit/push, tag, publication, or release authority.

- Current activation carriers use
  `haft.project-typeenv.activation-delta.v3` and
  `haft.project-typeenv.activation-admission-basis.v3`. Their exact authority
  generation is `compatible_successor_policy` for the automatic branch or
  `host_routed_operator_request` for an explicit incompatible-selection or
  rollback branch. Legacy v1/v2 `manual_type_env_activation` carriers and graph
  events remain decode-and-replay history only and are never emitted by the v9
  writer.
- Schema 57 preserves old activation rows and graph-event bytes, admits the two
  disjoint current generations, and requires the activation delta,
  authority-use record, and typed-memory graph event to agree exactly. Focused
  activation, selection-effect, typed-memory-store, SQLite transition, schema
  migration, and v56-upgrade tests passed on the current source.
- The current FPF integration lock remains revision
  `8b727cba9e893a467b82aab9da84fb7d6d945480`, embedded DB
  `sha256:e10be4501c3f2f94754fae64a72642dcbbbb30433204617979fb89348df946e5`,
  Base TypeEnv
  `typeenv:sha256:36e74f905065438532da7d486099c6a745dc82190f46c9f8958d13e3c44d2786`,
  and generator identity
  `fpf-refresh-inputs/v1:sha256:2083fd4eb36eda3c3dfba7b5ecbf43d53078c8b48f47a38c676dd923cbbaeebe`.
  The refresh was an exact same-source/tool-input rebase: one
  `derivation_tool_changed` delta, zero semantic/source/DB/Base deltas, and zero
  diagnostics. This generator coordinate supersedes the older `6ec021…` and
  intermediate `45aad…` and `ceccfd…` coordinates above.
- Before live dogfood mutation, an online SQLite backup was written to
  `/private/tmp/haft-v9-auto-typeenv.EGlenG/haft-before-auto-activation.db`.
  It is 255,729,664 bytes, SHA-256
  `20fb5b3e4c3d7189ce3bb52c1f858c5e7dfb9041c065901adae697d69bfe2acf`,
  and passed both `quick_check` and `integrity_check`; it records schema 56,
  head revision 2, graph revision 8, and zero host-routed TypeEnv requests.
- A source-built dogfood binary at
  `/private/tmp/haft-v9-compatible-successor-candidate.Ctt3BY/haft`, SHA-256
  `ad8f94d37347f93028b66913a3e211869d1f632197d348783863bc100f39fb1b`,
  ran `haft init --core-only`. The product path migrated schema 56→57 and
  atomically advanced head 2→3 and graph 8→9 to composite
  `typeenv:sha256:3ccfcdbc97f1a6f4a8241f03357ebcacf827868a35343cb19f88fd0ec07da615`.
  The new authority use is `compatible_successor_policy` /
  `automatic_compatible_successor`; the new graph event records
  `authority_class=compatible_successor_policy`; host-request count remained
  zero; and the obsolete review carrier was removed.
- A second identical `haft init --core-only` reported schema 57 already
  current and changed no head, graph, request, authority-use, event, or latest
  ref. Post-run `quick_check` and `integrity_check` are `ok`. The 19 pre-existing
  `artifact_links` foreign-key diagnostics match the pre-mutation backup
  byte-for-byte as ordered rows, SHA-256
  `84dc10016d2f0fba1ff349198e9bc7276d1957b7ecf62d081c7e99b5368673aa`;
  the migration introduced none of them.

P13 freeze capture and independent verification passed for the automatically
activated head-3/graph-9 basis. The review carrier is
`.context/p13/p13-freeze-input-candidate-905f6a4cc931a26c.json`, carrier digest
`sha256:b55f546c4b085a822bff74a3d0d82635fa65ae6234d4a1204146c4b234068f07`;
its exact freeze coordinates are now in the `frozen_for_execution` manifest.
Current continuation is current-source qualification and the consolidated P13
run. Installed processes still execute their previously loaded binary, so this
dogfood result must not be represented as installed/live P14 evidence.

Final current-source qualification then exposed and closed seven additional
release-test tails: two stale non-software applicability expectations, one
release-carrier active-versus-superseded expectation, one missing public link
to the compatible-successor decision, one context-cancellation ownership lint
finding, and three dead helpers left by v9 refactors. The final exact ordinary
run passed the entire Go repository closure; notable package times were
`internal/cli=704.339s`, `db=169.071s`,
`internal/projecttypeenvselectioneffect/sqlite=201.064s`, and
`internal/fpfrefresh=246.490s`. `go mod tidy -diff`, `go mod verify`,
`go list ./...`, `git diff --check`, `go vet ./...`, pinned
`golangci-lint v2.11.4` with `0 issues`, actionlint v1.7.7, shell syntax, exact
FPF integration verification, real/synthetic token gates, Haft-Pi typecheck
plus 34/34 tests, and OpenSleigh format/compile plus 509 tests with three
declared exclusions passed. The final generator rebase after the Go cleanup
again contained exactly one `derivation_tool_changed` delta, zero semantic,
source, DB, or Base deltas, and zero diagnostics.

### 2026-08-05 — non-duplicating qualification topology

This addendum supersedes only current instructions that made a local full race,
P13, hosted CI, and release execute overlapping repository-wide test contours.
It does not invalidate historical evidence on older source identities, import
that evidence into the current candidate, pass P13/P14, or grant release
authority.

- The operator-authorized local full race was stopped after 1 hour 40 minutes.
  Its exact `cli.test` leaf, `go test -race`, compiled `race-shard`, and
  `go run ./cmd/race-shard` parents were terminated leaf-to-root. No aggregate
  was atomically published; the partial run is not evidence.
- `task test:race` now consumes the exact `go_race` cases in
  `internal/p13acceptance/manifest.json`: 37 named tests in 12 packages,
  `-p=1`, `-cpu=2`, and bounded `GOMAXPROCS`. The previous full-repository
  behavior remains available only as the explicit `task test:race-full`.
- CI owns one full non-desktop race matrix and its exact-SHA aggregate. A
  manual P13 dispatch skips CI's ordinary/coverage, full-race, and lint jobs so
  P13 does not run beside a second qualification matrix.
- Release runs no second full race, FPF token/source verification, or lint. Its
  guard queries the exact candidate SHA's successful push CI run and requires
  the matching unexpired `race-aggregate-<sha>` artifact before release-only
  build, archive smoke, sealing, or later publication can proceed.
- The commission drain regression remains valid runner-neutral lifecycle
  coverage and is renamed `TestExternalRunnerDrainStatus`; it is not a public
  `haft harness` compatibility test.

The current source return condition is focused topology verification followed
by one canonical consolidated P13. Hosted full-race evidence still requires a
future successful CI run for an exact committed SHA. Installation, host
restart, P14, manual E2E, push, tag, publication, and release authority remain
separate and absent.

## One-line continuation rule

Continue from the first non-passing M-step on the same exact source identity;
return to the smallest producer when identity differs; stop only at Gate S1,
an exact SpecSection lifecycle act, commit/push/install authority, or the
runtime-specific D5 host transition. Never call the result
`READY_FOR_MANUAL_E2E` before M0-M9 all pass.
