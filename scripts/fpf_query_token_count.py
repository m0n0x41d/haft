#!/usr/bin/env python3
"""Count compact FPF Query JSON with one pinned o200k_base tokenizer.

This helper is intentionally narrower than a general token-analysis tool. It
accepts a batch emitted by the Go acceptance test, verifies the exact tokenizer
edition and a calibration vector, and returns only token counts. The official
o200k_base asset hash below is the hash enforced by tiktoken 0.9.0 when loading
the encoding.
"""

from __future__ import annotations

import importlib.metadata
import json
import platform
import sys
from typing import Any


TOKENIZER_DISTRIBUTION = "tiktoken"
TOKENIZER_VERSION = "0.9.0"
ENCODING_NAME = "o200k_base"
ENCODING_ASSET_SHA256 = "446a9538cb6c348e3516120d7c08b09f57c36495e2acfffe59a5bf8b0cfb1a2d"
CALIBRATION_TEXT = '{"calibration":"Haft FPF typed memory — Привет, 世界","mode":"concern"}'
CALIBRATION_TOKENS = 21
PYTHON_IMPLEMENTATION = "CPython"
PYTHON_MINIMUM = (3, 10)
PYTHON_MAXIMUM = (3, 13)


def fail(message: str) -> int:
    sys.stderr.write(f"FPF Query token gate: {message}\n")
    return 2


def validate_python_runtime() -> int:
    implementation = platform.python_implementation()
    version = sys.version_info[:2]
    if implementation != PYTHON_IMPLEMENTATION:
        return fail(
            f"Python implementation {implementation!r} is unsupported; "
            f"{PYTHON_IMPLEMENTATION} is required"
        )
    if version < PYTHON_MINIMUM or version > PYTHON_MAXIMUM:
        return fail(
            f"Python {platform.python_version()} is unsupported; "
            "CPython 3.10 through 3.13 is required"
        )
    return 0


def load_pinned_encoding() -> tuple[Any | None, int]:
    try:
        installed_version = importlib.metadata.version(TOKENIZER_DISTRIBUTION)
    except importlib.metadata.PackageNotFoundError:
        return None, fail(
            "tiktoken is unavailable; run the Taskfile gate, which installs "
            "scripts/fpf_query_token_count.requirements.txt in an isolated environment"
        )

    if installed_version != TOKENIZER_VERSION:
        return None, fail(
            f"tiktoken {installed_version} is installed; exact version "
            f"{TOKENIZER_VERSION} is required"
        )

    import tiktoken

    encoding = tiktoken.get_encoding(ENCODING_NAME)
    calibration_ids = encoding.encode(CALIBRATION_TEXT, disallowed_special=())
    calibration_count = len(calibration_ids)
    if calibration_count != CALIBRATION_TOKENS:
        return None, fail(
            f"{ENCODING_NAME} calibration produced {calibration_count} tokens; "
            f"expected {CALIBRATION_TOKENS}"
        )
    return encoding, 0


def parse_batch() -> tuple[list[dict[str, str]] | None, int]:
    try:
        payload = json.load(sys.stdin)
    except (json.JSONDecodeError, UnicodeDecodeError) as error:
        return None, fail(f"input is not valid UTF-8 JSON: {error}")

    if not isinstance(payload, dict) or payload.get("schema") != "haft.fpf-query-token-gate/v1":
        return None, fail("input schema must be haft.fpf-query-token-gate/v1")

    cases = payload.get("cases")
    if not isinstance(cases, list) or not cases:
        return None, fail("input cases must be a non-empty array")

    parsed: list[dict[str, str]] = []
    seen_ids: set[str] = set()
    for item in cases:
        if not isinstance(item, dict):
            return None, fail("each input case must be an object")
        case_id = item.get("case_id")
        canonical_json = item.get("canonical_json")
        working_json = item.get("working_json")
        values = (case_id, canonical_json, working_json)
        if not all(isinstance(value, str) and value for value in values):
            return None, fail("each case requires non-empty case_id, canonical_json, and working_json")
        if case_id in seen_ids:
            return None, fail(f"duplicate case_id {case_id!r}")
        for field_name, encoded in (
            ("canonical_json", canonical_json),
            ("working_json", working_json),
        ):
            try:
                decoded = json.loads(encoded)
            except json.JSONDecodeError as error:
                return None, fail(f"{case_id}.{field_name} is not valid JSON: {error}")
            if not isinstance(decoded, dict):
                return None, fail(f"{case_id}.{field_name} must encode one JSON object")
        seen_ids.add(case_id)
        parsed.append(
            {
                "case_id": case_id,
                "canonical_json": canonical_json,
                "working_json": working_json,
            }
        )
    return parsed, 0


def count_batch(encoding: Any, cases: list[dict[str, str]]) -> list[dict[str, int | str]]:
    counts: list[dict[str, int | str]] = []
    for item in cases:
        canonical_ids = encoding.encode(item["canonical_json"], disallowed_special=())
        working_ids = encoding.encode(item["working_json"], disallowed_special=())
        counts.append(
            {
                "case_id": item["case_id"],
                "canonical_tokens": len(canonical_ids),
                "working_tokens": len(working_ids),
            }
        )
    return counts


def main() -> int:
    status = validate_python_runtime()
    if status != 0:
        return status

    encoding, status = load_pinned_encoding()
    if status != 0:
        return status

    cases, status = parse_batch()
    if status != 0:
        return status

    counts = count_batch(encoding, cases)
    result = {
        "schema": "haft.fpf-query-token-gate-result/v1",
        "encoding": ENCODING_NAME,
        "tokenizer_distribution": TOKENIZER_DISTRIBUTION,
        "tokenizer_version": TOKENIZER_VERSION,
        "encoding_asset_sha256": ENCODING_ASSET_SHA256,
        "calibration_tokens": CALIBRATION_TOKENS,
        "python_implementation": platform.python_implementation(),
        "python_version": platform.python_version(),
        "counts": counts,
    }
    json.dump(result, sys.stdout, ensure_ascii=False, separators=(",", ":"))
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
