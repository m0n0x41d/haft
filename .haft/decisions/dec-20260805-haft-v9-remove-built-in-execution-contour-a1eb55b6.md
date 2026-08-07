---
id: dec-20260805-haft-v9-remove-built-in-execution-contour-a1eb55b6
kind: DecisionRecord
version: 1
status: active
title: Haft v9 removes all built-in agent execution and retains runner-neutral commission governance
mode: deep
valid_until: 2027-02-05
created_at: 2026-08-05T09:52:30Z
updated_at: 2026-08-05T09:52:30Z
---

# Haft v9 removes all built-in agent execution and retains runner-neutral commission governance

## 1. Problem Frame

**Problem statement:** Haft v9 is a governance substrate but still bundles and exposes two built-in coding-agent execution paths, including an Elixir/OTP Open-Sleigh runtime, creating a second product boundary, release burden, and stale compatibility surface.

## 2. Decision

**Selected:** Haft v9 removes all built-in agent execution and retains runner-neutral commission governance

**Selection policy:** Among options that preserve WorkCommission authority, audit history, external-runner interoperability, and safe v8 migration, maximize product coherence and maintenance reduction; when otherwise tied, prefer the reversible boundary that does not introduce an unproven replacement runtime.

**Why selected:** The selected boundary aligns the shipped product with the v8 governance-substrate pivot, removes the bundled BEAM runtime and duplicate agent orchestration, and preserves explicit WorkCommission authority plus external-runner lifecycle interfaces for future integrations.


**Invariants:**
- h-commission remains manual-only and WorkCommission remains an explicit bounded authority record.
- Haft core does not spawn a coding agent, execute a commission, apply a workspace diff, merge, push, or publish.
- Runner-neutral commission fields, lifecycle transitions, RuntimeRun records, and external completion remain supported.
- The installer never deletes ~/.open-sleigh user workspaces, logs, results, or configuration.
- Historical decisions, evidence, v7/v8 documentation, and changelog entries remain historical rather than being rewritten as current facts.
- Commissio and haft-commissio are outside the v9 implementation.

**Spec sections:**
- SS.constraints.runtime.001
- SS.interfaces.runtime.001
- SS.procedural.commission.001
- SS.procedural.runtime.001

## 3. Rationale

**Counterargument:** Extracting Open-Sleigh into a separately versioned legacy runner would preserve more continuity for existing users and reduce migration shock, even though it would delay v9 and retain protocol and maintenance work.

**Selected variant weakest link:** Actual Open-Sleigh and haft run adoption is unknown, so some external users may lose a zero-configuration execution path before a successor integration exists.

**Rejected alternatives:**
| Variant | Verdict | Reason |
|---------|---------|--------|
| Haft v9 removes all built-in agent execution and retains runner-neutral commission governance | **Selected** | The selected boundary aligns the shipped product with the... |
| Keep the bundled execution contour | Rejected | Preserves compatibility but retains the second product, BEAM release matrix, and architecture conflict. |
| Unbundle BEAM but retain haft harness and Open-Sleigh source | Rejected | Reduces archive size but leaves most code, CI, support, and product-boundary complexity. |
| Extract Open-Sleigh as a legacy runner before v9 | Rejected | Improves continuity but delays the release and keeps an unproven runtime recovery burden active. |
| Replace Open-Sleigh with Commissio in v9 | Rejected | Commissio has no implemented or live-qualified runtime contract and cannot be release evidence for v9. |

**Evidence requirements:**
- Focused CLI command-surface and WorkCommission lifecycle tests.
- Installer upgrade test proving exact legacy-runtime cleanup and user-data preservation.
- Archive smoke proving absence of Open-Sleigh/BEAM while core init, FPF Query, and MCP work.
- Updated P13 identity and consolidated exact-candidate qualification after all implementation and documentation edits.
- Installed P14 and manual E2E remain separate later gates.

**Predictions:**
| Claim | Observable | Threshold |
|-------|------------|-----------|
| The v9 public CLI no longer exposes built-in agent execution. | Root help and command registration. | Neither run nor harness is registered; commission remains registered. |
| The v9 release archive has no Open-Sleigh or BEAM payload. | Archive listing and smoke validation. | Zero Open-Sleigh, ERTS, Elixir, or BEAM runtime entries; core CLI, init, FPF Query, and MCP smoke still pass. |
| A v9 upgrade removes only the exact Haft-managed legacy runtime. | Installer migration test with independent sentinels. | ~/.haft/runtimes/open-sleigh/current is absent while ~/.open-sleigh and haft-embed sentinels remain unchanged. |
| External execution remains representable without Haft owning an executor. | Focused WorkCommission lifecycle test. | create, list-runnable, claim, preflight/start, event, and complete-external reach the expected terminal state. |

## 4. Consequences

