# P14 installed request/oracle contract

Status: prepared, not executed. Nothing in this package is installed-runtime
evidence, performed Work, a release claim, or restart authority.

`contract.json` (`haft.p14.request-oracle-contract/v3`) closes the P14 scenario
set before the live boundary. Each
scenario names its installed surface, deterministic request builder, oracle
kind, permitted fixture effect, required frozen binding groups, and the local
test that supplies the pre-install semantic oracle. The contract deliberately
contains no live PID, receipt, response, or passing verdict.

Run the structural gate while P13 is still pending:

```bash
go test -count=1 ./internal/p14acceptance \
  -run '^TestP14RequestOracleContract$'
```

After P13 passes, the prepared-carrier seal binds exact request bytes and
normalized local-oracle digests to the P13 evidence and candidate basis. Live
execution then creates a separate observation carrier; it never edits the
prepared carrier or retrofits an expected result after installation.

The exact input is a generated `haft.p14.prepared-request-oracle-input/v1`
value, not a hand-filled placeholder document. It contains one canonical
payload per declared surface, one shared semantic-request digest per scenario,
and either a normalized expected-result digest or a closed live-predicate set.
Its binding table is also closed: candidate, P13, and selected-project bases
are embedded; golden-memory, init-matrix, and identifier fixtures must be
digest-bound carriers under `.context/`; restart-checkpoint and live-MCP
identity remain explicit execution-time requirements. The seal rereads every
fixture carrier and rejects missing or changed bytes.

The `fpf_query_projection` family has its first executable generator. One
closed 22-case semantic matrix derives both installed-CLI argv and live-MCP
`haft_query` arguments. It covers concern/lookup/inspect default, working,
trace, and diagnostic views; exact `A.6.REL` and `A.15.2` hydration; trace and
diagnostic replay; request and source-snapshot mismatch; and fail-closed
`working + trace_ref`. The local oracle executes the candidate, validates the
working denylist, full bodies, reconstructable trace, diagnostic-only retrieval
internals, replay equality, and typed mismatches, then records only normalized
digests in the prepared scenario.

Exercise that generator against candidate bytes without publishing a carrier:

```bash
HAFT_P14_FPF_QUERY_CANDIDATE=/absolute/path/to/haft \
HAFT_P14_FPF_QUERY_EXPECTED_REVISION=<FROZEN_FPF_REVISION> \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14CaptureFPFQueryProjectionAgainstCandidate$'
```

This candidate exercise is still pre-install oracle preparation. It is not a
live MCP observation or P14 evidence.

Four code-graph families are executable rather than deferred:
`code_graph_exact_explore`, `code_graph_concern_explore`,
`code_graph_ambiguous_concern`, and `code_graph_coverage_diagnostic`. Their
shared builder derives exact `haft graph explore` argv and
`haft_query(action="explore", ...)` arguments, executes the frozen candidate
for the local oracle, and strips trace/index-epoch noise before digest
comparison. The exact `NeighborhoodRead` case requires a resolved seed and a
first source hop whose symbol and file are exactly
`NeighborhoodRead` / `internal/cli/memory_read_runtime.go`. That hop's opaque
anchor must join one reasoning item containing the active typed-memory decision
`dec-20260716-11f33e36` only in `module_decision_refs`; the superseded
`dec-20260713-9ed66ef0` is rejected. Concern cases require advisory ranking
with `identity_auto_selected=false`; the diagnostic case requires both
resolution and retrieval diagnostics.

