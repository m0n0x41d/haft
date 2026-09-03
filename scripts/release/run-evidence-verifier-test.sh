#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: run-evidence-verifier-test.sh PACKAGE TEST_NAME" >&2
  exit 64
fi

package="$1"
test_name="$2"
case "${package}:${test_name}" in
  ./internal/p13acceptance:TestP13VerifyFreezeInputCandidate | \
  ./internal/p13acceptance:TestP13ManifestStructureAndAnchors | \
  ./internal/p13acceptance:TestP13VerifyAcceptanceEvidenceFresh | \
  ./internal/p14acceptance:TestP14VerifyReleaseEvidenceBundle)
    ;;
  *)
    echo "release evidence verifier test is not in the closed allowlist" >&2
    exit 64
    ;;
esac

test_pattern="^${test_name}$"
listed=$(go test "$package" -list "$test_pattern")
if [ "$(printf '%s\n' "$listed" | grep -xc "$test_name")" -ne 1 ]; then
  echo "release evidence verifier test is absent or duplicated: $test_name" >&2
  exit 1
fi

events=$(mktemp "${TMPDIR:-/tmp}/haft-release-verifier.XXXXXX")
trap 'rm -f "$events"' EXIT

go test -count=1 -timeout=5m -json "$package" \
  -run "$test_pattern" | tee "$events"

jq -s -e \
  --arg test "$test_name" \
  '([.[] | select(.Test == $test and .Action == "run")] | length) == 1 and
   ([.[] | select(.Test == $test and .Action == "pass")] | length) == 1 and
   ([.[] | select(.Test == $test and
                  (.Action == "skip" or .Action == "fail"))] | length) == 0' \
  "$events" >/dev/null
