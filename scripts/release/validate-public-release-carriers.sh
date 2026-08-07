#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "usage: validate-public-release-carriers.sh CANDIDATE_SHA" >&2
  exit 64
fi

candidate_sha="$1"
if [[ ! "$candidate_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "public release carrier candidate must be a full lowercase Git SHA: $candidate_sha" >&2
  exit 1
fi

repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"

head_sha=$(git rev-parse HEAD)
if [ "$head_sha" != "$candidate_sha" ]; then
  echo "public release carrier check requires candidate HEAD $candidate_sha, observed $head_sha" >&2
  exit 1
fi

git cat-file -e "${candidate_sha}^{commit}"

public_decision_carriers=(
  ".haft/decisions/dec-20260716-318cdec5.md"
  ".haft/decisions/dec-20260716-11f33e36.md"
)

public_execution_carriers=(
  ".context/haft-v9-deterministic-closeout.plan.md"
  ".context/haft-v9-scope-freeze-inventory-20260728.md"
)

public_release_carriers=(
  "${public_decision_carriers[@]}"
  "${public_execution_carriers[@]}"
)

for carrier in "${public_release_carriers[@]}"; do
  if [ ! -f "$carrier" ]; then
    echo "public release carrier is missing from the checkout: $carrier" >&2
    exit 1
  fi

  if git check-ignore --no-index -q -- "$carrier"; then
    echo "public release carrier is still ignored: $carrier" >&2
    exit 1
  fi

  if ! git ls-files --error-unmatch -- "$carrier" >/dev/null 2>&1; then
    echo "public release carrier is not tracked: $carrier" >&2
    exit 1
  fi

  if ! git cat-file -e "${candidate_sha}:${carrier}"; then
    echo "public release carrier is absent from candidate tree $candidate_sha: $carrier" >&2
    exit 1
  fi
done

archive_entries=$(
  git archive --format=tar "$candidate_sha" -- "${public_release_carriers[@]}" \
    | tar -tf -
)
for carrier in "${public_release_carriers[@]}"; do
  if ! printf '%s\n' "$archive_entries" | grep -Fxq -- "$carrier"; then
    echo "public release carrier is absent from git archive $candidate_sha: $carrier" >&2
    exit 1
  fi
done

printf 'public release carriers validated: sha=%s carriers=%d\n' \
  "$candidate_sha" \
  "${#public_release_carriers[@]}"
