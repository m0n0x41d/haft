#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: build-native-candidate.sh VERSION OUTPUT" >&2
  exit 64
fi

version="$1"
output="$2"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "native candidate version must be exact semver without a v prefix" >&2
  exit 1
fi

output=$(python3 - "$output" <<'PY'
import os
import sys
print(os.path.abspath(sys.argv[1]))
PY
)
repository_root=$(git rev-parse --show-toplevel)
cd "$repository_root"
candidate_sha=$(git rev-parse HEAD)
if [[ ! "$candidate_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "native candidate requires one full Git SHA" >&2
  exit 1
fi
tree_state=$(git status --porcelain=v1 --untracked-files=all) || {
  echo "unable to inspect the worktree; refusing to build a native candidate" >&2
  exit 1
}
if [ -n "$tree_state" ]; then
  echo "native candidate build requires a clean checkout" >&2
  exit 1
fi

host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
case "$host_os-$host_arch" in
  linux-amd64|linux-arm64|darwin-arm64) ;;
  *)
    echo "unsupported native release host: $host_os-$host_arch" >&2
    exit 1
    ;;
esac

mkdir -p "$(dirname "$output")"
if [ -e "$output" ]; then
  echo "native candidate output already exists: $output" >&2
  exit 1
fi

source_time=$(git show -s --format=%cI HEAD)
ldflags="-s -w -X github.com/m0n0x41d/haft/internal/cli.Version=$version -X github.com/m0n0x41d/haft/internal/cli.Commit=$candidate_sha -X github.com/m0n0x41d/haft/internal/cli.BuildDate=$source_time"
CGO_ENABLED=1 GOOS="$host_os" GOARCH="$host_arch" \
  go build -buildvcs=true -trimpath -ldflags "$ldflags" -o "$output" ./cmd/haft/
chmod 0755 "$output"

version_output=$(mktemp "${TMPDIR:-/tmp}/haft-native-version.XXXXXX")
trap 'rm -f -- "$version_output"' EXIT
"$output" version > "$version_output"
test "$(wc -l < "$version_output" | tr -d ' ')" = 4
grep -Fx "haft $version" "$version_output" >/dev/null
grep -Fx "  commit:  $candidate_sha" "$version_output" >/dev/null
grep -Eq '^  built:   .+$' "$version_output"
grep -Eq '^  source:  .+$' "$version_output"
if grep -Fq '  modified: true' "$version_output"; then
  echo "native candidate reports modified source" >&2
  exit 1
fi

printf 'native candidate built: sha=%s version=%s target=%s-%s output=%s\n' \
  "$candidate_sha" "$version" "$host_os" "$host_arch" "$output"