**Rollback plan:**
Triggers:
- Runner-neutral WorkCommission lifecycle cannot complete an externally performed commission after the removal.
- The exact v9 candidate cannot pass core CLI, MCP, installer, archive, or P13 acceptance without a deleted execution dependency.
Steps:
1. Stop publication and keep v9 unreleased.
2. Restore the removed execution contour only on a dedicated compatibility branch from the exact predecessor commit or v8.1 tag.
3. Keep project ledgers and WorkCommission records unchanged; do not downgrade a migrated v9 ledger.
Blast radius: Haft CLI execution commands, release packaging, installer-managed Open-Sleigh runtime, current execution specifications, and documentation; artifact storage and runner-neutral commission records remain unchanged.

**Refresh triggers:**
- External users provide evidence that the removed commands are load-bearing and cannot migrate through v8.1 or an external runner.
- A runner-neutral lifecycle gap prevents Commissio or another executor from consuming authorized WorkCommissions.
- A future release deliberately reintroduces an official execution distribution.

**Affected files:** .github/release-notes/9.0.0.md, .github/workflows, .haft/specs/software-system.md, .haft/specs/term-map.md, MIGRATION-v8.md, README.md, Taskfile.yaml, docs, install.sh, internal/cli/commission.go, internal/cli/harness.go, internal/cli/run.go, internal/p13acceptance, open-sleigh

