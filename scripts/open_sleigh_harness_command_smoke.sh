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
workspace_root="$tmp/workspaces"
runtime="${OPEN_SLEIGH_RUNTIME:-${HAFT_OPEN_SLEIGH_RUNTIME:-$repo/open-sleigh}}"

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

create_problem() {
  local title="$1"
  local affected_file="$2"
  local problem_call="$tmp/problem-$title.json"

  cat > "$problem_call" <<JSON
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"haft_problem","arguments":{"action":"frame","context":"harness-command-smoke","title":"$title problem","signal":"The packaged harness command must consume decisions for $affected_file without listing every id.","acceptance":"One haft harness command selects decisions by context, creates commissions, and runs Open-Sleigh.","affected_files":["$affected_file"]}}}
JSON

  problem_response="$(HAFT_PROJECT_ROOT="$project" "$haftbin" serve < "$problem_call")"
  printf '%s' "$problem_response" | extract_tool_id prob
}

create_decision() {
  local problem_ref="$1"
  local title="$2"
  local affected_file="$3"
  local decision_call="$tmp/decision-$title.json"

  cat > "$decision_call" <<JSON
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"haft_decision","arguments":{"action":"decide","problem_ref":"$problem_ref","selected_title":"$title","why_selected":"The operator should not hand-write ImplementationPlan YAML for the common path.","selection_policy":"Prefer one transparent Haft command that still leaves plan and commission artifacts inspectable.","counterargument":"Taskfile scripts were already enough for smokes.","weakest_link":"The command must not hide duplicate commission creation or dependency rules.","why_not_others":[{"variant":"Manual YAML","reason":"It is too much ceremony for normal harness usage."}],"rollback":{"triggers":["haft harness run cannot consume decision-created work."]},"affected_files":["$affected_file"],"evidence_requirements":["go test ./internal/cli"]}}}
JSON

  decision_response="$(HAFT_PROJECT_ROOT="$project" "$haftbin" serve < "$decision_call")"
  printf '%s' "$decision_response" | extract_tool_id dec
}

problem_a="$(create_problem "Harness command A" "internal/cli/commission.go")"
problem_b="$(create_problem "Harness command B" "internal/cli/serve_commission.go")"
create_decision "$problem_a" "Harness command A" "internal/cli/commission.go" >/dev/null
create_decision "$problem_b" "Harness command B" "internal/cli/serve_commission.go" >/dev/null

(cd "$project" && "$haftbin" harness run \
  --mock \
  --once \
  --once-timeout-ms 8000 \
  --runtime "$runtime" \
  --repo-url "$repo" \
  --repo-ref local:haft-harness-command \
  --base-sha base-r1 \
  --target-branch feature/harness-command-smoke \
  --status-path "$status_path" \
  --log-path "$log_path" \
  --workspace-root "$workspace_root")

runnable="$(cd "$project" && "$haftbin" commission list-runnable)"
printf '%s\n' "$runnable"

case "$runnable" in
  *'"commissions":[]'*)
    ;;
  *)
    echo "expected no runnable WorkCommissions after harness command smoke" >&2
    exit 1
    ;;
esac
