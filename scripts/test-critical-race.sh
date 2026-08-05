#!/usr/bin/env bash

set -euo pipefail

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
manifest="$repository_root/internal/p13acceptance/manifest.json"
cases_file=$(mktemp "${TMPDIR:-/tmp}/haft-critical-race.XXXXXX")
trap 'rm -f "$cases_file"' EXIT

cd "$repository_root"

jq -er '
  .suites[]
  | select(.id == "go_race" and .kind == "go_test_race_critical")
  | .go_race_cases[]
  | [.package, ("^(" + (.tests | join("|")) + ")$")]
  | @tsv
' "$manifest" > "$cases_file"

if [ ! -s "$cases_file" ]; then
  echo "critical-race profile is empty" >&2
  exit 1
fi

export GOMAXPROCS="${GOMAXPROCS:-2}"
timeout="${HAFT_CRITICAL_RACE_TIMEOUT:-3h}"

while IFS=$'\t' read -r package pattern; do
  go test \
    -race \
    -count=1 \
    -timeout="$timeout" \
    -p=1 \
    -cpu=2 \
    -run "$pattern" \
    "$package"
done < "$cases_file"
