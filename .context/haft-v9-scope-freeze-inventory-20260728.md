# Haft v9 scope-freeze worktree inventory

Captured: 2026-07-28
Purpose: preserve mixed-authorship work while the frozen v9 P0 contract is
implemented. This is an inventory, not evidence, ownership attribution,
commit authority, or a release candidate.

## Git basis

- `HEAD` and `origin/dev`:
  `0f9c64effba0092646759c0f43b1cb8d29044367`
- `origin/main`:
  `5abf6fe7183b9076f47bde338656026c3d190aa1`
- tracked diff: 373 files, 34,461 insertions, 34,742 deletions;
- untracked physical files: 1,488;
- `data/FPF`: changed from `e264bfb1...` to `0990ff1d...`;
- `docs`: modified submodule worktree at `cb403430...`.

No file was deleted, reset, restored, stashed, cleaned, or attributed by this
inventory.

## Working classification

### Candidate v9 source

- typed-memory, TypeEnv, profile, authority, project-memory, P13/P14, init, FPF
  Query, code-index, and restart packages under `db/`, `internal/`, `cmd/`,
  `packages/`, and `scripts/`;
- current specifications, skills, host adapters, workflows, README, changelog,
  and migration guidance;
- the `data/FPF` revision change, subject to final source-identity validation.

Candidate source remains unstaged and uncommitted until every file is grouped
under one semantic commit proposal at the COMMIT GATE.

### Generated, cache, binary, or local evidence candidates

- `.pnpm-store/`;
- `codebase.test`;
- `.context-review-input/`;
- generated/runtime evidence under ignored `.context` lanes;
- local host projections such as `.grok/` unless their owning init contract
  explicitly classifies them as release source.

These paths must not enter a release commit by accident. They are not removed
by this plan.

### Unresolved ownership or intent

- the modified `docs` submodule worktree;
- any pre-existing file outside the frozen v9 packages;
- any source file whose only evidence of ownership is its presence in the
  dirty tree.

Unresolved items are preserved. If one overlaps a required P0 edit, the
implementer must inspect the exact diff and extend it minimally rather than
replacing the file wholesale.

## Largest untracked source groups

- `internal/projectmemory`: 200 files;
- `internal/cli`: 127 files;
- `internal/fpf`: 98 files;
- `internal/typedmemorystore`: 80 files;
- `internal/typedmemory`: 77 files;
- `internal/projectprofile`: 74 files;
- `internal/codebase`: 50 files;
- `internal/projecttypeenvselectioneffect`: 49 files;
- `internal/specmigrationv2`: 47 files;
- `internal/initplanning`: 45 files;
- `db`: 42 files;
- `internal/authority`: 40 files.

Counts are orientation only. File-by-file commit ownership is established from
the final diff and focused acceptance, not inferred from these directory
totals.

## Invalidation and return rules

- any tracked source change after immutable freeze invalidates P13/P14 evidence;
- any unclassified source file blocks the commit map, not unrelated
  implementation work;
- no generated carrier proves that its producing source is current;
- no old process or binary may contribute to installed P14 evidence;
- COMMIT, push/integration, SpecSection lifecycle, host restart, tag, and
  release remain separate operator gates.

## Historical source-closeout addendum — 2026-07-29, before the FPF 2ada refresh

This addendum preserves the historical 2026-07-28 baseline above and records
the post-C3/C4 source-closeout tree. It remains an inventory and proposed
partition, not ownership proof, staging authority, an atomic commit map, P13,
or installed-runtime evidence.

### Current Git basis and exact counts

- branch: `dev`;
- `HEAD` and `origin/dev`:
  `0f9c64effba0092646759c0f43b1cb8d29044367`;
- `origin/main`:
  `5abf6fe7183b9076f47bde338656026c3d190aa1`;
- staged paths: `0`;
- tracked dirty paths: `395` = `302 M` + `93 D`;
- normal collapsed untracked status entries: `408`;
- Git-visible untracked path records:
  `1,558` = `1,557` regular files plus the nested-repository directory
  `tui/.context/repos/roughdraft/`;
