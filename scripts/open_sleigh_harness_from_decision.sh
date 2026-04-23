#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
plan_path="${PLAN:-}"
decision_refs=("$@")
if [[ "${#decision_refs[@]}" -eq 0 && -n "${DECISIONS:-}" ]]; then
  read -r -a decision_refs <<< "$DECISIONS"
fi
if [[ "${#decision_refs[@]}" -eq 0 && -n "${DECISION:-}" ]]; then
  decision_refs=("$DECISION")
fi

if [[ -z "$plan_path" && "${#decision_refs[@]}" -eq 0 ]]; then
  echo "usage: task open-sleigh:harness-from-decision DECISION=dec-..." >&2
  echo "   or: task open-sleigh:harness-from-decisions DECISIONS='dec-a dec-b'" >&2
  echo "   or: task open-sleigh:harness-plan PLAN=.haft/plans/plan.yaml" >&2
  echo "   or: scripts/open_sleigh_harness_from_decision.sh dec-a dec-b" >&2
  exit 2
fi

tmp="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT

haftbin="$tmp/haft"
sleigh_path="$tmp/sleigh.harness-from-decision.md"
status_path="${SLEIGH_STATUS_PATH:-$HOME/.open-sleigh/status.json}"
log_path="${SLEIGH_LOG_PATH:-$HOME/.open-sleigh/runtime.jsonl}"
workspace_root="${SLEIGH_WORKSPACE_ROOT:-$HOME/.open-sleigh/workspaces}"
repo_ref="${REPO_REF:-local:$(basename "$repo")}"
valid_for="${VALID_FOR:-168h}"
concurrency="${CONCURRENCY:-2}"
max_claims="${MAX_CLAIMS:-50}"
poll_interval_ms="${POLL_INTERVAL_MS:-30000}"
repo_url="${REPO_URL:-$repo}"

extra_commission_args=()
if [[ -n "${COMMISSION_ARGS:-}" ]]; then
  read -r -a extra_commission_args <<< "$COMMISSION_ARGS"
fi

(cd "$repo" && go build -o "$haftbin" ./cmd/haft)

if [[ -n "$plan_path" ]]; then
  (cd "$repo" && "$haftbin" commission create-from-plan "$plan_path" \
    --repo-ref "$repo_ref" \
    --valid-for "$valid_for" \
    "${extra_commission_args[@]}")
elif [[ "${#decision_refs[@]}" -eq 1 ]]; then
  (cd "$repo" && "$haftbin" commission create-from-decision "${decision_refs[0]}" \
    --repo-ref "$repo_ref" \
    --valid-for "$valid_for" \
    "${extra_commission_args[@]}")
else
  (cd "$repo" && "$haftbin" commission create-batch "${decision_refs[@]}" \
    --repo-ref "$repo_ref" \
    --valid-for "$valid_for" \
    "${extra_commission_args[@]}")
fi

cat > "$sleigh_path" <<YAML
---
engine:
  poll_interval_ms: $poll_interval_ms
  status_path: $status_path
  status_interval_ms: 5000
  log_path: $log_path
  concurrency: $concurrency
  status_http:
    enabled: false
    host: 127.0.0.1
    port: 4767

commission_source:
  kind: haft
  selector: runnable
  max_claims: $max_claims
  lease_timeout_s: 300
  plan_ref: null

projection:
  mode: local_only
  targets: []
  writer_profile: manager_plain

agent:
  kind: codex
  version_pin: local
  command: codex app-server
  max_turns: ${AGENT_MAX_TURNS:-20}
  max_tokens_per_turn: ${AGENT_MAX_TOKENS_PER_TURN:-80000}
  wall_clock_timeout_s: ${AGENT_WALL_CLOCK_TIMEOUT_S:-600}
  max_retry_backoff_ms: 300000
  max_concurrent_agents: $concurrency