Two additional live-predicate scenarios bind those capabilities to the
verified resumed Codex process.
`agent_code_graph_orientation` starts from a sealed natural-language user
prompt and accepts only the first and sole tool call in that turn as the exact
read-only Explore probe. `agent_typed_memory_orientation` keeps its first three
calls strictly read-only: scripted resolve, prompt-driven resolve, and
scripted resolve must all return `known_absent` on one byte-identical TypeEnv
and graph revision. A second sealed user prompt then explicitly requests
persistence and must produce one task-level `haft_entity(action="establish")`
call. The scenario replays the exact unchanged request and idempotency key,
resolves the entity exactly, and passes the returned canonical
`{"ref_kind_id":"U.EntityRef","reference_id":"..."}` verbatim through
neighborhood and recall. The normalizer independently requires one graph
revision advance, `freshly_committed` followed by `replayed`, byte-composable
`next_read`, identity equality at every input/output boundary, and
non-authorizing interpretation throughout. Establishment arguments expose
only task-level identity, aliases, persistence reason, request provenance, and
idempotency; TypeEnv, basis, ref-kind, authority, and raw change-set
coordinates fail closed. Any extra prompt-turn tool call, implicit admission,
pre-save graph change, replay drift, or identity drift fails closed. Their
installed-CLI surface is satisfied only by the corresponding separately
executed and oracle-matched read scenario; their live-MCP surface must come
from the exact prompts, task transcript, and response capture.

The `identifier_namespace` family also has an executable semantic builder. Its
frozen fixture names one canonical `.haft` artifact and the exact carrier byte
digest. The builder derives only
`haft_query(action="node", symbol=<artifact-id>)`; its normalized oracle
requires `wrong_identifier_namespace`, `same_call_retryable=false`, and the
unchanged `haft_query(action="related", artifact_ref=<artifact-id>)` recovery.
At seal time the fixture is checked against the real `.haft` carrier before
the request can be accepted.

The `spec_section_read_protocol` family freezes the live-MCP regression for
project-versus-section lifecycle confusion. It derives both
`haft_spec_section(action="lifecycle", section_id=<exact-id>)` and
`haft_spec_section(action="next_step", section_id=<exact-id>)` from one
semantic request. Both must normalize to `section_id_not_applicable`, name the
project-level `ProjectSpecificationSet` object, and retain exact
`haft_query(action="spec_trace", ...)` and
`haft_query(action="spec_use", ..., use_context=<named-use>)` recovery. This
builder does not approve the section or execute the live request before P14.

Four read-only typed-memory families share a second executable builder:
`exact_profile_neighborhood`, `unknown_eoc`, `known_eoc_recall`, and
`read_affordance`. One digest-bound golden-memory fixture supplies the exact
selected TypeEnv/graph basis, EntityOfConcern, bounded context, projection
profile, dimensioned budgets, and queries. The builder derives strict
`haft memory <action> --input-file -` stdin and equivalent
`haft_query(action="memory", memory_request={"mode":<action>, ...})`
arguments from that one
semantic request. Its local oracle executes only the CLI read against the
frozen project, checks the non-authorizing interpretation contract, exact
snapshot/profile, unknown-identity abstention, scope-before-rank, and the
closed read-affordance grammar, then freezes the full normalized output digest
for live CLI/MCP comparison.

After P13 has produced the canonical fixture, exercise the read builder with:

```bash
HAFT_P14_MEMORY_READ_FIXTURE=.context/p14/fixtures/golden_memory_fixture-<CANDIDATE_PREFIX>.json \
HAFT_P14_MEMORY_READ_CANDIDATE=/absolute/path/to/haft \
HAFT_P14_MEMORY_READ_PROJECT_ROOT=/absolute/path/to/frozen/project \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14CaptureMemoryReadsAgainstCandidate$'
```

The sealed fixture basis must equal the selected project composite and graph
revision byte-for-byte; `project_current` is not accepted in this carrier.

Materialize the root-bound runtime fixtures from the passing P13 carrier and
the exact candidate. The request is coordinate-only:

```json
{
  "schema": "haft.p14.runtime-fixture-materialization-request/v1",
  "p13_evidence_path": ".context/p13/<PASSING_P13_V3_CARRIER>.json",
  "candidate_executable_path": "/absolute/path/to/haft-v9-p14-candidate"
}
```

```bash
GOMAXPROCS=1 GOFLAGS='-p=1' \
HAFT_P14_MATERIALIZE_RUNTIME_FIXTURES=.context/p14/runtime-fixture-materialization-request.json \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14MaterializeRuntimeFixtureCarriers$'
```

