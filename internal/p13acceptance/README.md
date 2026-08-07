# P13 consolidated acceptance harness

Status: the exact current FPF `8b727cb` / COV2 v5 source identity is pinned and
source-built dogfood `haft init --core-only` automatically activated its
proven-compatible successor under `compatible_successor_policy`. The project
ledger now records head revision 3 and graph revision 9 with zero host-routed
TypeEnv requests. Freeze capture and verification passed, and the manifest is
`frozen_for_execution` on those exact coordinates. This makes P13 executable;
it does not itself claim that the consolidated P13 run passed.

The active execution carrier is
`.context/haft-v9-deterministic-closeout.plan.md`, specifically
`### D2 — freeze and run one P13`. The detailed `G0`–`G8` mappings are the
manifest-local executable gate contract under that one D2 operation; the
historical typed-memory master plan is not active P13 authority.

The 2026-07-23 structural audit repinned G1/G2/G4/G6 to the current C.3
compiler/core/runtime, the full MCP admission handler, and sealed historical
classification replay. The 2026-07-24 audit added the previously missing G0I
core-init/host-publication gate and its current executable anchors while
keeping the unfinished public CLI policy explicit. The README currentness
check also requires the exact schema/writer identity from the manifest. The
structural check logs the digest of the exact current manifest bytes. A digest
from an earlier source candidate is neither a current freeze receipt nor
acceptance evidence.

The manifest maps every P13 gate (`G0`, `G0P`, `G0I`, `G1`-`G8`, and `G7R`)
to named Go test anchors and exact suite commands. The runner captures the
source tree, exact accepted profile, selected project TypeEnv receipt, typed
graph head, and database schema before the first suite and after the last. It
does not repeat that full identity capture around every suite: a changed final
identity fails the whole carrier. Each suite instead records an exact v1
dependency digest over its command, owning gate/anchor contract, relevant
source bytes, runtime input set, semantic basis, environment, tool family, and
installed dependency tree where applicable. The
v3 profile-declaration seam remains covered by `G0P`; P13 preserves the exact
generation of the currently admitted profile rather than relabeling it.

The normal Go suite covers the full non-desktop package closure. The race suite
has a different, explicit evidence profile: each package and exact test-name
set is stored in `manifest.json`, cases run one at a time with `-p=1`, and
each test binary runs with `-cpu=2`. An anchor is credited to the race suite
only when its package and test name match one exact case. This locally
exercises the stateful
concurrency boundaries around project-ledger and SQLite transactions, TypeEnv
selection, typed-memory writes and reads, graph epochs, cache invalidation, and
init publication without making full-repository race the local default.

The broader race closure runs once in CI: `.github/workflows/ci.yml` assigns
every non-desktop package to one validated race shard and publishes the exact
SHA's aggregate carrier. Release does not rerun those tests. Its guard requires
one successful push CI run for the candidate SHA and an unexpired aggregate
named for that same SHA before packaging can begin. Workflow definitions are
not passing evidence by themselves, and P13 does not import the CI carrier.
Manual P13 dispatch also skips the normal CI/coverage/full-race matrix so the
two qualification contours cannot execute redundantly in one workflow run.

The frozen source identity includes the v9 release-blocker report at
`.context/current-plan-issue-report.md`, the deterministic closeout carrier,
every Go build input reported by `go list`, the installed Haft-Pi dependency
tree, and the resolved executable/symlink bytes for the Go, Node, shell,
Python, C, and C++ toolchain closure used by the
suites. The explicit Query token suite runs
`scripts/fpf_query_token_gate.sh`, whose CPython 3.10-3.13 environment is
installed from the binary-only hash lock before the exact embedded and
synthetic o200k gates run. Cold execution still needs PyPI and the o200k BPE
asset network fetch; hashes fail closed, but this is not an offline-hermetic
wheelhouse or vendored-asset proof.

The manifest is a three-state fail-closed carrier:

1. `pending_final_source` carries no target FPF identity and no selection
   coordinates. This is the fail-closed state before one exact current-source
   candidate and its semantic delta have been accepted.
2. `pending_automatic_activation` carries the exact final FPF revision, source
   digests, Base TypeEnv identity, and compiler schema, while
   `freeze_input.posture` remains `pending_automatic_activation`. Enter this
   state only after P12F has accepted one internally consistent aligned
   candidate. The next mutation-capable reconciliation activates it
   automatically if the transaction-current compatibility predicate holds.
