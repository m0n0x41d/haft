#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
project_id=""
sleigh_pid=""

cleanup() {
  if [[ -n "$sleigh_pid" ]] && kill -0 "$sleigh_pid" 2>/dev/null; then
    kill "$sleigh_pid" 2>/dev/null || true
    wait "$sleigh_pid" 2>/dev/null || true
  fi
  rm -rf "$tmp"
  if [[ "$project_id" == qnt_* ]]; then
    rm -rf "$HOME/.haft/projects/$project_id"
  fi
}
trap cleanup EXIT

haftbin="$tmp/haft"
project="$tmp/project"
status_path="$tmp/status.json"
log_path="$tmp/runtime.jsonl"
sleigh_path="$tmp/sleigh.dynamic-haft.md"
engine_log="$tmp/open-sleigh.log"

(cd "$repo" && go build -o "$haftbin" ./cmd/haft)

mkdir -p "$project"
(cd "$project" && "$haftbin" init --local >/dev/null)
project_id="$(awk '/^id:/ {print $2}' "$project/.haft/project.yaml")"

cat > "$sleigh_path" <<YAML
---
engine:
  poll_interval_ms: 200
  status_path: $status_path
  status_interval_ms: 100
  log_path: $log_path
  concurrency: 1
  status_http:
    enabled: false

commission_source:
  kind: haft
  selector: runnable
  max_claims: 10
  lease_timeout_s: 300
  plan_ref: null

projection:
  mode: local_only
  targets: []
  writer_profile: manager_plain

agent:
  kind: mock
  version_pin: test
  command: mock
  max_turns: 3
  max_tokens_per_turn: 80000
  wall_clock_timeout_s: 60
  max_retry_backoff_ms: 1000
  max_concurrent_agents: 1

judge:
  kind: mock
  adapter_version: test
  max_tokens_per_turn: 4000
  wall_clock_timeout_s: 60

workspace:
  root: $tmp/workspaces
  cleanup_policy: keep

hooks:
  timeout_ms: 60000
  failure_policy:
    after_create: blocking
    before_run: blocking
    after_run: warning
  after_create: null
  before_run: null
  after_run: null

haft:
  command: "HAFT_PROJECT_ROOT=$project $haftbin serve"
  version: test

external_publication:
  branch_regex: "^(main|master|release/.*)$"
  tracker_transition_to: []
  approvers: ["smoke@example.com"]
  timeout_h: 24

phases:
  preflight:
    agent_role: preflight_checker
    tools: [haft_query, read]
    gates:
      structural: []
      semantic: []
  frame:
    agent_role: frame_verifier
    tools: [haft_query, read]
    gates:
      structural: []
      semantic: []
  execute:
    agent_role: executor
    tools: [read, write, bash, haft_note]
    gates:
      structural: []
      semantic: []
  measure:
    agent_role: measurer
    tools: [haft_decision, haft_refresh]
    gates:
      structural: []
      semantic: []
---

# Prompt templates

## Preflight
Check WorkCommission {{commission.id}}.

## Frame
Verify frame for {{commission.id}}.

## Execute
Execute {{commission.id}}.

## Measure
Measure {{commission.id}}.
YAML

(cd "$repo/open-sleigh" && mix open_sleigh.start --path "$sleigh_path" --mock-agent --mock-judge >"$engine_log" 2>&1) &
sleigh_pid=$!

for _ in {1..50}; do
  if [[ -s "$status_path" ]]; then
    break
  fi
  if ! kill -0 "$sleigh_pid" 2>/dev/null; then
    cat "$engine_log" >&2 || true
    exit 1
  fi
  sleep 0.1
done

cat > "$tmp/commission.json" <<JSON
{
  "id": "wc-real-haft-dynamic-001",
  "decision_ref": "dec-real-haft-dynamic-001",
  "decision_revision_hash": "decision-r1",
  "problem_card_ref": "pc-real-haft-dynamic-001",
  "implementation_plan_ref": "plan-real-haft-dynamic-001",
  "implementation_plan_revision": "plan-r1",
  "projection_policy": "local_only",
  "state": "queued",
  "valid_until": "2099-01-01T00:00:00Z",
  "fetched_at": "2026-04-22T10:00:00Z",
  "evidence_requirements": [],
  "scope": {
    "repo_ref": "local:haft",
    "base_sha": "base-r1",
    "target_branch": "feature/real-haft-dynamic",
    "allowed_paths": ["**/*"],
    "forbidden_paths": [],
    "allowed_actions": ["edit_files", "run_tests"],
    "affected_files": ["**/*"],
    "allowed_modules": [],
    "lockset": ["**/*"]
  }
}
JSON

(cd "$project" && "$haftbin" commission create --json "$tmp/commission.json" >/dev/null)

for _ in {1..100}; do
  runnable="$(cd "$project" && "$haftbin" commission list-runnable)"
  status="$(cat "$status_path" 2>/dev/null || true)"
  compact_status="$(printf '%s' "$status" | tr -d '[:space:]')"

  if [[ "$runnable" == *'"commissions":[]'* ]] &&
     [[ "$compact_status" == *'"running":[]'* ]] &&
     [[ "$compact_status" == *'"claimed":[]'* ]] &&
     [[ "$compact_status" == *'"retries":{}'* ]]; then
    printf '%s\n' "$runnable"
    exit 0
  fi

  if ! kill -0 "$sleigh_pid" 2>/dev/null; then
    cat "$engine_log" >&2 || true
    exit 1
  fi

  sleep 0.2
done

echo "expected dynamic WorkCommission to be consumed without restart" >&2
echo "--- open-sleigh log ---" >&2
cat "$engine_log" >&2 || true
echo "--- status ---" >&2
cat "$status_path" >&2 || true
echo >&2
echo "--- runnable ---" >&2
(cd "$project" && "$haftbin" commission list-runnable) >&2 || true
exit 1
