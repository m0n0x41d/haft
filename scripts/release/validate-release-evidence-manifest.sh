#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 5 ]; then
  echo "usage: validate-release-evidence-manifest.sh MANIFEST CANDIDATE_SHA VERSION P13_DIGEST P14_FINAL_DIGEST" >&2
  exit 64
fi

manifest="$1"
candidate_sha="$2"
version="$3"
p13_digest="$4"
p14_final_digest="$5"

if [[ ! "$candidate_sha" =~ ^[0-9a-f]{40}$ ]]; then
  echo "release-evidence candidate must be a full lowercase Git SHA" >&2
  exit 1
fi
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "release-evidence version must be an exact semver without a v prefix" >&2
  exit 1
fi
if [[ ! "$p13_digest" =~ ^sha256:[0-9a-f]{64}$ ]] ||
   [[ ! "$p14_final_digest" =~ ^sha256:[0-9a-f]{64}$ ]]; then
  echo "release-evidence carrier digests must be sha256 digests" >&2
  exit 1
fi

test -f "$manifest"

jq -e \
  --arg sha "$candidate_sha" \
  --arg version "$version" \
  --arg p13 "$p13_digest" \
  --arg p14 "$p14_final_digest" \
  '
  keys == ["candidate_sha", "p13", "p14", "schema", "status", "tag", "version"] and
  .schema == "haft.release.validation-evidence/v1" and
  .status == "passed" and
  .candidate_sha == $sha and
  .version == $version and
  .tag == ("v" + $version) and
  (.p13 | keys) == [
    "artifact_digest", "artifact_id", "artifact_name", "basis_artifact_digest",
    "basis_artifact_id", "basis_artifact_name", "basis_run_id", "basis_workflow",
    "carrier_digest", "carrier_path", "run_id"
  ] and
  (.p14 | keys) == [
    "artifact_digest", "artifact_id", "artifact_name", "bundle_digest",
    "final_carrier_digest", "final_carrier_path", "prepared_carrier_digest",
    "prepared_carrier_path", "producer_workflow", "qualified_executable_digest",
    "release_archives", "run_id", "valid_until"
  ] and
  .p13.carrier_digest == $p13 and
  .p14.final_carrier_digest == $p14 and
  .p13.artifact_name == ("p13-acceptance-" + $sha) and
  .p13.basis_artifact_name == "p13-frozen-basis" and
  .p14.artifact_name == ("p14-final-evidence-" + $sha) and
  .p14.producer_workflow == ".github/workflows/p14-evidence.yml" and
  (.p14.valid_until | fromdateiso8601) > now and
  (.p14.qualified_executable_digest | test("^sha256:[0-9a-f]{64}$")) and
  (.p14.release_archives | map(.name)) == [
    "haft-linux-amd64.tar.gz",
    "haft-linux-arm64.tar.gz",
    "haft-darwin-arm64.tar.gz"
  ] and
  (.p14.release_archives | all(
    (keys == ["digest", "name"]) and
    (.digest | test("^sha256:[0-9a-f]{64}$"))
  )) and
  (.p13.run_id | test("^[0-9]+$")) and
  (.p13.basis_run_id | test("^[0-9]+$")) and
  (.p14.run_id | test("^[0-9]+$")) and
  (.p13.carrier_path | test("^\\.context/p13/[^/]+\\.json$")) and
  (.p14.prepared_carrier_path | test("^\\.context/p14/[^/]+\\.json$")) and
  (.p14.final_carrier_path | test("^\\.context/p14/[^/]+\\.json$")) and
  ([
    .p13.artifact_digest,
    .p13.basis_artifact_digest,
    .p13.carrier_digest,
    .p14.artifact_digest,
    .p14.bundle_digest,
    .p14.prepared_carrier_digest,
    .p14.final_carrier_digest,
    .p14.qualified_executable_digest
  ] | all(test("^sha256:[0-9a-f]{64}$"))) and
  (.p13.artifact_id | type) == "number" and .p13.artifact_id > 0 and
  (.p13.basis_artifact_id | type) == "number" and .p13.basis_artifact_id > 0 and
  (.p14.artifact_id | type) == "number" and .p14.artifact_id > 0
  ' "$manifest" >/dev/null

canonical=$(mktemp "${TMPDIR:-/tmp}/haft-release-evidence.XXXXXX")
trap 'rm -f "$canonical"' EXIT
jq -S . "$manifest" > "$canonical"
cmp -s "$manifest" "$canonical" || {
  echo "release-evidence manifest is not canonical sorted JSON" >&2
  exit 1
}

printf 'release evidence manifest accepted: sha=%s version=%s p13=%s p14=%s\n' \
  "$candidate_sha" "$version" "$p13_digest" "$p14_final_digest"
