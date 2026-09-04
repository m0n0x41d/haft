#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 8 ]; then
  echo "usage: assemble-p14-release-evidence.sh PROJECT_ROOT P13_PATH PREPARED_PATH FINAL_PATH NATIVE_ROOT OUTPUT_ROOT CANDIDATE_SHA VERSION" >&2
  exit 64
fi

project_root="$1"
p13_path="$2"
prepared_path="$3"
final_path="$4"
native_root="$5"
output_root="$6"
candidate_sha="$7"
version="$8"

if [[ ! "$candidate_sha" =~ ^[0-9a-f]{40}$ ]] ||
   [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "P14 release assembly requires exact candidate SHA and version" >&2
  exit 1
fi
if [ "$(cd "$project_root" && pwd -P)" != "$project_root" ]; then
  echo "P14 release project root must be one physical absolute path" >&2
  exit 1
fi
if [[ ! "$p13_path" =~ ^\.context/p13/[A-Za-z0-9][A-Za-z0-9._-]*\.json$ ]] ||
   [[ ! "$prepared_path" =~ ^\.context/p14/[A-Za-z0-9][A-Za-z0-9._-]*\.json$ ]] ||
   [[ ! "$final_path" =~ ^\.context/p14/[A-Za-z0-9][A-Za-z0-9._-]*\.json$ ]]; then
  echo "P14 release carrier path is outside its closed namespace" >&2
  exit 1
fi
if [ -e "$output_root" ]; then
  echo "P14 release output already exists: $output_root" >&2
  exit 1
fi

targets=(linux-amd64 linux-arm64 darwin-arm64)
for target in "${targets[@]}"; do
  test -f "$native_root/haft-${target}.tar.gz"
  test -f "$native_root/haft-${target}.version-receipt.json"
done
native_symlink=$(find "$native_root" -type l -print -quit) || {
  echo "P14 native input scan failed" >&2
  exit 1
}
if [ -n "$native_symlink" ]; then
  echo "P14 native input contains a symlink" >&2
  exit 1
fi
test "$(find "$native_root" -maxdepth 1 -type f | wc -l | tr -d ' ')" = 6

mkdir -p "$output_root/repository/.context/p13"
mkdir -p "$output_root/repository/.context/p14"
mkdir -p "$output_root/release-artifacts"
for carrier_path in "$p13_path" "$prepared_path" "$final_path"; do
  if [ -L "$project_root/$carrier_path" ] || [ ! -f "$project_root/$carrier_path" ]; then
    echo "P14 release carrier must be a regular file: $carrier_path" >&2
    exit 1
  fi
done
cp "$project_root/$p13_path" "$output_root/repository/$p13_path"
cp "$project_root/$prepared_path" "$output_root/repository/$prepared_path"
cp "$project_root/$final_path" "$output_root/repository/$final_path"
for target in "${targets[@]}"; do
  cp "$native_root/haft-${target}.tar.gz" "$output_root/release-artifacts/haft-${target}.tar.gz"
done

digest_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print "sha256:" $1}'
  else
    shasum -a 256 "$1" | awk '{print "sha256:" $1}'
  fi
}
p13_digest=$(digest_file "$output_root/repository/$p13_path")
prepared_digest=$(digest_file "$output_root/repository/$prepared_path")
final_digest=$(digest_file "$output_root/repository/$final_path")

for target in "${targets[@]}"; do
  receipt="$native_root/haft-${target}.version-receipt.json"
  archive="$output_root/release-artifacts/haft-${target}.tar.gz"
  test "$(jq -r '.archive_digest' "$receipt")" = "$(digest_file "$archive")"
  jq -e \
    --arg sha "$candidate_sha" \
    --arg version "$version" \
    '.schema == "haft.p14.native-version-receipt/v1" and
     .candidate_sha == $sha and .version == $version and
     (keys == ["archive_digest", "archive_name", "candidate_sha", "executable_digest", "goarch", "goos", "schema", "version", "version_output_base64", "version_output_digest"])' \
    "$receipt" >/dev/null
done

qualified_digest=$(jq -er '.executable_digest' "$native_root/haft-darwin-arm64.version-receipt.json")
test "$(jq -er '.preparation.frozen_basis.candidate.executable_digest' "$project_root/$prepared_path")" = "$qualified_digest"
test "$(jq -er '.preparation.p13_evidence.carrier_path' "$project_root/$prepared_path")" = "$p13_path"
test "$(jq -er '.preparation.p13_evidence.carrier_digest' "$project_root/$prepared_path")" = "$p13_digest"

valid_until=$(python3 - "$project_root/$final_path" <<'PY'
import datetime
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    carrier = json.load(source)
observed = []
for scenario in carrier["observation"]["scenario_observations"]:
    for surface in scenario["surface_observations"]:
        raw = surface["observed_at"]
        observed.append(datetime.datetime.fromisoformat(raw.replace("Z", "+00:00")))
if not observed:
    raise SystemExit("P14 final carrier has no observed surface time")
oldest = min(observed).replace(microsecond=0)
valid_until = oldest + datetime.timedelta(hours=24)
print(valid_until.astimezone(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"))
PY
)

bundle="$output_root/release-evidence.json"
jq -n \
  --arg sha "$candidate_sha" \
  --arg p13_path "$p13_path" \
  --arg p13_digest "$p13_digest" \
  --arg prepared_path "$prepared_path" \
  --arg prepared_digest "$prepared_digest" \
  --arg final_path "$final_path" \
  --arg final_digest "$final_digest" \
  --arg valid_until "$valid_until" \
  --arg qualified_digest "$qualified_digest" \
  --slurpfile linux_amd64 "$native_root/haft-linux-amd64.version-receipt.json" \
  --slurpfile linux_arm64 "$native_root/haft-linux-arm64.version-receipt.json" \
  --slurpfile darwin_arm64 "$native_root/haft-darwin-arm64.version-receipt.json" \
  '{
    schema: "haft.p14.release-evidence-bundle/v2",
    status: "passed",
    candidate_sha: $sha,
    p13_carrier_path: $p13_path,
    p13_carrier_digest: $p13_digest,
    prepared_carrier_path: $prepared_path,
    prepared_carrier_digest: $prepared_digest,
    final_carrier_path: $final_path,
    final_carrier_digest: $final_digest,
    valid_until: $valid_until,
    qualified_archive: "haft-darwin-arm64.tar.gz",
    qualified_member: "haft",
    qualified_executable_digest: $qualified_digest,
    release_archives: [
      {name: "haft-linux-amd64.tar.gz", path: "release-artifacts/haft-linux-amd64.tar.gz", digest: $linux_amd64[0].archive_digest, goos: "linux", goarch: "amd64"},
      {name: "haft-linux-arm64.tar.gz", path: "release-artifacts/haft-linux-arm64.tar.gz", digest: $linux_arm64[0].archive_digest, goos: "linux", goarch: "arm64"},
      {name: "haft-darwin-arm64.tar.gz", path: "release-artifacts/haft-darwin-arm64.tar.gz", digest: $darwin_arm64[0].archive_digest, goos: "darwin", goarch: "arm64"}
    ],
    native_version_receipts: [$linux_amd64[0], $linux_arm64[0], $darwin_arm64[0]]
  }' > "$bundle"

test "$(find "$output_root" -type f | wc -l | tr -d ' ')" = 7
printf 'P14 release evidence assembled: bundle=%s p13=%s final=%s valid_until=%s\n' \
  "$bundle" "$p13_digest" "$final_digest" "$valid_until"