The materializer verifies fresh P13 evidence, snapshots the selected project's
SQLite ledger through a consistent `VACUUM INTO`, binds memory writes to the
exact selected project root, and creates six isolated fresh init templates at
their sealed future physical roots. It publishes the golden-memory and
init-matrix fixtures no-clobber. A copied ledger is never rebound to another
project root.

The complete generator consumes one canonical coordinate-only request. It does
not accept hand-authored semantic requests, oracle digests, scenario lists, or
verdicts:

```json
{
  "schema": "haft.p14.prepared-input-generation-request/v1",
  "p13_evidence_path": ".context/p13/<PASSING_P13_V3_CARRIER>.json",
  "candidate_executable_path": "/absolute/path/to/haft-v9-p14-candidate",
  "skill_carriers_root": "/absolute/path/to/installed/skills",
  "instruction_carrier_path": "/absolute/path/to/AGENTS.md",
  "mcp_config_carrier_path": "/absolute/path/to/config.toml",
  "golden_memory_fixture_path": ".context/p14/fixtures/golden_memory_fixture-<CANDIDATE_PREFIX>.json",
  "init_matrix_fixture_path": ".context/p14/fixtures/init_matrix_v4-<CANDIDATE_PREFIX>.json",
  "identifier_fixture_path": ".context/p14/fixtures/identifier_fixture.json"
}
```

All absolute paths are resolved to their physical targets. P13 must be the
passing, unchanged v3 carrier for this exact project. Fixture paths must be
canonical carriers directly under `.context/p14/fixtures/`; the generator
decodes their closed schemas, binds their byte digests, and verifies the
selected TypeEnv/graph and referenced template/artifact bytes. It captures the
candidate, FPF, dirty-state, instructions, skill-root, and MCP-config basis
through the production restart observer.

Generate the exact input after P13 and the fixtures exist:

```bash
GOMAXPROCS=1 GOFLAGS='-p=1' \
HAFT_P14_GENERATE_PREPARED_INPUT=.context/p14/prepared-input-generation-request.json \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14GeneratePreparedRequestOracleInput$'
```

One shared builder registry derives all 26 top-level scenarios. The
`fresh_initialization` scenario contains exactly six nested init subcases:
`--core-only`, full `--claude`, full `--codex`, `--codex --mcp-only`,
skills-only `--agents`, and full stable-host `--all`. There is no implicit
non-TTY Claude default. The generator runs the candidate-owned read-only FPF,
typed-memory, and code-graph local oracles, recaptures the frozen basis,
invokes the address-only P13 freshness verifier with one build worker, and
publishes one canonical no-clobber
`p14-prepared-request-oracle-input-<digest>.json`. The result remains
preparation only.

Those six entries remain the installed host-effect subcases. The same
`fresh_initialization` semantic request also binds a separate nine-profile
source-receipt matrix: TypeScript, Python, Rust, Zig manual fallback, Elixir
manual fallback, Dart manual fallback, docs-only, mixed software/model, and an
empty-project manual fallback. It is explicitly
`p13_exact_source_test_receipt`, not a seventh init subcase or an installed
runtime observation. Its exact receipt is
`internal/cli::TestOnboardProfileMatrixRunsReviewThroughPublicApplyCommand`;
P14 cannot seal the request unless that test belongs to the same passing P13
candidate basis.

The init normalizer validates semantic bytes, not file presence. It rebuilds
the canonical twelve-skill source bundle from `internal/cli/skill`, checks
adapter edition, bundle/kernel digests, every rendered Claude/Codex/agents
byte digest and mode, exact Claude JSON and Codex TOML MCP fragments, and the
managed instruction-section digest. `--all` must publish exactly the four
stable Claude/Codex manifests and no experimental carrier root. Symlinks and
non-regular semantic carriers fail closed. The same six cases seed known
legacy Claude command and Codex prompt files plus wrong-scope, foreign-name,
backup, and nested sentinels. Full stable-host selections must remove only the
owned legacy files; core-only, MCP-only, and agents-only runs must preserve
them byte-for-byte and mode-for-mode.