<!-- haft:structured_data
{
  "authority_provenance": "host_routed_operator_request",
  "choice_result": {
    "choice_rule": "Among options that preserve WorkCommission authority, audit history, external-runner interoperability, and safe v8 migration, maximize product coherence and maintenance reduction; when otherwise tied, prefer the reversible boundary that does not introduce an unproven replacement runtime.",
    "comparison_basis": [
      "Product coherence with the v8 governance-substrate pivot",
      "Maintenance, CI, installer, archive, and security burden",
      "Migration safety and preservation of authority/evidence records",
      "Current implementation and live-evidence readiness"
    ],
    "next_move": "choose_now",
    "option_set": [
      "Haft v9 removes all built-in agent execution and retains runner-neutral commission governance",
      "Keep the bundled execution contour",
      "Unbundle BEAM but retain haft harness and Open-Sleigh source",
      "Extract Open-Sleigh as a legacy runner before v9",
      "Replace Open-Sleigh with Commissio in v9"
    ],
    "reason": "The selected boundary aligns the shipped product with the v8 governance-substrate pivot, removes the bundled BEAM runtime and duplicate agent orchestration, and preserves explicit WorkCommission authority plus external-runner lifecycle interfaces for future integrations.",
    "reopen_condition": "Reopen only if external-runner lifecycle fails or concrete user evidence makes a compatibility distribution necessary.",
    "reversibility": "Source and packaging removal is recoverable from Git history or the v8.1 tag before publication; v9 ledger migration remains a separate rollback boundary.",
    "subject_ref": "Haft v9 product boundary",
    "variant_ref": "Haft v9 removes all built-in agent execution and retains runner-neutral commission governance"
  },
  "claims": [
    {
      "claim": "The v9 public CLI no longer exposes built-in agent execution.",
      "id": "claim-001",
      "observable": "Root help and command registration.",
      "probability": 0.98,
      "status": "unverified",
      "threshold": "Neither run nor harness is registered; commission remains registered."
    },
    {
      "claim": "The v9 release archive has no Open-Sleigh or BEAM payload.",
      "id": "claim-002",
      "observable": "Archive listing and smoke validation.",
      "probability": 0.95,
      "status": "unverified",
      "threshold": "Zero Open-Sleigh, ERTS, Elixir, or BEAM runtime entries; core CLI, init, FPF Query, and MCP smoke still pass."
    },
    {
      "claim": "A v9 upgrade removes only the exact Haft-managed legacy runtime.",
      "id": "claim-003",
      "observable": "Installer migration test with independent sentinels.",
      "probability": 0.95,
      "status": "unverified",
      "threshold": "~/.haft/runtimes/open-sleigh/current is absent while ~/.open-sleigh and haft-embed sentinels remain unchanged."
    },
    {
      "claim": "External execution remains representable without Haft owning an executor.",
      "id": "claim-004",
      "observable": "Focused WorkCommission lifecycle test.",
      "probability": 0.95,
      "status": "unverified",
      "threshold": "create, list-runnable, claim, preflight/start, event, and complete-external reach the expected terminal state."
    }
  ],
  "counterargument": "Extracting Open-Sleigh into a separately versioned legacy runner would preserve more continuity for existing users and reduce migration shock, even though it would delay v9 and retain protocol and maintenance work.",
  "decision_subject_ref": "Haft v9 product boundary",
  "drift_watch_targets": [
    {
      "target_ref": "api_contract:haft/cli-execution-surface",
      "trigger": "schema_or_behavior_changed"
    },
    {
      "target_ref": "api_contract:haft/commission-lifecycle",
      "trigger": "schema_or_behavior_changed"
    }
  ],
  "evidence_requirements": [
    "Focused CLI command-surface and WorkCommission lifecycle tests.",
    "Installer upgrade test proving exact legacy-runtime cleanup and user-data preservation.",
    "Archive smoke proving absence of Open-Sleigh/BEAM while core init, FPF Query, and MCP work.",
    "Updated P13 identity and consolidated exact-candidate qualification after all implementation and documentation edits.",
    "Installed P14 and manual E2E remain separate later gates."
  ],
  "governance_targets": [
    {
      "kind": "spec_section",
      "ref": "spec_section:SS.procedural.commission.001"
    },
    {
      "kind": "spec_section",
      "ref": "spec_section:SS.interfaces.runtime.001"
    },
    {
      "kind": "spec_section",
      "ref": "spec_section:SS.constraints.runtime.001"
    },
    {
      "kind": "spec_section",
      "ref": "spec_section:SS.procedural.runtime.001"
    },
    {
      "kind": "api_contract",
      "ref": "api_contract:haft/cli-execution-surface"
    },
    {
      "kind": "api_contract",
      "ref": "api_contract:haft/commission-lifecycle"
    }
  ],
  "implementation_footprint": {},
  "invariants": [
    "h-commission remains manual-only and WorkCommission remains an explicit bounded authority record.",
    "Haft core does not spawn a coding agent, execute a commission, apply a workspace diff, merge, push, or publish.",
    "Runner-neutral commission fields, lifecycle transitions, RuntimeRun records, and external completion remain supported.",
    "The installer never deletes ~/.open-sleigh user workspaces, logs, results, or configuration.",
    "Historical decisions, evidence, v7/v8 documentation, and changelog entries remain historical rather than being rewritten as current facts.",
    "Commissio and haft-commissio are outside the v9 implementation."
  ],
  "problem_statement": "Haft v9 is a governance substrate but still bundles and exposes two built-in coding-agent execution paths, including an Elixir/OTP Open-Sleigh runtime, creating a second product boundary, release burden, and stale compatibility surface.",
  "refresh_triggers": [
    "External users provide evidence that the removed commands are load-bearing and cannot migrate through v8.1 or an external runner.",
    "A runner-neutral lifecycle gap prevents Commissio or another executor from consuming authorized WorkCommissions.",
    "A future release deliberately reintroduces an official execution distribution."
  ],
  "rollback_blast_radius": "Haft CLI execution commands, release packaging, installer-managed Open-Sleigh runtime, current execution specifications, and documentation; artifact storage and runner-neutral commission records remain unchanged.",
  "rollback_steps": [
    "Stop publication and keep v9 unreleased.",
    "Restore the removed execution contour only on a dedicated compatibility branch from the exact predecessor commit or v8.1 tag.",
    "Keep project ledgers and WorkCommission records unchanged; do not downgrade a migrated v9 ledger."
  ],
  "rollback_triggers": [
    "Runner-neutral WorkCommission lifecycle cannot complete an externally performed commission after the removal.",
    "The exact v9 candidate cannot pass core CLI, MCP, installer, archive, or P13 acceptance without a deleted execution dependency."
  ],
  "section_refs": [
    "SS.constraints.runtime.001",
    "SS.interfaces.runtime.001",
    "SS.procedural.commission.001",
    "SS.procedural.runtime.001"
  ],
  "selected_title": "Haft v9 removes all built-in agent execution and retains runner-neutral commission governance",
  "selection_policy": "Among options that preserve WorkCommission authority, audit history, external-runner interoperability, and safe v8 migration, maximize product coherence and maintenance reduction; when otherwise tied, prefer the reversible boundary that does not introduce an unproven replacement runtime.",
  "spec_binding_preflight": {
    "record_kind": "spec_binding_preflight",
    "schema_version": 1,
    "selected_section_refs": [
      "SS.constraints.runtime.001",
      "SS.interfaces.runtime.001",
      "SS.procedural.commission.001",
      "SS.procedural.runtime.001"
    ],
    "state": "provided_refs_valid",
    "status_debt": {
      "severity": "none"
    }
  },
  "spec_binding_preflight_required": true,
  "task_context": "haft-v9-remove-built-in-execution-contour",
  "weakest_link": "Actual Open-Sleigh and haft run adoption is unknown, so some external users may lose a zero-configuration execution path before a successor integration exists.",
  "why_not_others": [
    {
      "reason": "Preserves compatibility but retains the second product, BEAM release matrix, and architecture conflict.",
      "variant": "Keep the bundled execution contour"
    },
    {
      "reason": "Reduces archive size but leaves most code, CI, support, and product-boundary complexity.",
      "variant": "Unbundle BEAM but retain haft harness and Open-Sleigh source"
    },
    {
      "reason": "Improves continuity but delays the release and keeps an unproven runtime recovery burden active.",
      "variant": "Extract Open-Sleigh as a legacy runner before v9"
    },
    {
      "reason": "Commissio has no implemented or live-qualified runtime contract and cannot be release evidence for v9.",
      "variant": "Replace Open-Sleigh with Commissio in v9"
    }
  ],
  "why_selected": "The selected boundary aligns the shipped product with the v8 governance-substrate pivot, removes the bundled BEAM runtime and duplicate agent orchestration, and preserves explicit WorkCommission authority plus external-runner lifecycle interfaces for future integrations."
}
haft:end -->
