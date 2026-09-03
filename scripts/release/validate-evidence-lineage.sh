#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 8 ]; then
  echo "usage: validate-evidence-lineage.sh RUN_JSON ARTIFACTS_JSON RUN_ID CANDIDATE_SHA WORKFLOW_PATH EVENT ARTIFACT_NAME REPOSITORY" >&2
  exit 64
fi

run_json="$1"
artifacts_json="$2"
run_id="$3"
candidate_sha="$4"
workflow_path="$5"
event_name="$6"
artifact_name="$7"
repository="$8"

case "$run_id" in
  ''|*[!0-9]*)
    echo "evidence run ID must contain only decimal digits" >&2
    exit 1
    ;;
esac

if [[ ! "$candidate_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "evidence candidate must be a full lowercase Git SHA: $candidate_sha" >&2
  exit 1
fi

if [ -z "$workflow_path" ] || [ -z "$event_name" ] ||
   [ -z "$artifact_name" ] || [ -z "$repository" ]; then
  echo "evidence lineage selectors must be non-empty" >&2
  exit 1
fi
if [ "$workflow_path" = "*" ] || [ "$event_name" = "*" ]; then
  echo "evidence lineage selectors must name one exact workflow and event" >&2
  exit 1
fi

test -f "$run_json"
test -f "$artifacts_json"

jq -e \
  --argjson run_id "$run_id" \
  --arg sha "$candidate_sha" \
  --arg workflow "$workflow_path" \
  --arg event "$event_name" \
  --arg repository "$repository" \
  '.id == $run_id and
   .status == "completed" and
   .conclusion == "success" and
   .head_sha == $sha and
   .repository.full_name == $repository and
   .head_repository.full_name == $repository and
   .path == $workflow and
   .event == $event' \
  "$run_json" >/dev/null

jq -e \
  --arg name "$artifact_name" \
  '([.artifacts[] | select(.name == $name)]) as $matches |
   .total_count == 1 and
   ($matches | length) == 1 and
   $matches[0].expired == false and
   ($matches[0].id | type) == "number" and
   ($matches[0].id > 0) and
   ($matches[0].digest | type) == "string" and
   ($matches[0].digest | test("^sha256:[0-9a-f]{64}$"))' \
  "$artifacts_json" >/dev/null

printf 'evidence lineage accepted: run=%s workflow=%s artifact=%s sha=%s\n' \
  "$run_id" "$workflow_path" "$artifact_name" "$candidate_sha"