Seal that generated input with:

```bash
HAFT_P14_SEAL_PREPARED_INPUT=.context/p14/<GENERATED_INPUT>.json \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14SealPreparedRequestOracleCarrier$'
```

The seal re-runs read-only P13 freshness verification, recaptures the current
restart basis through the production observer, verifies candidate/FPF/carrier
bytes, and publishes an atomic no-clobber carrier under `.context/p14/`. It
does not install the candidate, restart Codex, call the live MCP, or record any
scenario as passed. Exact request generation remains a post-P13 operation;
until then the static contract is the only prepared P14 carrier.

The separate installed-observation carrier core is also closed. It binds one
sealed prepared carrier by path, byte digest, and preparation digest; one safe
runtime summary; and one observation for every declared scenario surface.
Every surface observation carries the exact prepared request digest, canonical
observed payload, timestamp, source kind, and source-receipt digest.
Normalized scenarios pass only when every surface independently reproduces the
prepared normalized digest. Live-predicate scenarios pass only when the exact
closed predicate set is observed true. Missing surfaces are rejected; an
observed mismatch is retained as `failed` evidence rather than erased.

The source kinds are closed:

- `installed_cli_execution`;
- `actual_codex_mcp_capture`;
- `restart_checkpoint_verification`.

A newly spawned `haft serve` subprocess cannot be relabelled as
`actual_codex_mcp_capture`. The runtime summary stores only safe receipt
digests, process coordinates, one-writer/wake-count results, and cleanup
posture; private checkpoint bytes and nonces are never copied into the
observation carrier. The carrier is canonical, digest-named, no-clobber, and
revalidates the unchanged prepared bytes on read.

The safe runtime-summary adapter is now implemented. The domain capability
`agenthostrestart.LoadVerifiedRuntimeSnapshot` reads the checkpoint, live-MCP
receipt, fallback receipt, loop-guard marker, gitignore posture, and temporary
stage posture under one shared store lock. It accepts only canonical
`verified` attempt-1 state, validates the private receipts inside their owning
package, and returns paths, timestamps, counters, booleans, and SHA-256
digests—never nonces or receipt bodies. P14 maps that secret-free snapshot to
the runtime binding and separately requires exact candidate/project identity,
chronology, one exact-task resume lease, one-writer posture, supervisor removal, loop-guard
reservation, and cleanup.

The installed-CLI runner source is implemented as a separate, partial capture.
It consumes one sealed prepared carrier, verifies the exact executable digest,
executes every declared `installed_cli` surface, applies a closed
builder-family normalizer, and writes one digest-named no-clobber
`captured_not_final` carrier:

```bash
GOMAXPROCS=1 GOFLAGS='-p=1' \
HAFT_P14_CAPTURE_INSTALLED_CLI=.context/p14/<SEALED_PREPARED_CARRIER>.json \
HAFT_P14_INSTALLED_CLI_CANDIDATE=/absolute/path/to/installed/haft \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14CaptureInstalledCLI$'
```

Initialization restores project and `HOME` templates at the exact sealed
physical roots to which their ledgers were originally bound. Typed-memory
mutation, concurrency, and existing-record backfill keep the exact selected
project root read-only and clone only the prepared `HOME` store per scenario.
Project bases, home templates, and their before/after states are digest-bound;
symlinks and non-regular files are rejected. Read-only FPF and typed-memory
reads, plus code Explore, use a fresh bounded clone of the sealed `HOME`
store. Their project and `HOME` semantic digests must both remain byte-equal
before and after execution. The `invalid`, `underdetermined`, and
`authority_rejection` memory scenarios apply the same two-snapshot no-write
gate; `rows_written=0` is response data, not proof of that gate.
Each command has a 90-second timeout and a four-MiB limit per output stream.
Inherited project-root, project-ID, and `HOME` overrides are stripped before
the sealed environment is applied.

