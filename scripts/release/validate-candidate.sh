#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: validate-candidate.sh VERSION CANDIDATE_SHA MAIN_SHA" >&2
  exit 64
fi

version="$1"
candidate_sha="$2"
main_sha="$3"

if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "release version must be exact semver without a v prefix: $version" >&2
  exit 1
fi

if [[ ! "$candidate_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "candidate SHA must be a full lowercase Git SHA: $candidate_sha" >&2
  exit 1
fi

if [[ ! "$main_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "main SHA must be a full lowercase Git SHA: $main_sha" >&2
  exit 1
fi

if [ "$candidate_sha" != "$main_sha" ]; then
  echo "release candidate $candidate_sha is not current origin/main $main_sha" >&2
  exit 1
fi

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)
bash "$script_dir/validate-public-release-carriers.sh" "$candidate_sha"

printf 'release candidate accepted: version=%s sha=%s\n' "$version" "$candidate_sha"
