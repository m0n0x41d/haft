# Open-Sleigh First Real Run

This checklist records the first non-mock canary run. It separates setup
claims from runtime evidence.

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