- total Git-visible dirty path records: `1,953`;
- ignored path records are outside this inventory;
- tracked diff: `395 files changed, 39,302 insertions(+), 38,837
  deletions(-)`, including one binary path, `internal/cli/fpf.db`.

`data/FPF` is a clean submodule at
`0990ff1d1ccee4587b8f7e16e7a725a8edbe66b4`, changed from the superproject's
recorded `e264bfb1...`. `docs` remains at recorded/current HEAD
`cb4034307bd2bb3c909afa53f4065a41928b36f1` with seven dirty Astro worktree
files and is excluded from every candidate commit.

### Source verification on the final bytes

The isolated source candidate is
`/private/tmp/haft-v9-source.w7YxTN/haft`,
`sha256:ad3161af7228e02a494e3ec0cea03e9b0ed0bf00073f204c7fcc66875f01bb77`.
It was not installed.

Current-green source evidence:

- `go test ./internal/cli -count=1 -timeout=30m`;
- `go test ./... -count=1 -timeout=30m`;
- `go vet ./...`;
- pinned `golangci-lint v2.11.4` with no issue-count truncation:
  `0 issues`;
- `packages/haft-pi`: TypeScript typecheck and `33/33` tests;
- actionlint v1.7.7 over all four `.github/workflows/*.yml` carriers;
- `bash -n install.sh scripts/*.sh scripts/release/*.sh`;
- `git diff --check`.

Direct-source C4 evidence is
`/private/tmp/haft-v9-source.w7YxTN/c4/run-nanh_hfa/results.json`,
`sha256:e3c9f31897e4daf31eb099c89951248289858bbf4c6f5d3bbd52ded26b7ed670`.
It is green for the actual compacted twelve-tool MCP catalog and closed
schemas, typed no-write recovery, Codex/Claude TTY and flag equivalence,
idempotency, exact legacy takeover, collision-before-first-write, single
error/no Usage, preservation outside managed markers, and non-mutating
predecessor-checkpoint diagnostics. This is source-probe evidence only, not
D3/P14 or installed-runtime evidence.

### Exhaustive primary path partition

The following precedence makes one exhaustive primary assignment for all
1,953 Git-visible dirty path records:

`excluded > unresolved > cleanup > restart > acceptance > graph > entity >
release > typed-memory > MCP/init`.

| Proposed semantic group | Primary paths | Status split | Sorted path-manifest SHA-256 |
|---|---:|---:|---|
| 1. release harness, v8.1 lineage, carriers, notes, workflows | 17 | 6 M / 11 U | `6ecad2bab6b0fe08d910fe1a28219f4b872120f3c9d8067162556ddef6659742` |
| 2. predecessor checkpoint and restart compatibility | 33 | 33 U | `7220440433279988a6dbc2bf138c345957ecb1709341ff9c92decb1ad6c946ea` |
| 3. typed-memory core, storage, schema, migrations, TypeEnv, profile, authority | 1,061 | 25 M / 1,036 U | `deb7f4224e4f3090fcf2078b5878cc7bed6f689774e1455c35402353b5c26fb0` |
| 4. task-level entity establishment and onboarding | 87 | 87 U | `985d42ee127f42ae10a244547dec1f3a6887da5c79455817ee0917aceefe73e3` |
| 5. MCP schemas, canonical skills, host adapters, init, public carriers | 404 | 165 M / 239 U | `ee7158b56dac770d6e78f5ef39a296d4a7b91bb0c4c362a9e0c38dd1664a9c92` |
| 6. canonical project paths and code-graph governance lanes | 151 | 75 M / 76 U | `29e68d1bb162e0d996890af3913cb4a322237f71ced1838e4c206964c8493511` |
| 7. P13/P14 acceptance harness | 43 | 1 M / 42 U | `794877feed6ecd23d0078c2d8bb56d890129f655497265c21eae0020884cbea8` |
| 8. contract-preserving lint and obsolete-code cleanup | 100 | 92 D / 7 M / 1 U | `409d4594ab3396f93e22fde2e5b54e546b436598f7bdb1544e8a11841f3e4289` |
| excluded generated/local/user work | 33 | 1 M / 32 U | `28a153a7240afb7441cd9248abbcd42f9517fb3725c6986374de3cb6d21006c9` |
| unresolved ownership or lifecycle | 24 | 1 D / 22 M / 1 U | `3d21313590b582f1710cc184cda030b6dbc6ae82ef4724dbb62516b7b096bb70` |

