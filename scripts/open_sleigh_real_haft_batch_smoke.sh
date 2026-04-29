#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp="$(mktemp -d)"
project_id=""

cleanup() {
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
sleigh_path="$tmp/sleigh.batch.md"

(cd "$repo" && go build -o "$haftbin" ./cmd/haft)

mkdir -p "$project/internal/cli"
(cd "$project" && "$haftbin" init --local >/dev/null)
project_id="$(awk '/^id:/ {print $2}' "$project/.haft/project.yaml")"
printf 'package cli\n' > "$project/internal/cli/commission.go"
printf 'package cli\n' > "$project/internal/cli/serve_commission.go"

extract_tool_id() {
  local prefix="$1"
  python3 -c '
import json
import re
import sys

prefix = sys.argv[1]
payload = json.load(sys.stdin)
text = payload["result"]["content"][0]["text"]
match = re.search(r"ID: (" + re.escape(prefix) + r"-[A-Za-z0-9-]+)", text)
if not match:
    raise SystemExit("missing " + prefix + " id in tool response")
print(match.group(1))
' "$prefix"
}

create_decision() {
  local title="$1"
  local affected_file="$2"
  local problem_call="$tmp/problem-$title.json"
  local decision_call="$tmp/decision-$title.json"

  cat > "$problem_call" <<JSON
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"haft_problem","arguments":{"action":"frame","title":"$title problem","signal":"Open-Sleigh needs batch work for $affected_file.","acceptance":"A batch-created commission is consumed.","affected_files":["$affected_file"]}}}
JSON

  problem_response="$(HAFT_PROJECT_ROOT="$project" "$haftbin" serve < "$problem_call")"
  problem_ref="$(printf '%s' "$problem_response" | extract_tool_id prob)"

  cat > "$decision_call" <<JSON
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"haft_decision","arguments":{"action":"decide","problem_ref":"$problem_ref","selected_title":"$title","why_selected":"The harness should consume one bounded WorkCommission per DecisionRecord.","selection_policy":"Prefer per-decision authorization with Open-Sleigh queue consumption.","counterargument":"A single large commission would be simpler.","weakest_link":"Overlapping locksets must control concurrency.","why_not_others":[{"variant":"Single large commission","reason":"It hides per-decision authorization boundaries."}],"rollback":{"triggers":["Batch-created commissions are not consumed."]},"affected_files":["$affected_file"],"evidence_requirements":["go test ./internal/cli"]}}}
JSON

  decision_response="$(HAFT_PROJECT_ROOT="$project" "$haftbin" serve < "$decision_call")"
  printf '%s' "$decision_response" | extract_tool_id dec
}

decision_a="$(create_decision "Batch smoke A" "internal/cli/commission.go")"
decision_b="$(create_decision "Batch smoke B" "internal/cli/serve_commission.go")"

(cd "$project" && "$haftbin" commission create-batch "$decision_a" "$decision_b" \
  --repo-ref local:haft-batch \
  --base-sha base-r1 \
  --target-branch feature/batch-smoke >/dev/null)

cat > "$sleigh_path" <<YAML
---
engine:
  poll_interval_ms: 30000
  status_path: $status_path
  status_interval_ms: 100
  log_path: $log_path
  concurrency: 2
  status_http:
    enabled: false

commission_source:
  kind: haft
  selector: runnable
  max_claims: 50
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
  max_concurrent_agents: 2

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

(cd "$repo/open-sleigh" && mix open_sleigh.start --path "$sleigh_path" --mock-agent --mock-judge --once --once-timeout-ms=5000)

runnable="$(cd "$project" && "$haftbin" commission list-runnable)"
printf '%s\n' "$runnable"

case "$runnable" in
  *'"commissions":[]'*)
    ;;
  *)
    echo "expected no runnable WorkCommissions after batch smoke" >&2
    exit 1
    ;;
esac