3. `frozen_for_execution` requires `freeze_input.posture=selected_and_frozen`,
   exact post-P12E head/selection-receipt/graph coordinates, and a target Base
   equal to the selected final FPF identity.

The consolidated command fails before package discovery or suite execution in
the first two states. Candidate Stage coordinates are not activation authority
and must not be copied into the frozen state. The preceding P12E Transition
advanced the exact revision-1 project head from FPF `44dd881`
(`C=d6097b...`, `B=aa1eec...`) to the now-predecessor `0990ff1` candidate at
head revision 2. The current automatic activation advanced from that exact
predecessor rather than relabeling it. The activated target Stage remains schema v5
and byte-match the current canonical profile basis, compatible ProfileFit, and
installed transition-profile closure when final freeze input is captured. The
same preflight requires the exact selected FPF checkout plus embedded index
metadata, schema 58 with its exact writer-54 marker (independently verified), and an
explicitly empty excluded-Go-package set. Any
changed coordinate, missing anchor, skipped anchor, non-empty waiver set, or
failed command blocks the run.

After any release-relevant source-byte change, recapture the already-activated
profile/head/receipt/graph basis as a review candidate with:

```bash
HAFT_P13_CAPTURE_FREEZE_INPUT=1 \
  go test -count=1 -v ./internal/p13acceptance \
  -run '^TestP13CaptureFreezeInputCandidate$'
```

Compatible-successor activation is already complete before capture, but a
capture made before the current manifest and source bytes is stale because the
carrier binds both the manifest and full acceptance identity. This read-only capture uses the same identity
loader and closure checks as the consolidated runner. It atomically publishes a no-clobber
`haft.p13.freeze-input-candidate/v1` carrier under `.context/p13/`, but does
not edit `manifest.json`. Its posture is
`review_candidate_not_selection_or_evidence`: the carrier records an already
activated basis for exact review; it cannot activate a TypeEnv head, authorize
Work, pass P13, or establish evidence. Verify its manifest and identity
digests against the current selected basis with:

```bash
HAFT_P13_VERIFY_FREEZE_INPUT=.context/p13/<CAPTURED_CARRIER>.json \
  go test -count=1 -v ./internal/p13acceptance \
  -run '^TestP13VerifyFreezeInputCandidate$'
```

The verifier is read-only, rejects non-canonical or extended JSON, and fails
when the manifest bytes or selected profile/head/receipt/graph basis changed.
After it passes, copy only the carrier's `freeze_input` object into the
still-pending manifest as a separately reviewed mechanical edit. A
pre-selection Stage is not accepted by either path.

After automatic activation exists, those exact accepted coordinates have been
frozen, and the manifest status is `frozen_for_execution`, run exactly:

```bash
HAFT_P13_RUN_CONSOLIDATED=1 \
  go test -count=1 -timeout=12h -v ./internal/p13acceptance \
  -run '^TestP13ConsolidatedAcceptance$'
```

The command publishes one canonical JSON evidence record with atomic
no-clobber semantics under the ignored `.context/p13/` directory, rereads the
published bytes, and logs the carrier path and digest. The record contains the
consolidated identity digest, exact invocations, execution window, freshness
boundary, per-suite dependency/provenance, and output digests. The first frozen
v9 run executes every suite because the older focused green results were not
published as canonical reusable evidence carriers; prose and terminal history
are not imported as proof. A later run may reuse only suites from one prior
passing v3 carrier whose exact dependency digest still matches:

```bash
HAFT_P13_REUSE_ACCEPTANCE_EVIDENCE=.context/p13/<PRIOR_P13_EVIDENCE>.json \
HAFT_P13_REUSE_ACCEPTANCE_DIGEST=sha256:<PRIOR_CARRIER_DIGEST> \
HAFT_P13_RUN_CONSOLIDATED=1 \
  go test -count=1 -timeout=12h -v ./internal/p13acceptance \
  -run '^TestP13ConsolidatedAcceptance$'
```