The partial carrier records exact argv and stdin digest, bounded base64
stdout/stderr with matching digests, exit status, time interval, fixture
posture, and closed checks. Carrier validation reconstructs the command
results from those raw bytes, requires the exact sealed command and check IDs,
re-runs the family normalizer, and compares the derived digest with the oracle.
Commandless normalized receipts, copied oracle digests, and self-selected
check IDs fail closed. Commandless runtime/agent predicates are accepted only
when their exact captured dependency scenarios are present and valid. The
partial carrier cannot contain a release claim or substitute for live MCP or
restart evidence. The exact installed candidate has not yet been executed
because final P13 identity and the sealed real prepared carrier do not exist.

Typed-memory write preparation now distinguishes the two real execution
contexts instead of applying fixture language to both. Installed CLI requests
declare
`selected_project_read_only_with_fresh_home_clone_per_scenario`; live MCP
requests declare `selected_project_ordered_dogfood`, carry no project/home
template paths, and name semantic predecessors. The live concurrency request
is bound to the graph revision after `positive_typed_write`. The actual capture
run must execute all read-only and rejection requests before either write, then
the positive idempotent write, then concurrency. It may not relabel the selected
project as a fixture or restore its database between live calls.

The actual resumed-Codex MCP adapter source is implemented as a second partial
capture. It does not run a headless Codex CLI. After the installed candidate
has been restarted and its private runtime checkpoint has been verified,
generate one exact request packet:

```bash
GOMAXPROCS=1 GOFLAGS='-p=1' \
HAFT_P14_GENERATE_CODEX_MCP_REQUEST=.context/p14/<SEALED_PREPARED_CARRIER>.json \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14GenerateActualCodexMCPRequestPacket$'
```

The resulting `p14-codex-mcp-request-<digest>.json`
(`haft.p14.codex-mcp-request/v3`) is
`capture_requested_not_executed`. It binds the sealed preparation, verified
resumed-task/process receipt, exact ordered tool arguments, request digests,
predecessors, concurrency groups, and the two sealed natural-language
orientation prompts. Execute its calls through the current resumed Codex task
in packet order. Calls in one non-empty `parallel_group` belong to the same
concurrent invocation batch.

The capture input still projects the prompt, tool call, and response fields
needed by the closed family normalizers:

- prompt history supplies the exact user text, thread, turn, role, digest, and
  history-read time;
- task history supplies thread, turn, tool-call ID, server/tool, exact
  arguments, status, duration, turn-local ordinal/count, and the time history
  was read;
- the response projection records the corresponding MCP body, timestamp, error
  posture, base64 bytes, and digest.

Those projections are not accepted as evidence by themselves. Ingestion
locates the unique append-only Codex JSONL session for the verified thread,
reads one immutable prefix, and independently matches every projected prompt,
tool, exact argument object, call ID, turn-local ordinal/count, duration,
status, timestamp, error posture, and response body against raw
`mcp_tool_call_end` history. It rejects an extra Haft call anywhere between
request generation and history observation. The partial carrier binds the
source path, prefix byte count/digest, session metadata, selected line digests,
and a recomputable history-evidence digest. Finalization rereads the exact
prefix; later append-only session events are allowed, but changed or missing
prefix bytes fail closed.

Assemble the projections into canonical
`haft.p14.codex-mcp-capture-input/v3` JSON named
`p14-codex-mcp-capture-input-<label>.json` under `.context/p14/`, then ingest
it:

```bash
GOMAXPROCS=1 GOFLAGS='-p=1' \
HAFT_P14_INGEST_CODEX_MCP_CAPTURE=.context/p14/<CAPTURE_INPUT>.json \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14IngestActualCodexMCPCapture$'
```

Ingestion also rereads the request packet and prepared carrier, re-verifies
the current runtime snapshot, and runs the same closed FPF, memory-read,
memory-operation, identifier, and SpecSection normalizers used by the P14
oracle. It writes a canonical no-clobber
`haft.p14.codex-mcp-capture/v5` carrier with status `captured_not_final`.
A semantic mismatch is retained as failed observation rather than discarded.
The partial carrier cannot claim P14 completion, release readiness, or release
authority.

