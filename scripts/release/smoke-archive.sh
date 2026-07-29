#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: smoke-archive.sh ARCHIVE VERSION SHORT_SHA" >&2
  exit 64
fi

archive="$1"
version="$2"
short_sha="$3"
extract_seed=$(mktemp -d)
extract_dir=$(cd "$extract_seed" && pwd -P)
trap 'rm -rf -- "$extract_dir"' EXIT

tar -xzf "$archive" -C "$extract_dir"
mkdir -p "$extract_dir/home"
mkdir -p "$extract_dir/project"

test -f "$extract_dir/README.md"
test -f "$extract_dir/LICENSE"
test -f "$extract_dir/CHANGELOG.md"

version_output=$("$extract_dir/haft" version)
printf '%s\n' "$version_output" | grep -F "haft $version"
printf '%s\n' "$version_output" | grep -F "commit:  $short_sha"

fpf_query_output=$(
  HOME="$extract_dir/home" \
    HAFT_EMBED_BIN=/definitely/missing/haft-embed \
    "$extract_dir/haft" fpf query "weak link" --max-total-candidates 1 --json
)
printf '%s\n' "$fpf_query_output" | grep -Fq '"kind":"candidate_set"'
printf '%s\n' "$fpf_query_output" \
  | grep -Eq '"groups":\[\{"source_role":"(practical_use_card|toc_row)"'

(
  cd "$extract_dir/project"
  HOME="$extract_dir/home" \
    HAFT_EMBED_BIN=/definitely/missing/haft-embed \
    "$extract_dir/haft" init --codex --local
)
test -f "$extract_dir/project/.haft/config.yaml"
test -f "$extract_dir/project/.haft/project.yaml"
test ! -e "$extract_dir/project/.haft/specs/target-system.md"
test ! -e "$extract_dir/project/.haft/specs/software-system.md"
test ! -e "$extract_dir/project/.haft/specs/term-map.md"
test -f "$extract_dir/project/.codex/config.toml"
test -f "$extract_dir/project/AGENTS.md"
grep -Fq '<!-- haft:start -->' "$extract_dir/project/AGENTS.md"
grep -Fq '<!-- haft:end -->' "$extract_dir/project/AGENTS.md"
grep -Fq '[mcp_servers.haft]' "$extract_dir/project/.codex/config.toml"
test -f "$extract_dir/project/.agents/skills/h-reason/SKILL.md"
test -f "$extract_dir/project/.agents/skills/h-decide/SKILL.md"
test "$(
  find "$extract_dir/project/.agents/skills" \
    -mindepth 2 \
    -maxdepth 2 \
    -name SKILL.md \
    | wc -l \
    | tr -d ' '
)" = "12"

mcp_output=$(
  cd "$extract_dir/project"
  printf '%s\n' \
    '{"jsonrpc":"2.0","method":"initialize","id":1,"params":{}}' \
    '{"jsonrpc":"2.0","method":"tools/call","id":2,"params":{"name":"haft_query","arguments":{"action":"status"}}}' \
    | HOME="$extract_dir/home" \
      HAFT_EMBED_BIN=/definitely/missing/haft-embed \
      "$extract_dir/haft" serve 2>/dev/null
)
printf '%s\n' "$mcp_output" | grep -Fq '"protocolVersion"'
printf '%s\n' "$mcp_output" | grep -Fq '"serverInfo":{"name":"haft","version":"'"$version"'"}'
printf '%s\n' "$mcp_output" | grep -Fq 'Haft Status'
if printf '%s\n' "$mcp_output" | grep -F '"isError":true'; then
  echo "fresh-project MCP status returned isError" >&2
  exit 1
fi

migration_help=$(
  HOME="$extract_dir/home" "$extract_dir/haft" spec migrate --help
)
printf '%s\n' "$migration_help" | grep -Fq -- '--json'
for removed_flag in --to --packet --apply --admit-review --recover; do
  if printf '%s\n' "$migration_help" | grep -Fq -- "$removed_flag"; then
    echo "removed spec-migrate flag remains public: $removed_flag" >&2
    exit 1
  fi
done

fresh_haft_before="$extract_dir/fresh-haft-before"
cp -R "$extract_dir/project/.haft" "$fresh_haft_before"
if migration_absence_output=$(
  cd "$extract_dir/project"
  HOME="$extract_dir/home" "$extract_dir/haft" spec migrate --json 2>&1
); then
  echo "fresh project unexpectedly had a prepared specification migration" >&2
  exit 1
fi
printf '%s\n' "$migration_absence_output" \
  | grep -Fq 'migration_candidate_not_prepared'
diff -qr "$fresh_haft_before" "$extract_dir/project/.haft"

open_sleigh="$extract_dir/runtimes/open-sleigh/bin/open_sleigh"
test -x "$open_sleigh"
runtime_output=$(
  cd "$extract_dir"
  ./runtimes/open-sleigh/bin/open_sleigh eval 'IO.write("open_sleigh_runtime_ok")'
)
printf '%s\n' "$runtime_output" | grep -F 'open_sleigh_runtime_ok'

haft_embed="$extract_dir/runtimes/haft-embed/bin/haft-embed"
if [ -x "$haft_embed" ]; then
  "$haft_embed" --version | grep -F 'haft-embed'
fi

printf 'archive smoke passed: archive=%s version=%s commit=%s\n' "$archive" "$version" "$short_sha"