Both reuse variables are required together. The prior carrier must be
canonical, byte-match its supplied digest, and record one unchanged identity.
It may be a passing carrier or a failed carrier with individually passing suite
results. Only a suite whose own status is `pass`, exact dependency digest still
matches, and current required Go anchors are present may be imported. Failed,
changed, missing, or newly required suites execute normally. A new passing
result is P13 evidence on the new consolidated identity. It is not installed
P14 evidence, a clean-candidate P16 result, a release claim, or release
authority.

Before a later P14 carrier relies on that record, recheck that its exact bytes,
manifest, full acceptance identity, frozen selection, suites, gates, and
anchors are still current:

```bash
HAFT_P13_VERIFY_ACCEPTANCE_EVIDENCE=.context/p13/<P13_EVIDENCE>.json \
HAFT_P13_VERIFY_ACCEPTANCE_DIGEST=sha256:<P13_CARRIER_DIGEST> \
  go test -count=1 -v ./internal/p13acceptance \
  -run '^TestP13VerifyAcceptanceEvidenceFresh$'
```

This is read-only freshness verification. It does not rerun P13, extend the
record's freshness window, or turn the record into P14 installed evidence.

During active development, only the structural check is intended to run:

```bash
go test -count=1 ./internal/p13acceptance -run '^TestP13ManifestStructureAndAnchors$'
```

The structural check executes when the complete ignored private basis is
present, skips when that basis is wholly absent in a clean public checkout,
and fails when only part of the basis is present. Absence is not structural
evidence; the remote preflight must restore the exact frozen inputs above
before invoking the same check.

Ordinary `go test ./...`, package race, and coverage commands do not execute
the consolidated runner. The runner requires the exact
`HAFT_P13_RUN_CONSOLIDATED=1` capability above and strips that capability from
every child-suite environment before setting `HAFT_P13_CHILD=1`. The separate
manual CI job runs the same consolidated acceptance on an explicitly
provisioned frozen-basis bundle with a 60-minute job cap. A passing job
preserves its non-publishing evidence artifact; it does not tag or release.

### Manual remote consolidated P13

`.github/workflows/ci.yml` exposes the manual
`run_p13` input and gives that named consolidated-acceptance job a 60-minute
operational cap. It requires `p13_basis_run_id` to identify an existing
same-repository workflow run containing an artifact named `p13-frozen-basis`.
The downloaded artifact must have this shape:

```text
basis.json
repository/.agents/skills/<current generated skill tree>
repository/.context/current-plan-issue-report.md
repository/.context/haft-v9-deterministic-closeout.plan.md
repository/.context/p13/<freeze-input-candidate>.json
repository/.haft/config.yaml
repository/.haft/project-profile.yaml
repository/.haft/project.yaml
home/.haft/projects/<project-id>/haft.db
```

`basis.json` is a small boundary manifest:

```json
{
  "schema": "haft.p13.remote-frozen-basis/v1",
  "candidate_sha": "<full workflow commit SHA>",
  "freeze_candidate_path": ".context/p13/<freeze-input-candidate>.json"
}
```

The ignored `.agents`, `.context`, and project-basis `.haft` inputs are part of
the frozen source identity; a clean checkout cannot synthesize them. The job
fails closed when the run ID, candidate SHA, generated skill tree, required
carriers, project-basis files, candidate path, or project database is absent.
It verifies the freeze input and manifest, then runs
`TestP13ConsolidatedAcceptance` once with `GOMAXPROCS=1` and `GOFLAGS=-p=1`.
Exactly one new passing evidence carrier must appear; the same job immediately
runs `TestP13VerifyAcceptanceEvidenceFresh` and uploads only that carrier as a
14-day workflow artifact. The upload is evidence transport, not a GitHub
Release or a public release claim. No producer for the sensitive frozen-basis
input artifact is enabled by default; until one is explicitly provisioned, the
manual P13 lane is unavailable rather than falsely green.

Before the consolidated run, the same manifest-owned bounded race profile may
be exercised directly without publishing a P13 evidence carrier:

```bash
task test:race
```

`scripts/test-critical-race.sh` reads the exact package/test sets from
`manifest.json`; it does not maintain a second list. A passing focused command
locates race failures on the current bytes. It is not a substitute for the
consolidated identity-bound evidence record. The deliberately expensive
`task test:race-full` is the explicit local adapter for reproducing CI's full
closure, not a prerequisite before P13.