codex:
  approval_policy: ${CODEX_APPROVAL_POLICY:-never}
  thread_sandbox: ${CODEX_THREAD_SANDBOX:-workspace-write}
  turn_sandbox_policy:
    type: workspaceWrite
  read_timeout_ms: 5000
  turn_timeout_ms: ${CODEX_TURN_TIMEOUT_MS:-3600000}
  stall_timeout_ms: 300000

judge:
  kind: codex
  adapter_version: mvp1-judge
  max_tokens_per_turn: 4000
  wall_clock_timeout_s: 120

workspace:
  root: $workspace_root
  cleanup_policy: keep

hooks:
  timeout_ms: 60000
  failure_policy:
    after_create: blocking
    before_run: blocking
    after_run: warning
  after_create: |
    git clone --depth 1 $repo_url .
    mix deps.get || true
  before_run: |
    git pull --ff-only origin main || true
  after_run: null
  before_remove: null

haft:
  command: "HAFT_PROJECT_ROOT=$repo $haftbin serve"
  version: local

external_publication:
  branch_regex: "^(main|master|release/.*)$"
  tracker_transition_to: []
  approvers: ["ivan@weareocta.com"]
  timeout_h: 24

phases:
  preflight:
    agent_role: preflight_checker
    tools: [haft_query, read, grep, bash]
    gates:
      structural:
        - commission_runnable
        - decision_fresh
        - scope_snapshot_fresh
      semantic: []
  frame:
    agent_role: frame_verifier
    tools: [haft_query, read, grep]
    gates:
      structural:
        - problem_card_ref_present
        - described_entity_field_present
        - valid_until_field_present
      semantic: [object_of_talk_is_specific]
  execute:
    agent_role: executor
    tools: [read, write, edit, bash, haft_note]
    gates:
      structural: [design_runtime_split_ok]
      semantic: [lade_quadrants_split_ok]
  measure:
    agent_role: measurer
    tools: [haft_decision, haft_refresh]
    gates:
      structural:
        - evidence_ref_not_self
        - valid_until_field_present
      semantic: [no_self_evidence_semantic]
---

# Prompt templates

## Preflight
You are the Commission Preflight checker. Given WorkCommission {{commission.id}},
read the linked ProblemCard, DecisionRecord, scope, base branch, and lockset.
Report whether current context materially changed. You do not authorize
execution; Haft decides after validating deterministic preflight facts.

## Frame
You are the Frame verifier. Given WorkCommission {{commission.id}} and linked
ProblemCardRef {{commission.problem_card_ref}}, verify that the upstream
ProblemCard is present, fresh, and sufficiently specific.

## Execute
You are the Executor. Given DecisionRecord {{decision.id}} and WorkCommission
{{commission.id}}, implement only the bounded scope and produce external
evidence.

## Measure
You are the Measurer. Given WorkCommission {{commission.id}}, assemble external
evidence and decide whether the measured outcome passes.
YAML

run_args=()
if [[ "${ONCE:-0}" == "1" ]]; then
  run_args+=(--once --once-timeout-ms="${ONCE_TIMEOUT_MS:-5000}")
fi
if [[ "${MOCK_AGENT:-0}" == "1" ]]; then
  run_args+=(--mock-agent)
fi
if [[ "${MOCK_JUDGE:-0}" == "1" ]]; then
  run_args+=(--mock-judge)
fi

if [[ -n "$plan_path" ]]; then
  printf 'Created/queued commission(s) from %s\n' "$plan_path"
else
  printf 'Created/queued commission(s) for %s\n' "${decision_refs[*]}"
fi
printf 'Open-Sleigh config: %s\n' "$sleigh_path"
printf 'Status: %s\n' "$status_path"
printf 'Runtime log: %s\n' "$log_path"

(cd "$repo/open-sleigh" && REPO_URL="$repo_url" mix open_sleigh.start --path "$sleigh_path" "${run_args[@]}")