The partition has no unmatched path, and its status totals reconcile exactly
to `302 M + 93 D + 1,558 U = 1,953`. The manifest digests identify the
audited sorted path sets; they do not make the sets atomic or authorize
staging.

Primary rule coverage:

1. exact release workflows, notes, public carriers, release scripts,
   installer, stream-truth tests, and v8.1 upgrade coverage;
2. checkpoint/supervisor binaries, `internal/agenthostrestart`, and the exact
   CLI status compatibility files;
3. `db` and typed-memory, TypeEnv, profile, authority, storage, and artifact
   packages except the graph-owned affected-path lane;
4. profile onboarding and task-level entity/read/recovery projections;
5. remaining CLI, FPF server/schema, init, project, tools, Pi, public skill and
   host carriers, and the FPF source/token gate;
6. codebase, codeintel, contextgraph, graph, graphrank, Overseer, present,
   projectpath, and exact CLI graph lanes;
7. `internal/p13acceptance`, `internal/p14acceptance`, and CI acceptance
   orchestration;
8. `.golangci`, `internal/agent`, and deleted obsolete legacy CLI, FPF,
   present, project, tools, and Pi paths.

### Overlap and atomicity warning

The raw semantic rules intersect on at least 233 paths before precedence:

- 79 typed-memory + MCP/init paths in the FPF local-practice, contract,
  project-TypeEnv, TypeEnv, and TypeEnv-SQL packages;
- 75 MCP/init + cleanup paths, primarily deleted legacy CLI, FPF, project,
  tools, and Pi files;
- 44 typed-memory + entity paths in task-level project-memory entity, read,
  neighborhood, and recall lanes;
- 17 entity + MCP/init CLI adapters and FPF entity/memory/onboard schemas;
- 9 MCP/init + graph paths;
- 5 graph + cleanup paths;
- 3 restart + MCP/init CLI status paths;
- 1 release + acceptance path, `.github/workflows/ci.yml`.

Cross-theme carriers such as `README.md`, `AGENTS.md`, `CLAUDE.md`,
`CHANGELOG.md`, `Taskfile.yaml`, and stream-truth tests may also need hunk-level
assignment. Therefore the table is an exhaustive review partition, but it is
not yet an exact atomic commit map. Before any staging request, the 233
overlaps and cross-theme carrier hunks must be resolved into a materialized
exact-path/hunk appendix, or the proposed commits must be deliberately
coarsened without breaking dependency order.

The exact diagnostic path sets are:

- primary partition:
  `/private/tmp/haft-v9-c5-inventory-partition.tsv`,
  `sha256:eaec387393218bc75186457fffa67e647d3783e889e00815a742bc069f8762a5`;
- all raw overlap memberships:
  `/private/tmp/haft-v9-c5-overlaps.tsv`,
  `sha256:a9619fcc52974faefe31ba3888416a892ac8c0bda1bc48d627a1d3a9d7c9819a`.

Both are mode `0600` temporary diagnostics. The first has exactly 1,953
unique path rows; the second has 466 membership rows for the 233 overlapping
paths.

### Refined COMMIT GATE disposition

The overlap graph has exactly two connected components:

1. release plus P13/P14 acceptance — 60 primary paths, with
   `.github/workflows/ci.yml` as the only cross-theme path;
2. restart, typed memory, entity/onboarding, MCP/skills/init, code graph, and
   cleanup — 1,836 primary paths with 232 cross-theme paths.

