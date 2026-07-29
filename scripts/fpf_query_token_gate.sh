#!/usr/bin/env bash

set -euo pipefail

token_gate_script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
token_gate_repo_root="$(cd -- "$token_gate_script_dir/.." && pwd -P)"
token_gate_python_override="${HAFT_QUERY_TOKEN_GATE_BOOTSTRAP_PYTHON:-}"
token_gate_bootstrap_python=""
token_gate_mode="run"

case "$#:${1:-}" in
  "0:") ;;
  "1:--print-bootstrap-python") token_gate_mode="print-bootstrap-python" ;;
  *)
    echo >&2 "usage: scripts/fpf_query_token_gate.sh [--print-bootstrap-python]"
    exit 2
    ;;
esac

resolve_supported_token_gate_python() {
  local token_gate_python_candidate="$1"

  env -u PYTHONHOME -u PYTHONPATH \
    "$token_gate_python_candidate" -c '
import os
import platform
import sys

supported = (
    platform.python_implementation() == "CPython"
    and sys.version_info.major == 3
    and 10 <= sys.version_info.minor <= 13
)
if not supported:
    raise SystemExit(1)
print(os.path.realpath(sys.executable))
'
}

if [[ -n "$token_gate_python_override" ]]; then
  token_gate_bootstrap_python="$(
    resolve_supported_token_gate_python "$token_gate_python_override"
  )" || {
    echo >&2 \
      "FPF Query token gate: HAFT_QUERY_TOKEN_GATE_BOOTSTRAP_PYTHON must resolve to CPython 3.10-3.13"
    exit 1
  }
fi

if [[ -z "$token_gate_bootstrap_python" ]]; then
  for token_gate_python_candidate in python3.13 python3.12 python3.11 python3.10; do
    token_gate_resolved_python="$(
      resolve_supported_token_gate_python "$token_gate_python_candidate" 2>/dev/null
    )" || continue
    token_gate_bootstrap_python="$token_gate_resolved_python"
    break
  done
fi

if [[ -z "$token_gate_bootstrap_python" ]]; then
  echo >&2 \
    "FPF Query token gate: no CPython 3.10-3.13 runtime found; set HAFT_QUERY_TOKEN_GATE_BOOTSTRAP_PYTHON"
  exit 1
fi

env -u PYTHONHOME -u PYTHONPATH \
  "$token_gate_bootstrap_python" -c '
import platform
import sys

supported = (
    platform.python_implementation() == "CPython"
    and sys.version_info.major == 3
    and 10 <= sys.version_info.minor <= 13
)
if not supported:
    raise SystemExit(
        "FPF Query token gate requires CPython 3.10-3.13; got "
        + platform.python_implementation()
        + " "
        + platform.python_version()
    )
'

if [[ "$token_gate_mode" == "print-bootstrap-python" ]]; then
  printf '%s\n' "$token_gate_bootstrap_python"
  exit 0
fi

token_gate_env="$(mktemp -d "${TMPDIR:-/tmp}/haft-query-token-gate.XXXXXX")"

cleanup_token_gate_env() {
  rm -rf -- "$token_gate_env"
}

trap cleanup_token_gate_env EXIT HUP INT TERM

env -u PYTHONHOME -u PYTHONPATH \
  "$token_gate_bootstrap_python" -m venv "$token_gate_env"

echo >&2 \
  "FPF Query token gate: a cold run needs package-index and o200k_base asset network access; package wheels and the BPE asset are SHA-256 verified."

env -u PYTHONHOME -u PYTHONPATH \
  "$token_gate_env/bin/python" \
  -m pip \
  --isolated \
  install \
  --disable-pip-version-check \
  --quiet \
  --require-hashes \
  --only-binary=:all: \
  --index-url=https://pypi.org/simple \
  --requirement "$token_gate_repo_root/scripts/fpf_query_token_count.requirements.txt"

cd -- "$token_gate_repo_root"

run_exact_token_gate_test() {
  local token_gate_package="$1"
  local token_gate_test_name="$2"
  local token_gate_log="$token_gate_env/${token_gate_test_name}.log"

  env -u PYTHONHOME -u PYTHONPATH \
    HAFT_QUERY_TOKEN_GATE_PYTHON="$token_gate_env/bin/python" \
    go test \
    -tags=query_token_gate \
    -count=1 \
    -v \
    "$token_gate_package" \
    -run "^${token_gate_test_name}$" \
    | tee "$token_gate_log"

  if grep -Fq '[no tests to run]' "$token_gate_log"; then
    echo >&2 "FPF Query token gate anchor disappeared: $token_gate_package/$token_gate_test_name"
    return 1
  fi

  if ! grep -Fq -- "--- PASS: ${token_gate_test_name} " "$token_gate_log"; then
    echo >&2 "FPF Query token gate did not observe exact PASS anchor: $token_gate_package/$token_gate_test_name"
    return 1
  fi
}

run_exact_token_gate_test \
  ./internal/cli \
  TestFPFQueryWorkingViewEmbeddedO200kAcceptance

run_exact_token_gate_test \
  ./internal/fpf \
  TestFPFQueryWorkingViewSyntheticTokenCalculus