Ingestion also launches one bounded `haft serve` protocol probe from the exact
installed executable after verified live-MCP fulfillment. It preserves the
exact raw base64 request/response bytes and digests for JSON-RPC `initialize`
and `tools/list`. The validator requires the exact 12-tool order; always
advertised `haft_onboard`, `haft_entity`, memory query, and raw-memory tools;
closed nested schemas; the `{request:{...}}` `haft_memory` v2 envelope; and no
array degraded to `any[]`. Initialize instructions must contain only the
global memory, persistence, authority, code-preflight, and MethodPack
invariants. This bounded helper proves actual response bytes from the installed
server. It does not replace the verified host-owned Codex process generation.

The shared `haft_query` schema globally requires only `action`: Anthropic
rejects top-level union or conditional schemas and would take the MCP server
offline. The outer object remains `additionalProperties:false` and advertises
`memory_request`; its nested `oneOf` resolve, neighborhood, and recall branches
carry the exact closed required sets. The strict runtime rejects an omitted
`memory_request`, legacy flat memory fields, and unknown fields. P14 therefore
does not claim that conditional top-level requiredness is expressible in the
shared host-compatible schema.

Before finalization, capture the separate actual-host Claude proof. Install the
same candidate with the full Claude adapter, establish the restart checkpoint,
and start one new main Claude session in the selected project. In that session,
make exactly these three Haft calls, sequentially and without another Haft call
between the checkpoint and capture:

```text
mcp__haft__haft_query(action="status", full=false)
mcp__haft__haft_onboard(action="status")
mcp__haft__haft_query(action="status", full=false)
```

The first status call must fulfill the pending live-MCP challenge. The bounded
onboarding status read must return before the final status call begins. Identify
the new Claude PID and the single descendant `haft serve` PID, then create this
coordinate-only request under `.context/p14/`:

```json
{
  "schema": "haft.p14.claude-host-proof-request/v2",
  "prepared_carrier_path": ".context/p14/<SEALED_PREPARED_CARRIER>.json",
  "claude_pid": 12345,
  "mcp_pid": 12346
}
```

Then run:

```bash
GOMAXPROCS=1 GOFLAGS='-p=1' \
HAFT_P14_CAPTURE_CLAUDE_HOST_PROOF=.context/p14/<CLAUDE_PROOF_REQUEST>.json \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14CaptureActualClaudeHostProof$'
```

The request cannot name a history root, session file, transcript event,
response, protocol result, or Codex capture. The harness discovers the unique
main-session JSONL only under canonical `~/.claude/projects`, rejects symlinks
and subagent histories, freezes one append-only prefix, and independently reads
the observations.

The transcript must contain post-checkpoint `deferred_tools_delta` and
`mcp_instructions_delta` events for Haft. A separate bounded raw JSON-RPC
subprocess obtains `initialize` and `tools/list` from the same executable and
project root; the harness requires the Claude startup deltas to match those
instructions and tool names exactly. That subprocess is explicitly separate
protocol-byte proof, not evidence that its PID is the host-owned MCP PID and
not imported Codex proof.

The harness then requires exact direct `tool_use` / matching `tool_result`
pairs for status → bounded onboarding status → status. The first pair must
bracket the private live-MCP receipt fulfillment; all three must be successful,
sequential, non-empty, and bounded to 30 seconds. Both status response bodies
must contain the server-emitted runtime line, whose PID, start time, and
physical executable path exactly match the live-MCP receipt. Missing,
unmatched, extra, stale, errored, hung, or wrong-runtime Haft events fail
closed. The selected MCP PID must equal the live challenge receipt PID, retain
the same start generation, candidate digest and project root, and remain the
exact Claude descendant before and after transcript ingestion. The frozen
transcript prefix and both processes are re-observed during finalization;
append-only later events are allowed, but changed prefix bytes, exited/reused
processes, a wrong binary, or a cross-root session fail closed.