A diagnostic coarsening adds `go.mod` and `go.sum` to the second component,
excludes the 18 OpenSleigh paths, and holds the four spec carriers at their
lifecycle gate:

| Disposition | Paths | Status split |
|---|---:|---:|
| A — release + P13/P14 integration component | 60 | 7 M / 53 U |
| B — coarsened product-source component, including module files | 1,838 | 274 M / 92 D / 1,472 U |
| excluded generated/local/OpenSleigh user work | 51 | 19 M / 32 U |
| lifecycle-blocked spec carriers | 4 | 2 M / 1 D / 1 U |

These reconcile exactly to 1,953 paths. The materialized exact path list is
`/private/tmp/haft-v9-c5-integration-components.tsv`,
`sha256:e9be65d0ff6d08413ac94f5081b6aa6b7e8000cb2a0fd7462197cc2f3209fc0e`.

This is a complete path disposition, but A and B are integration components,
not proven semantic-atomic commits. Component B deliberately collapses six
connected themes. `go mod tidy -diff` produced no diff, `go mod verify`
reported `all modules verified`, and `go list -deps -test ./...` succeeded;
the pair is therefore the reproducible dependency closure for the selected
source. The exact provenance and necessity of every version upgrade,
especially `golang.org/x/net v0.47.0 -> v0.56.0` and
`modernc.org/sqlite v1.41.0 -> v1.54.0`, is not independently established.
The B assignment remains conditional on that review.

Before either component can be proposed for staging:

- resolve the module-version provenance or regenerate the pair from an
  explicitly selected source set;
- resolve the four spec carriers through the separate semantic and lifecycle
  gates;
- reconstruct and verify B-on-HEAD, then A-on-B, in disposable trees; or
- perform the finer 233-path hunk assignment and verify every resulting
  intermediate tree.

### Exact exclusions and unresolved set

Excluded from every proposed commit:

- `.context-review-input/**` — 14 paths;
- `.grok/**` — 13 paths;
- `.pnpm-store/**` — 3 paths;
- `codebase.test`;
- dirty `docs` submodule worktree;
- nested repository `tui/.context/repos/roughdraft/`;
- all 18 dirty OpenSleigh user-work paths:
  - `open-sleigh/SPEC.md`;
  - `open-sleigh/lib/mix/tasks/open_sleigh.start.ex`;
  - `open-sleigh/lib/open_sleigh/gates/semantic/lade_quadrants_split_ok.ex`;
  - `open-sleigh/lib/open_sleigh/orchestrator.ex`;
  - `open-sleigh/lib/open_sleigh/phase.ex`;
  - `open-sleigh/lib/open_sleigh/workspace_manager.ex`;
  - `open-sleigh/specs/agents/prompts/5.4-pro-confirmation.md`;
  - `open-sleigh/specs/agents/prompts/5.4-pro-review.md`;
  - `open-sleigh/specs/target-system/GATES.md`;
  - `open-sleigh/specs/target-system/HAFT_CONTRACT.md`;
  - `open-sleigh/specs/target-system/ILLEGAL_STATES.md`;
  - `open-sleigh/specs/target-system/OPEN_QUESTIONS.md`;
  - `open-sleigh/specs/target-system/PHASE_ONTOLOGY.md`;
  - `open-sleigh/specs/target-system/SYSTEM_CONTEXT.md`;
  - `open-sleigh/specs/target-system/TERM_MAP.md`;
  - `open-sleigh/test/open_sleigh/orchestrator_commission_lifecycle_test.exs`;
  - `open-sleigh/test/open_sleigh/orchestrator_retry_test.exs`;
  - `open-sleigh/test/open_sleigh/workflow_test.exs`.

Still unresolved pending exact lifecycle or dependency-provenance review:

- `.haft/specs/enabling-system.md`;
- `.haft/specs/software-system.md`;
- `.haft/specs/target-system.md`;
- `.haft/specs/term-map.md`;
- `go.mod`;
- `go.sum`.

The module pair appears under diagnostic component B only as the explicit
conditional disposition described above. No unresolved path is silently
assigned from its dirty-tree presence alone.

