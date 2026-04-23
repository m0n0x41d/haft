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
task open-sleigh:harness-haft
```

`task open-sleigh:smoke-real-haft` builds the current Haft binary, creates a
temporary Haft project, creates a real `WorkCommission` with `haft commission
create`, starts Open-Sleigh against a real temporary `haft serve`, waits for
idle, and verifies no runnable commissions remain.

For manual local setup, seed the project through the CLI:

```sh
haft commission create --json commission.json
haft commission list-runnable
haft commission claim wc-...
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