After checkpoint verification, both partial captures, and the Claude proof
exist, finalize one coordinate-only request:

```json
{
  "schema": "haft.p14.installed-observation-finalization-request/v2",
  "prepared_carrier_path": ".context/p14/<SEALED_PREPARED_CARRIER>.json",
  "installed_cli_capture_path": ".context/p14/<INSTALLED_CLI_CAPTURE>.json",
  "codex_mcp_capture_path": ".context/p14/<CODEX_MCP_CAPTURE>.json",
  "claude_host_proof_path": ".context/p14/<CLAUDE_HOST_PROOF>.json"
}
```

Store it under `.context/p14/` with a filename beginning
`p14-installed-observation-finalization-request`, then run:

```bash
GOMAXPROCS=1 GOFLAGS='-p=1' \
HAFT_P14_FINALIZE_INSTALLED_OBSERVATION=.context/p14/<FINALIZATION_REQUEST>.json \
  go test -count=1 -v ./internal/p14acceptance \
  -run '^TestP14FinalizeInstalledObservationCarrier$'
```

The finalizer does not accept caller-supplied verdicts or observations. It
strictly rereads the sealed preparation, installed-CLI capture, actual-Codex
MCP capture, request packet, verified private restart snapshot, and the actual
Claude proof; requires one candidate, preparation, project, task, and runtime
basis; rereads the frozen Claude transcript prefix, re-observes the Claude and
descendant MCP generations; derives the
host-process receipts; joins every declared surface in contract order; and
computes the final scenario and carrier verdicts. The final carrier binds the
Claude proof carrier/evidence digests. Missing, duplicate, stale,
wrong-generation, or cross-runtime inputs fail closed. Observed oracle
mismatches remain a canonical `failed` carrier rather than being erased.
After persisting that diagnostic carrier, the documented finalization command
exits nonzero unless the carrier status, observation status, and every scenario
verdict are `passed`; a retained failure can never appear as a green P14 run.

Only after final P13, exact candidate build/install, real prepared-input
generation, installed-CLI capture, actual-Codex MCP capture, restart
verification, and actual-host Claude proof may the complete installed
observation carrier be finalized.

The seal is fail-closed on builder coverage. The executable registry now covers
all 26 declared top-level scenarios: FPF Query, identifier namespace, the
exact SpecSection read-protocol rejection, seven read-only memory scenarios,
the six-subcase initialization matrix, five typed-memory operation protocols,
one CLI-only existing-record backfill protocol, four code-graph scenarios, and
five closed live-observation protocols for runtime identity, host resume,
cleanup, code-graph agent orientation, and typed-memory agent orientation.
Generic
scenario/surface JSON cannot pass as an exact request. The operation protocols
separately freeze
validate/admit/replay/reread, Invalid, Underdetermined, authority rejection,
concurrent same-request/conflicting-request behavior, and source-owned
dry-run/apply/idempotent-replay backfill. Builder coverage is still design-time
preparation: the real post-P13 fixture bytes, installed-candidate execution
outputs, and live observations have not happened yet.

Five historical installed-run mismatches have a closed evidence fixture and
regression. A mandatory digest-sealed tracked source extract contains all five
raw capture records, all five prepared records, and the full archived memory
fixture, so a clean checkout recomputes semantic/request/observation digests,
argv/stdin linkage, projections, dispositions, and regression claims without
depending on ignored `.context` files. When the original ignored
`.context/p14` carriers are present, the extract is additionally compared
structurally against their full-file SHA-256 identities:

- `positive_typed_write` and `concurrency_idempotency` are `stale` oracle
  vocabulary (`applied` now normalizes to semantic `committed`);
- `invalid` and `underdetermined` are `oracle` fixture defects repaired with
  an exact duplicate entity and an unadmitted bounded context;
- `existing_record_backfill` is an `oracle` parser defect; only that family
  accepts one whitespace-formatted JSON document while compact-output
  contracts remain strict.

The historical capture files are unchanged and are not retroactively called
passing.