### Open closeout gates

Seven of the eight P0 clauses are current-green. The remaining compound clause
stays open because active typed-memory binding content still includes
merge/split while the frozen v2 SoftwareSystemSpec and runtime schema allow
only context-bound `admit_alias` and `supersede_alias`. The implementation
supports keeping the frozen v2 contract and explicitly narrowing or
superseding the older binding claim, but that recommendation is not authority.

Consequently:

- P0 is not fully closed;
- MethodRun
  `mpull-20260728-implement-the-operator-approved-haft-v9-scope-fr-17122bd7`
  is closed with both hard gates satisfied, no waivers, and verification
  result `partial`;
- D2 through D7 remain open;
- no P13 or P14 claim is current for these source bytes;
- the temporary C4 binary is not an installed candidate;
- the loaded MCP process remains stale and was not restarted;
- SpecSection lifecycle, commit, push/integration, tag, publication, and
  release remain separate human gates.

### Operational disclosures

During an exploratory C4 init invocation, shell variables failed to set the
intended tool working directory and initialized `/tmp/.haft`, `/tmp/.codex`,
and `/tmp/AGENTS.md`. The three paths were identified by creation time and
moved intact to the recoverable quarantine
`/tmp/haft-v9-source.w7YxTN/accidental-tmp-init-20260728T232938+0400/`.
They are absent from `/tmp`, and the repository was unaffected.

A read-only diagnostic also issued
`PRAGMA wal_checkpoint(PASSIVE)` against
`/Users/ivanzakutnii/.haft/projects/qnt_e3149c17/haft.db`; SQLite reported
`0|16809|16809`. It may have checkpointed existing WAL pages into the database
file, but no logical project-memory mutation was requested or intentionally
performed.

The stale loaded MCP timed out after 60 seconds on both final
`haft_query(status, full=true)` and `haft_method(show)` attempts. A direct
source-binary `haft_method(show)` returned the open MethodRun, while a
source-binary full-status diagnostic was stopped after more than 90 seconds
without a result. None of those status attempts is used as installed-runtime
evidence.

No repository path was staged, committed, reset, restored, stashed, cleaned,
or deleted during this addendum.

## Current source-integration addendum — 2026-07-29

This addendum supersedes the word “Current” in the historical addendum above
for closeout orientation. It describes the FPF integration worktree over parent
HEAD `432eb8572974b022a63e98972c2222d3d364c6b6`; it is not a post-commit tree
identity.

- `data/FPF`: `0990ff1` -> `2ada413629b846ef308222d16489a82cb5b40a71`;
- committed-index candidate: 7,968 source units, database SHA-256
  `2df40bb9d4e754ee8e9d47ced1eb4926fbb941ac42880dc0a365c60fa4b04bc4`;
- current source compiler: `fpf-base-typeenv.cov2.v5`;
- current Base TypeEnv:
  `typeenv:sha256:effff65cae9eaf1aba287245df79c460fbeaee5f666dcaa7992bfeb251c1e35e`;
- historical `0990ff1` V4 Base retained for exact replay;
- Local-Practice `1.4.0` prepared as a non-binding candidate, with no head
  selection;
- the embedded token gate was rebaselined and passed against the refreshed
  source bundle;
- an exact `task fpf-index` rerun retained the V4-to-V5 compatibility
  assessment and the byte-identical database SHA-256 above;
- the embedded “target system” query again reaches the source-owned
  `SYSTEM-IN-CONTEXT` card and its current C.26, C.32.PAD, and E.18.NET
  witnesses;
- full `cmd/indexer`, `internal/fpf`, `internal/fpf/typeenvsql`, and
  `internal/cli` package checks passed on the refreshed source worktree.

No earlier P13/P14 evidence is current for these bytes. The
RoleAssignment/work attribution, C.2.1 ClaimGraph invariant, and existing
merge/split semantic questions remain outside this evidence-maintenance
addendum. Commit, SpecSection lifecycle, install/restart, P14, tag, publication,
and release authority remain separate.

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
