# Open-Sleigh First Real Run

This checklist records the first non-mock canary run. It separates setup
claims from runtime evidence.

## Local Commission Canaries

Run the local-only canaries before any tracker-backed run:

```sh
mix test test/open_sleigh/commission_canary_test.exs
```

These canaries use `test/fixtures/commissions/*.json` through the local
`CommissionSource` adapter. They do not require Linear, Jira, GitHub, or Haft
server credentials, and they do not start ProjectionWriterAgent.

Record:

- green local-only: fixture commission listed, claimed for preflight, admitted
  by deterministic gates, and terminal diff stays inside Scope
- stale-block: decision revision drift blocks as `blocked_stale` before Execute
- scope-block: an in-workspace mutation outside Scope fails terminally as
  `mutation_outside_commission_scope`

## Today Harness Path

Use this path when Open-Sleigh should execute Haft-owned local
`WorkCommission` fixtures without requiring Linear/Jira/GitHub or a live Haft
server yet. It keeps the work source commission-first, uses mock Haft for
ProblemCard/artifact IO, uses the deterministic judge, and leaves the agent
adapter real.

```sh
cd open-sleigh
mix deps.get
export REPO_URL=/Users/ivanzakutnii/Repos/projects/haft
mix open_sleigh.doctor --path sleigh.commission.md.example --mock-haft
mix open_sleigh.start --path sleigh.commission.md.example --mock-haft --mock-judge
```

For a bounded smoke instead of a long-running harness, add `--once`. For a fully
deterministic dry run, add `--mock-agent` or use `--mock`. Add
`--once-timeout-ms=5000` when the smoke should wait for the mock phase loop to
become idle before printing status.

Use the Haft commission-source path when testing the MVP-1R intake contract
itself. With `--mock-haft`, the in-memory Haft supplies
`haft_commission.list_runnable` and `haft_commission.claim_for_preflight`:

```sh
cd open-sleigh
mix open_sleigh.doctor --path sleigh.haft.md.example --mock-haft
mix open_sleigh.start --path sleigh.haft.md.example --mock --once --once-timeout-ms=5000
```

From the repo root the same checks are available as:

```sh
task open-sleigh:doctor-haft
task open-sleigh:smoke-haft
task open-sleigh:smoke-real-haft
task open-sleigh:smoke-real-haft-dynamic
task open-sleigh:smoke-real-haft-from-decision
task open-sleigh:smoke-real-haft-batch
task open-sleigh:smoke-real-haft-plan
task open-sleigh:smoke-harness-command
task open-sleigh:harness-haft
task open-sleigh:harness-from-decision DECISION=dec-...
task open-sleigh:harness-from-decisions DECISIONS="dec-a dec-b"
task open-sleigh:harness-plan PLAN=.haft/plans/implementation.yaml
```

`task open-sleigh:smoke-real-haft` builds the current Haft binary, creates a
temporary Haft project, creates a real `WorkCommission` with `haft commission
create`, starts Open-Sleigh against a real temporary `haft serve`, waits for
idle, and verifies no runnable commissions remain.

For normal operator use, use the packaged Haft command:

```sh
# normal path: all active DecisionRecords without WorkCommissions
haft harness run

# one decision, explicit override
haft harness run dec-...

# several independent decisions, explicit override
haft harness run dec-a dec-b dec-c

# all active decisions linked to one ProblemCard
haft harness run --problem prob-...

# all active decisions in one optional Haft context
haft harness run --context harness-mvp

# the whole active DecisionRecord backlog, including already commissioned decisions
haft harness run --all-active-decisions

# ordered decisions: dec-b waits for dec-a, dec-c waits for dec-b
haft harness run dec-a dec-b dec-c --sequential

# dry smoke: create commissions, run one mock Open-Sleigh pass, then exit
haft harness run --mock --once

# prepare only: generate .haft/plans/*.yaml and WorkCommissions, do not start runtime
haft harness run --prepare-only
```

`haft harness run` is the preferred path. With no selectors it selects active
DecisionRecords that do not already have WorkCommissions. `context` is only an
optional Haft metadata filter; the operator does not need to maintain one for
normal use. The command generates an observable plan under `.haft/plans/`,
creates/reuses WorkCommissions, writes a temporary `sleigh.md`, and starts
Open-Sleigh. Reusing the same `--plan` does not create duplicate commissions
unless `--force-create-commissions` is set.

Runtime lookup order:

1. `--runtime`
2. `HAFT_OPEN_SLEIGH_RUNTIME` or `OPEN_SLEIGH_RUNTIME`
3. repo-local `open-sleigh/`
4. installed runtime at `~/.haft/runtimes/open-sleigh/current`

Release installs use the installed runtime and do not require users to install
Elixir/Mix for normal harness runs. Source checkout runs may still use `mix`
directly.

For lower-level manual setup, seed the project through the commission CLI:

```sh
haft commission create-from-decision dec-... \
  --allowed-path open-sleigh/lib/open_sleigh/commission_source/haft.ex \
  --lock open-sleigh/lib/open_sleigh/commission_source/haft.ex \
  --evidence "mix test test/open_sleigh/commission_source/haft_test.exs"
haft commission create-batch dec-a dec-b dec-c
haft commission create-from-plan .haft/plans/implementation.yaml
haft commission create --json commission.json
haft commission list-runnable
haft commission claim wc-...
```

`create-from-decision` / `create-batch` are the preferred operator paths: Haft loads the active
DecisionRecord, freezes `decision_revision_hash`, derives default scope from
`affected_files` when possible, and writes the runnable WorkCommission.
`create-from-plan` accepts an ImplementationPlan-lite YAML/JSON carrier:

```yaml
id: plan-mvp2
revision: p1
repo_ref: local:haft
base_sha: base-r1
target_branch: feature/mvp2
projection_policy: local_only
valid_for: 168h
defaults:
  allowed_actions: [edit_files, run_tests]
  evidence_requirements:
    - go test ./internal/cli
decisions:
  - ref: dec-a
  - ref: dec-b
    depends_on: [dec-a]
```

`depends_on` uses DecisionRecord ids from the same plan. Haft maps those refs
to concrete WorkCommission ids and enforces them in `list-runnable` and
`claim`, so Open-Sleigh only sees commissions whose prerequisites are already
completed.
`open_sleigh.start` replenishes dynamically while it is running, so a
commission created after startup is consumed without restarting the harness.
`task open-sleigh:smoke-real-haft-from-decision` proves the same path without a
hand-written commission fixture: it creates a real ProblemCard and
DecisionRecord through `haft serve`, runs `haft commission create-from-decision`,
and verifies Open-Sleigh consumes the resulting WorkCommission.
`task open-sleigh:smoke-real-haft-batch` does the same for a two-decision queue
using `haft commission create-batch`.
`task open-sleigh:smoke-real-haft-plan` proves the plan-file path.

For an operator run against a real local DecisionRecord:

```sh
haft harness run
haft harness run dec-...
haft harness run --problem prob-...
haft harness run --context harness-mvp
haft harness run --all-active-decisions
haft harness run dec-a dec-b dec-c --sequential
haft harness run --plan .haft/plans/implementation.yaml
task open-sleigh:harness-from-decision DECISION=dec-...
task open-sleigh:harness-from-decisions DECISIONS="dec-a dec-b dec-c"
task open-sleigh:harness-plan PLAN=.haft/plans/implementation.yaml
```

Useful environment overrides:

```sh
ONCE=1 MOCK_AGENT=1 MOCK_JUDGE=1 task open-sleigh:harness-from-decision DECISION=dec-...
ONCE=1 MOCK_AGENT=1 MOCK_JUDGE=1 task open-sleigh:harness-from-decisions DECISIONS="dec-a dec-b"
COMMISSION_ARGS='--allowed-path internal/cli --lock internal/cli' task open-sleigh:harness-from-decision DECISION=dec-...
```

Monitor from another shell:

```sh
cd open-sleigh
mix open_sleigh.status --path ~/.open-sleigh/status.json
tail -f ~/.open-sleigh/runtime.jsonl
```

## Inputs

- Date:
- Operator:
- Config path:
- Status path:
- Log path:
- Workspace root:
- Tracker project/team:
- Tracker ticket:
- Repository URL:
- Target branch:
- ProblemCard reference:
- Expected outcome:

## Preflight

Run:

```sh
mix deps.get
mix open_sleigh.doctor --path sleigh.md
mix open_sleigh.gate_report --json
```

Evidence:

- Doctor passed: yes/no
- Gate report passed: yes/no
- Errors:
- Warnings:
- Notes:

## Canary Run

Run one polling pass first:

```sh
mix open_sleigh.start --path sleigh.md --once
```

Evidence:

- Command exit: pass/fail
- Status snapshot updated: yes/no
- Runtime log entries written: yes/no
- Tracker ticket comment posted: yes/no
- Workspace created: yes/no
- Haft artifact written: yes/no
- Human gate requested: yes/no
- Tracker transition attempted: yes/no

## Status Snapshot

Run:

```sh
mix open_sleigh.status --path <status-path>
mix open_sleigh.status --path <status-path> --json
```

If `engine.status_http.enabled: true`, also open:

```text
http://127.0.0.1:<port>/dashboard
http://127.0.0.1:<port>/api/v1/state
```

Record:

- claimed:
- running:
- pending_human:
- retries:
- failures:
- stale:
- recent failure summaries:

## Runtime Log

Inspect:

```sh
tail -n 20 <log-path>
```

Record event ids for relevant runtime events:

- runtime_started:
- tracker_poll_requested:
- once_poll_completed:
- runtime_stopping:
- failure event ids:

## Decision

- First real run accepted: yes/no
- Blocking issue:
- Next command:
- Cleanup required:
