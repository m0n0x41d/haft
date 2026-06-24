# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Fixed

- **Decision reconciliation selection summary counters.** `haft decision
  reconcile selection-draft` now reports explicit
  `review_required_candidates`, `apply_ready_candidates`, and `template_items`
  counters so operator-review work cannot be mistaken for apply-ready
  selections.
- **Decision reconciliation selection template boundary.** `haft decision
  reconcile selection-draft --json` now emits
  `selection_document_template_boundary`, making explicit that template items
  are emitted review candidates, not selected or apply-ready candidates.
- **Decision reconciliation apply help boundary.** `haft decision reconcile
  apply --help` now names apply authority as limited to the selected
  reconciliation mutation, not evidence, GateDecision, claim truth, global
  truth, or publication.
- **Decision reconciliation selection-review help boundary.** `haft decision
  reconcile selection-review --help` now names validation reports as context,
  not operator approval, not evidence, not GateDecision, not claim truth, not
  global truth, not publication, and not apply authority.
- **Decision reconciliation help boundary.** `haft decision reconcile --help`
  now names reconciliation plans as review context, not operator approval, not
  evidence, not GateDecision, not claim truth, not global truth, not
  publication, and not apply authority.
- **Decision reconciliation metrics help boundary.** `haft decision reconcile
  metrics --help` now names metrics packets as review context, not operator
  approval, not evidence, not GateDecision, not claim truth, not global truth,
  not publication, and not apply authority.
- **Overseer drain help boundary.** `haft overseer drain --help` now names drain
  as opt-in, machine-safe-only, not semantic approval, evidence, GateDecision,
  claim truth, global truth, publication, or reconciliation apply authority.
- **Overseer judgment help boundary.** `haft overseer judgment --help` now names
  judgment packets as read-only review metadata that do not mutate, approve, or
  create evidence.
- **Carrier manifest help boundary.** `haft carrier manifest --help` now names
  carrier classes as review/discovery metadata, not binding authority by
  themselves.
- **Carrier check help boundary.** `haft carrier check --help` now names semio
  findings as review inputs, not evidence, approval, or GateDecision.
- **Drift events help boundary.** `haft drift events --help` now names computed
  `root_cause`, `resolution_status`, and `suggested_next_command` as review
  posture, not evidence, approval, or gate passage.
- **Reconciliation selection-draft help boundary.** `haft decision reconcile
  selection-draft --help` now names operator-approval, evidence-truth,
  gate-passage, and apply-authority boundaries for draft/current-metrics review
  aids.
- **Governing-set help boundary.** `haft decision governing-set --help` now
  names evidence, approval, gate, claim-truth, global-truth, publication, and
  reconciliation-apply authority boundaries.
- **Value space help boundary.** `haft value space --help` now names evidence,
  approval, gate, claim-truth, global-truth, publication, and product-value
  proof boundaries.
- **Evidence path help boundary.** `haft evidence path --help` now names claim
  truth and publication alongside evidence, approval, gate, and global-truth
  non-authority boundaries.
- **Spec lifecycle help boundaries.** `haft spec sync`, `haft spec export`,
  `haft spec apply-change`, and `haft spec classify-change` help now name
  evidence, gate, claim-truth, global-truth, and prose-authority boundaries.
- **Drift route compact boundary.** `haft drift route` text output now names
  the same `claim_truth` and `publication` non-authority posture already
  present in JSON.
- **Blocked-use attention compact boundary.** `haft attention blocked` text
  output now names the same `claim_truth` and `publication` non-authority
  posture already present in JSON.
- **Baseline audit spec-review classification.** `haft baseline audit` now
  classifies spec-review `authority_boundary.rebaseline` vocabulary as
  read-only lifecycle authority instead of legacy ambiguous baseline debt.
- **Spec review authority boundary.** `haft spec review` and
  `haft_query(action="spec_review")` now expose fielded authority-boundary
  posture for advisory semantic review packets, making findings explicitly not
  evidence, approval, rebaseline, GateDecision, SpecUseAdmission, claim truth,
  global truth, or publication.
- **Spec use publication boundary.** `haft spec use` and
  `haft_query(action="spec_use")` now include publication posture in
  `current_authority` and OperationalGate/no-OperationalGate authority
  boundaries, keeping SpecificationUseRecord exact/audit output aligned with
  other read-only projection surfaces that are not evidence, approval,
  GateDecision, claim truth, global truth, or publication.
- **Generated contract carrier sync.** `haft interface contract-generation
  --sync-materialized-carriers` now rewrites only kernel interface catalog
  source-digest markers in listed host/skill/plugin/Pi carriers, then verifies
  them with the existing materialized-carrier check. The sync report is explicit
  CLI-only and states that it is not host runtime materialization, binding
  authority, evidence, approval, GateDecision, claim truth, global truth, or
  publication.
- **Contract-audit authority boundary.** `haft interface contract-audit` and
  `haft_query(action="contract_audit")` now expose a top-level read-only
  authority boundary stating that the contract inventory is not evidence,
  approval, GateDecision, claim truth, global truth, publication, schema
  generation, or host materialization.
- **Maintenance drain authority boundary.** `haft overseer drain --json`,
  `haft_refresh(action="drain")`, and `haft interface refresh.drain` now state
  that drain reports, reconciliation proposals, and after-action reports are
  not claim truth, global truth, or publication authority. Compact JSON remains
  bounded and drain mutation behavior is unchanged.
- **Read-only route/attention authority boundaries.** `haft drift route`,
  `haft attention blocked`, and their MCP discovery surfaces now expose
  advisory route and blocked-use attention boundaries as not evidence,
  approval, GateDecision, claim truth, global truth, or publication. This keeps
  repair suggestions and action invitations from looking like performed work,
  approval, or published truth.
- **Governing-set authority boundary.** `haft decision governing-set` and
  `haft_query(action="governing_set")` now expose current-authority frontier,
  snapshot, and answer-path boundaries as read-only projections that are not
  evidence, approval, GateDecision, claim truth, global truth, or publication.
  Text summaries now print the full frontier boundary instead of truncating it
  back into an ambiguous gate-only cue.
- **Spec use authority boundary.** `haft spec use` and
  `haft_query(action="spec_use")` now expose `current_authority` as a
  read-only frontier that is not evidence, approval, GateDecision, claim truth,
  or global truth. The no-OperationalGate case now emits an explicit
  non-authority boundary instead of an empty boundary object, and text
  summaries print the full gate boundary including WorkCommission and
  claim-truth posture.
- **Spec sync/apply top-level authority boundary.** `haft spec sync` and
  `haft spec apply-change` now state that SQL edition mutations are not
  approval, rebaseline, evidence, GateDecision, claim truth, global truth, or
  prose authority. This matches the strengthened source/publication/carrier
  audit subrecord boundary.
- **Spec export publication boundary.** SpecSection export/sync exact-audit
  records now distinguish publication projection metadata from approval,
  rebaseline, evidence, GateDecision, claim truth, and global truth. The
  `--markdown` carrier projection still omits audit metadata, while JSON/text
  views expose the stronger authority boundary.
- **Baseline audit authority boundary.** `haft baseline audit --json` now
  includes an explicit `mutation_boundary`, and the text summary prints the
  same read-only boundary. Interface contract test fragments such as
  `missing_symbol_baseline` and
  `propose_rebaseline_with_binding_targets` are classified as legacy binding
  scope vocabulary instead of false-positive ambiguous baseline debt.
- **Decision reconciliation draft counter clarity.** R9 selection drafts now
  distinguish broad `plan_scope_enrichment_candidates` from draftable
  `reviewable_scope_enrichment_candidates`, while keeping the existing
  `scope_enrichment_candidates` field as the draftable compatibility count.
  This prevents embedded `current_metrics` from appearing to contradict the
  selection-draft summary when the reconciliation plan also contains non-apply
  review work.
- **Decision reconciliation selection draft metrics context.** R9
  `haft decision reconcile selection-draft --json` now embeds a read-only
  `current_metrics` snapshot from the reconciliation metrics surface, so
  operator-reviewed scope-enrichment selections carry visible before/after
  context without making drafts apply-ready or copying metrics into the
  operator-approved selection template.
- **DriftEvent binding drill-down routing.** DriftEvents with
  `root_cause=binding_target_missing` now point `suggested_next_command` at
  ranked `haft drift bindings --dry-run --json` review before reconciliation,
  instead of sending operators to the broader decision-reconcile report first.
- **Drift binding candidate review ranking.** Compact
  `haft drift bindings --dry-run --json` items now include review-only
  `ranking_policy`, ranked candidate previews, and file-level
  `candidate_review_groups` so old-decision binding reviews surface the most
  relevant symbols first without creating binding authority or changing the
  full audit path.
- **Drift binding dry-run review load.** `haft drift bindings --dry-run
  --json` now keeps compact review packets token-bounded by projecting
  candidate symbols and diagnostics as previews with omitted counts. Long
  diagnostic messages and full candidate lists remain available through the
  explicit full audit path, `haft drift bindings --json`.
- **Projection authority boundary completeness.** `haft correspondence graph`,
  `haft change case`, and their `haft_query(...)`/`haft interface ...`
  surfaces now explicitly report that these read-only projections are not
  claim truth and not publication, matching the existing non-proof,
  non-evidence/non-work, non-approval, non-gate, and non-global-truth
  boundaries.
- **Value-space authority boundary completeness.** `haft value space` and
  `haft_query(action="value_space")` now explicitly report that engineering
  value projections are not claim truth and not publication, matching the
  existing not-score/not-evidence/not-approval/not-gate/not-global-truth
  boundary.
- **Decision reconciliation fallback target cleanup.** R9
  `enrich_scope` reconciliation selections now support explicit
  `remove_whole_file_fallback_targets` entries for named existing
  whole-file fallback `binding_targets`. Selection drafts surface those
  removable fallback keys as review hints, `selection-review` rejects unknown
  or non-fallback removals, and apply removes only the operator-approved
  fallback targets while preserving precise governance/symbol targets.
- **Decision reconciliation draft subject preservation.** R9
  `haft decision reconcile selection-draft --json` now preserves an existing
  `decision_subject_ref` in proposed `enrich_scope` selections instead of
  replacing it with a TODO placeholder. The draft remains report-only:
  `operator_approval_ref` and review reasons are still required, and whole-file
  fallback targets still require operator review before apply.
- **Drift binding dry-run command parity.** `haft drift bindings --dry-run
  --json` is now accepted as an explicit read-only alias for the default
  binding-target review report, matching the command surfaced by status and
  overseer drill-down hints. Its JSON output is compact by default with
  omitted-item counts and a `full_audit_command`, while plain
  `haft drift bindings --json` remains the full audit report. Combining
  `--dry-run` with binding mutation flags now fails closed. `haft interface
  drift.binding_review --json` now documents the CLI-only review/mutation
  boundary, and generated-contract carrier digest markers were synced to the
  updated kernel interface catalog.
- **Spec health and recall evidence regressions.** Added focused regression
  coverage for spec drift `section_id` reporting, CLI/MCP spec-onboarding JSON
  key parity, cross-project recall history injection for paraphrased hits, and
  the live CrossHybrid recall floor against the current decision corpus.
- **Generated-contract materialized carrier check.** `haft interface
  contract-generation --check-materialized-carriers` now validates every listed
  host/skill/plugin/Pi carrier for current source-digest and authority-boundary
  markers in one compact read-only report, so agents can detect stale
  generated-contract carriers without manually grepping the repo.
- **Overseer drain compact JSON projection.** `haft overseer drain --dry-run
  --json` now emits a compact audit projection by default while preserving
  complete summary counts, explicit omitted counters, and
  `full_audit_command`. Use `--json --full` for the full audit payload and
  `--limit N` to tune the compact sample size.
- **Decision reconciliation approval-readiness review metadata.** R9
  `haft decision reconcile selection-draft --json` now includes structured
  per-candidate `approval_readiness` metadata: why the draft is not
  apply-ready, which placeholders remain, what operator checks are required,
  and which selection fields must be confirmed. Compact text shows
  `readiness` and blocker count while preserving `operator_approved=false`,
  `selected_candidates=0`, and the separate fail-closed `selection-review` /
  `apply` boundary.
- **Decision reconciliation draft structured proposed selection.** R9
  `haft decision reconcile selection-draft --json` now exposes a
  `proposed_selection` object for each review candidate in addition to the
  legacy escaped `selection_template` string, so agents and operators can
  inspect the exact proposed `enrich_scope` item without parsing JSON text.
  The field remains report-only, keeps `operator_approved=false`, and does not
  copy review-only hints or create apply authority.
- **Overseer status JSON compact projection.** `haft overseer status --json`
  now emits the same grouped drift/stale signal projection as compact text by
  default, with `--full` preserving the raw per-decision signal list for exact
  drill-down.
- **Product-value evidence refresh.** The 2026-06-24 product-value evidence
  packet now includes the stale `haft serve` diagnostic and contract-audit
  authority-inventory guard, with updated value-space evidence-ref counts and
  current drift metrics while preserving the equal-budget comparison gap.
- **Contract-audit authority inventory guard.** Contract-audit regression tests
  now pin the binding and lifecycle/semantic mutation surfaces
  (`decision.decide`, `decision.reconcile_apply`, and `spec.apply_change`) so
  they cannot silently disappear from the read-only surface inventory.
- **Doctor stale serve diagnostics.** `haft doctor` now reports current-project
  `haft serve` processes, warning when duplicate or older MCP servers are still
  running from a different executable than `PATH` resolves, so rebuild/restart
  drift is visible before agents trust stale status output.
- **Spec export interface contract.** `haft interface spec.export --json` now
  documents the existing `haft spec export` publication projection surface,
  including SQL source edition, publication hash, carrier bytes, markdown-only
  output, and the read-only authority boundary.
- **README product-claim boundary.** The public README now describes Haft as a
  bounded project-local FPF governance substrate for human-authorized AI
  engineering work, not as a complete FPF operating system.
- **Product-value evidence ref count.** The 2026-06-24 product-value evidence
  packet now reports the actual public `haft value space` evidence-ref count
  (`15`) instead of the stale earlier packet count.
- **Status maintenance-plan drill-down wording.** Default status now labels
  `haft_refresh(action="plan")` as compact and names
  `haft_refresh(action="plan", verbose=true)` as the full work-order route.
- **Status runtime provenance.** `haft_query(action="status")` now includes a
  bounded `haft serve` runtime fingerprint with PID, process start time,
  executable path, and executable mtime so agents can distinguish stale MCP
  server processes from freshly rebuilt binaries.
- **Maintenance plan compact default.** `haft_refresh(action="plan")` now
  keeps deterministic and machine-checkable work visible while sampling the
  Rung-3 judgment tail by default, with `verbose=true` retaining the full work
  order. This prevents the MCP maintenance plan from dumping dozens of
  operator-judgment tasks into agent context.
- **Baseline audit bounded JSON projection.** `haft baseline audit --json
  --limit N` now keeps full summary/diagnostic counts while truncating the
  emitted findings with explicit `projection.omitted_findings` and a
  `full_audit_command`, so agents can check `legacy_ambiguous_baseline`
  without dumping the full repository audit into context.
- **Product-value scope-violation evidence.** The 2026-06-24 product-value
  packet now records a deliberate R9 scope/authority violation attempt as a
  read-only blocked-use attention item with exact source-return requirements,
  without claiming runtime gate passage, approval, evidence truth, or global
  product proof.
- **Contract-generation discovery shape counts.** The
  `query.contract_generation` interface discovery example now matches the live
  manifest counts after `spec.apply_change`, with a regression test comparing
  the illustrative summary to the generated report.
- **Spec sync-back no-bloat guards.** Default status, default
  `code_context`, MCP `tools/list`, and compact contract-generation summaries
  now regression-test that the long `spec.apply_change` dry-run contract shape
  stays behind explicit interface/contract drill-downs.
- **Spec sync-back interface contract.** `haft interface spec.apply_change`
  now documents the CLI-only Markdown-to-SQL sync-back path, including the
  required classify/dry-run/apply sequence, authority boundary, `planned_edition`
  dry-run shape, and the guarantee that sync-back is not approval, rebaseline,
  evidence, GateDecision, or prose authority.
- **Root spec carrier semio coverage.** `haft carrier check` now scans
  root-level `spec/*.md` support docs, and the root workflow/agent contracts no
  longer describe removed `haft agent` or desktop surfaces as current runtime
  paths.
- **Enabling-system carrier semio coverage.** `haft carrier check` now scans
  `spec/enabling-system/*.md`, treats the desktop layer contract as an explicit
  archived carrier, and keeps current enabling-system architecture/stack docs
  aligned to the v8 host-skills + MCP + CLI surface model.
- **Dead-surface carrier semio guard.** Carrier semio checks no longer treat a
  neighboring "supported hosts" phrase as permission to present desktop/TUI/
  standalone surfaces as current, and target-system support docs now state the
  v8 current surface as host skills/prompts + MCP + CLI while keeping desktop
  references archived/provenance-only.
- **Transport parity guard current query actions.** The transport action parity
  regression now documents the current `haft_query` MCP action surface,
  including code-intelligence, ceremony, check, and term-resolution actions,
  while keeping the old standalone surface as documented legacy drift instead
  of silently omitting current actions from the guard.
- **Product-value evidence post-rebuild packet.** The 2026-06-24 product-value
  evidence now reflects the rebuilt installed CLI, the latest autonomous
  maintenance run/undo command, the read-only overseer drill-down fix, the host
  discipline carrier guard, and current reconciliation metrics without
  presenting the packet as approval, gate passage, global truth, or solved
  authority-frontier noise.
- **Overseer status read-only drill-down hints.** Compact overseer/status output
  no longer advertises mutating `haft overseer maintain --json` as an
  inspection path; it now points drift/stale/suppression review at bounded
  judgment, dry-run drain, status JSON, or `haft_refresh` read-only drill-downs.
- **Host discipline carrier semio coverage.** `haft carrier manifest` now
  classifies root `CLAUDE.md` as a current host discipline mirror, and
  `haft carrier check` scans it for prompt/schema/model authority-boundary
  violations alongside `AGENTS.md`, README, templates, skills, generated
  surfaces, and Pi bundle carriers.
- **WLNK assurance boundary wording.** The older assurance one-liner now labels
  evidence/formality/reliability displays as diagnostic-only, not approval, gate
  passage, claim truth, global truth, or publication.
- **Maintenance reconciliation proposal drill-downs.** Read-only maintenance
  reconciliation proposals now suggest bounded
  `haft decision reconcile --json --limit 5` / governing-set `--limit 5`
  inspection commands instead of full audit JSON dumps.
- **Decision reconciliation CLI compact limits.** Read-only
  `haft decision reconcile --json --limit N` and
  `haft decision governing-set --json --limit N` now return bounded compact
  projections matching the MCP drill-down behavior while preserving legacy
  `--json` full-audit output.
- **Overseer status bounded judgment drill-down.** `haft overseer status` now
  points compact drift-confirmation signals at
  `haft overseer judgment --json --limit 20` instead of the full audit JSON
  packet.
- **Bounded overseer judgment JSON.** `haft overseer judgment --json --limit N`
  now returns a compact read-only review packet with omitted task/proposal
  counts and a `full_audit_command`, and status/interface hints point agents to
  the bounded drill-down instead of forcing a full needs-judgment dump.
- **Default status needs-binding drift count.** The default cockpit Decision
  Health line now classifies legacy whole-file fallback DriftEvents as
  needs-binding resolution work, matching the DriftEvent summary instead of
  reporting them as material drift or `0 needs-binding` noise.
- **SQL-first spec check document summaries.** `haft spec check --json` now
  aggregates repeated SQL-first SpecSection carrier rows by path/kind so the
  `documents` list reports each carrier once while preserving section counts,
  active counts, term-map counts, and findings.
- **Carrier-only semantic drift routing.** `haft drift route carrier_only` and
  `haft_query(action="drift_route", drift_kind="carrier_only")` now classify
  carrier-only changes as recognized carrier-layer drift instead of unknown
  high-risk drift, preserving semantic authority while keeping the route
  read-only.
- **Product-value evidence live rebuild refresh.** The 2026-06-24 product-value
  evidence packet now matches rebuilt live metrics and includes the
  report-only reconciliation review packet slice as evidence without claiming
  approval, gate passage, global truth, or solved authority-frontier noise.
- **Current governing frontier snapshot checks.** `haft decision governing-set`
  now supports `--write-snapshot` and `--check-snapshot` JSON carriers so agents
  can compare read-only authority-frontier digests across slices without
  creating approval, evidence truth, gate passage, or reconciliation authority.
- **Bounded reconciliation selection drafts.** `haft decision reconcile
  selection-draft --json` now emits a compact review slice by default with
  emitted/omitted candidate counts and an explicit `--full` audit command,
  preventing R9 scope-enrichment review candidates from flooding agents while
  preserving the complete report-only draft.
- **Reviewable reconciliation draft ordering.** Bounded reconciliation
  selection drafts now put higher-confidence, precise-target enrichment
  candidates before low-confidence TODO-heavy candidates, keeping the default
  review packet focused without changing apply authority.
- **Selection document templates for reconciliation drafts.** Report-only
  reconciliation selection drafts now include a top-level
  `selection_document_template` assembled from the emitted bounded items, with
  an empty `operator_approval_ref` so review/apply still reject it until the
  operator explicitly approves the packet.
- **Reconciliation selection template export.** `haft decision reconcile
  selection-draft --write-template selection.json` now writes the bounded
  `selection_document_template` to a review file while preserving the empty
  `operator_approval_ref`, so `selection-review` and `apply` remain
  fail-closed until explicit operator approval exists.
- **Reconciliation selection placeholder validation.** `selection-review` and
  `apply` now reject `TODO_...` placeholder values in reconciliation selection
  documents, so exported templates cannot become apply-ready by filling only
  `operator_approval_ref`.
- **Reconciliation subject-ref review hints.** Selection drafts now include
  report-only `decision_subject_ref_suggestions` derived from decision metadata
  to reduce R9 review friction, while exported apply templates still keep TODO
  placeholders and require explicit reviewed values.
- **Reconciliation carrier review hints.** Selection drafts now include
  `decision_carrier_hint` and `review_commands` so R9 reviewers can jump back
  to the source DecisionRecord and narrowed draft view without treating carrier
  text as approval or apply authority.
- **Current-operation reconciliation selection validation.** `selection-review`
  and `apply` now reject stale `enrich_scope` selections when the current
  reviewed group no longer advertises `apply_operation=enrich_scope`, forcing
  agents to rebuild the packet from the current reconciliation plan.
- **Stale reconciliation group diagnostics.** `selection-review` and `apply`
  now explain when an old `reviewed_group_id` is gone but the same
  `decision_refs` match a current reconciliation group, including the current
  group id and apply operation while keeping the stale packet fail-closed.
- **Default status contract-generation bloat guard.** Default
  `haft_query(action="status")` now has regression coverage proving it does not
  inline generated contract-generation manifests, fragments, schema digests, or
  runtime schema-audit details.
- **Compact contract-generation CLI bloat guard.** Compact
  `haft interface contract-generation` output now has regression coverage
  proving it stays at summary counts and does not inline generated fragment,
  schema, or materialized-carrier detail.
- **Baseline audit autonomous rebaseline classification.** `haft baseline
  audit` now classifies human-readable autonomous maintenance titles such as
  "Autonomous additive rebaseline" as autonomous maintenance wording instead
  of unresolved legacy ambiguous baseline debt.
- **DriftEvent resolution record posture.** DriftEvent reports now expose a
  read-only `resolution_record_posture` so agents can distinguish applied
  resolution ledger records from stale target bindings, inactive waivers, and
  unsupported ledger statuses without treating the record as decision authority.
- **DriftEvent resolution posture binding.** DriftEvent resolution ledger
  records now bind to the event's materiality and audit-only posture, so an old
  resolved/waived record remains visible for audit but no longer closes the
  event after the same target becomes materially different.
- **Value-space simplify/kill dashboard visibility.** `haft value space` now
  prints the read-only simplify/kill review triggers in compact text output
  instead of hiding them behind a count, while preserving the no-score and
  not-gate/not-approval authority boundaries.
- **Compact drift/reconciliation drill-down limits.**
  `haft_query(action="drift_events"|"decision_reconcile"|"governing_set",
  limit=N)` now applies `limit` to compact MCP projections while preserving
  `full=true` audit views untruncated.
- **Host-style integer compact limits.** DriftEvents compact-projection tests now
  cover both JSON-decoded numeric limits and host-supplied integer limits, so
  bounded `limit=5` drill-downs stay bounded across MCP client adapters.
- **Bounded DriftEvent fanout payloads.** Compact DriftEvent reports now cap
  each event's inlined `impacted_decisions` and expose
  `omitted_impacted_decisions`, so `haft_query(action="drift_events", limit=5)`
  stays bounded while `full=true` retains the full audit payload.
- **Budgeted status drill-down recommendations.** Compact status,
  reconciliation cues, and prompt-governor attention now recommend bounded
  `limit=5` drift/reconciliation/governing-set drill-downs, while keeping
  explicit `full=true` audit routes for untruncated inspection.
- **Bundled status carrier drill-down sync.** The bundled `/h-status`,
  `/h-reason`, `/h-verify`, and Pi `h-status` carriers now point agents at the
  same bounded compact drift/reconciliation/governing-set drill-downs, and the
  interface catalog documents `limit`/`full` for those MCP actions.
- **Contract-generation runtime schema audit.** `haft interface
  contract-generation --json` and `haft_query(action="contract_generation")`
  now include a read-only `runtime_schema_audit` that validates generated MCP
  schema fragments against the live `ToolCatalog` action enum, required fields,
  properties, and schema digests, while compact output keeps only mirror/drift
  counts.
- **Runtime schema audit no-default-bloat guards.** Default status,
  `code_context`, MCP `tools/list`, and compact contract-generation text now
  reject inline `runtime_schema_audit` detail while preserving summary counts
  behind the explicit contract-generation surface.
- **Contract-generation materialized carrier manifest.** `haft interface
  contract-generation --json` and `haft_query(action="contract_generation")`
  now list materialized host/skill/plugin/Pi carrier files with source-contract,
  source-digest guarded sync posture, required markers, fragment refs, and
  validation refs, while compact text keeps only counts and default status still
  omits the manifest.
- **Generated MCP schema fragment materialization.** `haft interface
  contract-generation --write-schema-fragments <path>` now writes a
  deterministic JSON carrier for generated MCP action schema fragments with
  source and carrier digests, giving host/schema sync checks a real generated
  artifact without making it runtime schema authority.
- **Generated description fragment materialization.** `haft interface
  contract-generation --write-description-fragments <path>` now writes a
  deterministic JSON carrier for generated host/skill/plugin/Pi description
  fragments with source and carrier digests, so wording sync can be checked as
  generated artifact bytes without treating generated text as approval.
- **Generated contract carrier drift checks.** `haft interface
  contract-generation --check-schema-fragments <path>` and
  `--check-description-fragments <path>` now compare materialized carrier files
  with the current kernel interface catalog and fail on drift without rewriting
  the files.
- **Governing frontier snapshot digest.** `haft decision governing-set --json`
  and `haft_query(action="governing_set")` now include a stable
  `snapshot_digest` in the read-only snapshot metadata, so current-authority
  frontier projections can be referenced and compared without relying on
  status prose or volatile `generated_at` values.
- **Overseer reconciliation posture summary.** Maintenance-run JSON now includes
  a compact read-only `reconciliation_summary` with proposal counts, kind
  counts, fallback/high-fanout posture, max fanout, suggested commands, and the
  proposal authority boundary so agents do not need to dump every proposal to
  see the maintenance posture.
- **Overseer autonomous action allowlist.** Maintenance-run normalization now
  accepts only maintenance effects (`auto_rebaseline`, `observable_run`,
  `revalidate_stale`) as executed autonomous actions, with regression coverage
  rejecting lifecycle/binding kinds such as supersede, retire, merge, and
  approve.
- **Overseer autonomous ledger retention.** Status loading now retains the
  newest maintenance run with executed autonomous actions even after a later
  report-only `overseer maintain` run becomes `latest-maintenance`, so the
  operator still sees the exact undo commands for applied autonomous work.
- **Pi contract-source digest guard.** The bundled Pi tool metadata now carries
  the current kernel interface catalog `source_digest`, and regression coverage
  compares it against the read-only `contract_generation` manifest so Pi
  descriptions cannot silently drift from kernel-owned interface contracts.
- **MCP status autonomous-maintenance disclosure guard.** Regression coverage now
  proves `haft_query(action="status")` surfaces latest overseer maintenance
  actions, including the `AUTONOMOUS MAINTENANCE` block and exact
  `haft overseer undo <run-id> <action-id>` command, before the normal Haft
  cockpit.
- **Projection assurance authority boundary.** Audit/evidence projections now
  state that displayed `R_eff`, `F_eff`, formality scale, and bridge-loss
  diagnostics are not approval, gate passage, claim truth, global truth, or
  publication.
- **Governing frontier history split.** `haft decision governing-set` and
  `haft_query(action="governing_set")` now expose an explicit
  `authority_frontier` separating current governing DecisionRecord refs from
  terminal historical refs, so audit consumers do not infer live authority from
  superseded/deprecated history.
- **Decision reconciliation draft selection boundary.** `haft decision reconcile
  selection-draft` now keeps low/medium-confidence scope-enrichment candidates
  as review candidates instead of counting them as selected apply candidates;
  compact text prints both review-candidate and selected-candidate counts.
- **Pi generated schema mirror coverage.** Pi TypeBox schema regression coverage
  now validates every generated MCP schema fragment, not only `haft_query`, and
  the packaged Pi mirrors now include the missing ProblemCard, ChoiceResult,
  semantic-spine decision, and binding-target fields from the kernel interface
  catalog.
- **Formality schema wording boundary.** Decision evidence schema text now names
  current F0-F9 formality, `formality_scale_id`, legacy/unversioned bridge/loss
  posture, and the non-authority boundary, while MCP `tools/list` remains under
  the compact context budget.
- **MCP DriftEvent compact source-items guard.** Default
  `haft_query(action="drift_events")` JSON now omits serialized
  `source_items` audit detail and reports `omitted_source_items` instead,
  preserving `source_items` only for `full=true` audit drill-downs.
- **Baseline audit authority-boundary classification.** `haft baseline audit`
  now classifies explicit `not_approval_not_rebaseline` spec sync/apply
  authority-boundary tokens as lifecycle-authority wording instead of legacy
  ambiguous baseline debt, eliminating false-positive status/audit noise for
  those surfaces.
- **Carrier authority receipt wording guard.** Carrier semio checks now flag
  prompt text, model-supplied arguments, generated text/schema, skill
  descriptions, plugin metadata, and Pi metadata when they are described as
  authorization receipts, approval evidence, binding authority, or gate
  passage, while preserving the valid host receipt verifier boundary.
- **Contract-generation materialized schema filtering.** Generated MCP schema
  fragments now omit fields explicitly excluded from MCP schema coverage and
  skip CLI/input-file-only apply contracts, preventing read-only discovery
  actions from masquerading as materialized host schemas.
- **MCP DriftEvent resolution ledger parity.** `haft_query(action="drift_events")`
  and compact status/governor status now apply the same default
  `.haft/drift-event-resolutions.json` read-only overlay as `haft drift
  events`, so resolved or unexpired waived DriftEvents are not reported as open
  cockpit noise through MCP while remaining non-binding metadata in drill-downs.
- **DriftEvent scope-manifest root cause.** Scope-manifest adjacent file churn
  now reports `root_cause=implementation_footprint_churn` instead of
  `schema_changed`, while material scope-manifest events keep the
  `schema_changed` root cause.
- **Decision reconciliation interface discovery.** Interface contracts now
  expose `claim_lifecycle_update` selection documents and the exact
  `lineage_relations` labels restored by full reconciliation preview, so
  agents can discover the existing operator-approved claim lifecycle and
  lineage paths without inferring them from code.
- **Pi query action mirror guard.** The MCP tool catalog tests now compare the
  Pi/plugin `haft_query` action enum against the kernel `tools/list` enum, so
  new read-only query actions fail tests if the bundled Pi schema mirror drifts.
- **Read-only query stale footer fallback.** If the typed status stale lane is
  unavailable while rendering MCP query footers, Haft now suppresses raw stale
  debt instead of falling back to `FindStaleDecisions` counts that can disagree
  with the compact status cockpit.

### Added

- **Spec apply-change dry-run preview.** `haft spec apply-change --dry-run`
  now runs the same typed carrier parser, SQL freshness/conflict guard, and
  planned-edition projection as a real sync-back without writing the SQL
  edition store, so scalar/relationship carrier edits can be reviewed before
  becoming source-truth editions.
- **Reconciliation review packet writer.** `haft decision reconcile
  selection-draft --write-review-packet review.json` now writes the bounded
  report-only draft with decision carrier hints, review commands, omitted
  counts, mutation boundaries, and the non-approved embedded selection template,
  so R9 scope-enrichment review can be persisted without creating approval or
  apply authority.
- **Product-value evidence packet.** `spec/target-system/PRODUCT_VALUE_EVIDENCE.md`
  now records the 2026-06-24 bounded dogfood evidence packet for stale
  reconciliation fail-closed behavior, default-surface bloat guards, and
  baseline-audit cleanup, while explicitly preserving the no-score,
  not-approval, not-gate, and not-equal-budget-proof boundaries.
- **Spec SQL-edition export surface.** `haft spec export SECTION_ID` now
  renders one current SQL SpecSection edition as a deterministic Markdown
  carrier projection, with `--json` for exact hashes/audit metadata and
  `--markdown` for carrier bytes, while preserving the boundary that export is
  not approval, rebaseline, evidence, or prose authority.
- **Spec sync-back field registry guard.** SpecSection carrier change
  classification now has regression coverage proving every exported
  `SpecSection` JSON field is explicitly classified as scalar, relationship,
  carrier-only, or high-risk, so future canonical fields cannot silently bypass
  SQL sync-back posture.
- **Spec apply-change text audit output.** Human-readable
  `haft spec apply-change` now prints the source episteme, publication
  projection, carrier bytes, imported semantic mutation, and carrier-only
  disposition, so SQL truth versus carrier/projection boundaries are visible
  without switching to JSON.
- **Overseer after-action authority guard.** Maintenance run regression
  coverage now asserts after-action reports remain report-only and do not carry
  reconciliation apply or binding lifecycle mutation cues while listing
  auto-closed, evidence-checked, and remaining operator-judgment items.
- **Generated contract no-default-bloat guard.** Compact interface, status, and
  code-context regression helpers now also reject generated contract virtual
  carrier paths, keeping generated host/schema surface inventory behind
  explicit carrier/contract drill-downs.
- **Spec PublicationUnit round-trip guard.** SpecSection publication regression
  coverage now proves carrier path changes do not change the semantic source
  edition or publication hash, while semantic edits do, keeping SQL truth,
  publication projection, and carrier metadata separated.
- **Spec sync-back stale-carrier conflict guard.** `haft spec apply-change`
  now fails closed when an existing current SQL SpecSection edition has a
  different semantic hash than the reviewed `--before` carrier, returning a
  conflict payload and preserving SQL truth instead of overwriting a newer
  edition.
- **Decision reconciliation text audit cues.** Human-readable
  `haft decision reconcile selection-review` output now prints the read-only
  mutation boundary, and `haft decision reconcile apply` summaries print
  lineage relation labels plus claim lifecycle updates, so operator review can
  see the authority and after-action effects without switching to JSON.
- **Spec sync-back fail-closed preservation guard.** `haft spec apply-change`
  regression coverage now proves an unknown/high-risk carrier edit cannot
  overwrite an existing current SQL SpecSection edition.
- **Generated contract carrier semio guard.** `haft carrier check` now includes
  virtual surfaces for `contract_generation` preview and schema fragments, so
  generated host/plugin/Pi carrier text is authority-boundary checked before
  materialization.
- **Pi runtime-carrier materialization guard.** `haft init --pi` regression
  coverage now pins the embedded package boundary: runtime extension, prompt,
  skill, README, and package metadata are materialized, while tests, scripts,
  lockfile, tsconfig, and `node_modules` stay out.
- **Contract-generation schema fragments.** `haft interface contract-generation`
  and `haft_query(action="contract_generation")` now emit read-only generated
  MCP action schema fragments with schema digests, transport required fields,
  action-specific handler-validated fields, and the explicit boundary that
  schema visibility is not operator authorization or host materialization.
- **Generated schema fragment parity guard.** Generated MCP schema fragments
  are now tested against `tools/list` action enums, required fields, and
  top-level properties so `contract_generation` cannot drift into a fantasy
  schema surface.
- **Generated schema property fidelity.** `contract_generation`
  `generated_schema_fragments` now copy the actual `tools/list` property schema
  subsets for generated per-action fragments instead of placeholder field
  descriptions.
- **Pi generated schema mirror parity.** The generated MCP schema fragments are
  now compared against the materialized Pi `haft_query` TypeBox schema, so Pi
  action, field, and required-field drift is caught before release.
- **Contract-audit required-field parity.** `haft interface contract-audit` and
  `haft_query(action="contract_audit")` now report MCP `required` coverage for
  transport-level required fields, including missing required schema fields and
  action-specific required fields that remain validated by handlers.
- **Contract-audit default-bloat guards.** Default status, default
  `code_context`, and MCP `tools/list` tests now reject inline
  contract-audit required-coverage fragments, keeping required-field parity in
  explicit drill-downs only.
- **Pi package contract-carrier guard.** The Pi README/package metadata now
  point maintainers at the read-only `contract_generation` fragments before
  editing Pi tool, skill, or prompt wording, with regression coverage preserving
  the boundary that generated text and schema visibility are not approval
  receipts.
- **Pi h-status generated-query carrier guard.** The Pi `h-status` skill now
  points at `contract_generation`, DriftEvent, decision reconciliation, and
  governing-set drill-downs, with regression coverage tied to the same
  generated fragments as the bundled skill carriers.
- **Pi generated-query contract sync guard.** Pi tool metadata now carries
  compact generated-fragment hints for contract generation, DriftEvents,
  decision reconciliation, and governing-set drill-downs, with a regression test
  tying those hints back to the kernel contract-generation manifest.
- **Bundled skill generated-query contract sync guard.** Embedded `h-status`,
  `h-verify`, and `h-reason` now carry compact generated-fragment drill-down
  hints for contract generation, DriftEvents, decision reconciliation, and
  governing-set queries, with regression coverage tied to the kernel manifest.
- **Decision reconciliation selection-draft cues.**
  `haft decision reconcile selection-draft` now emits report-only candidate
  posture, confidence, suggested review action, and blocking questions so
  agents can drop ambiguous scope-enrichment candidates instead of guessing
  operator-approved selections; compact output includes posture/confidence.
- **Contract-generation preview fragments.** `haft interface contract-generation`
  now emits read-only generated preview fragments from the kernel interface
  catalog for host/skill/plugin/Pi synchronization, while keeping compact
  output count-only and preserving the authority boundary that generated text
  and schema visibility are not operator approval.
- **Pi generated-contract authority guards.** The bundled Pi tool metadata and
  manual-gate prompts now carry the generated-contract authority boundary for
  read-only and binding surfaces, with Go and Pi tests proving the wording
  stays aligned with the `contract_generation` manifest instead of treating
  schema visibility as operator approval.
- **Bundled skill/template authority guards.** Embedded `h-decide`,
  `h-commission`, `h-reason`, the current `AGENTS.md`/`CLAUDE.md`, and the
  `haft init` project template now carry the same generated-contract binding
  boundary, with a regression test tying the carrier wording back to the
  kernel manifest.
- **Contract-generation manifest evidence.** The read-only
  `contract_generation` manifest now hashes the full kernel interface catalog
  source and always reports validation refs, so an empty generator-target queue
  is an evidenced state instead of a bare `targets: []` projection.
- **Spec-use current-authority gate input.** `haft spec use` and
  `haft_query(action="spec_use")` now include a read-only
  `current_authority` frontier posture for the SpecSection target, and
  OperationalGate evaluation fails closed on current-authority conflicts or
  overlap that requires operator review.
- **Spec-section `section_id` drift target evaluators.** Semantic binding
  targets now recognize `section_id` in YAML and JSON carriers, so
  spec-section targets can bind to bounded target ranges instead of falling
  back to file-level drift when carriers use `section_id` rather than `id`.
- **Measurement evidence formality posture.** `Measure` now applies the current
  F0-F9 formality scale metadata to measurement evidence before claim
  recomputation and persistence, avoiding bare numeric F2 evidence in exact and
  audit paths.
- **WLNK formality projection diagnostics.** Assurance/WLNK summaries no longer
  promote evidence items with missing formality scale metadata to the current
  F0-F9 scale; unversioned evidence is reported with an explicit
  `unversioned-formality` scale and source-scale gap bridge loss.
- **Value-space evidence-missing summary.** `haft value space` compact output
  now shows the declared measurement window, method ref, distinct evidence-ref
  count, and the number of value characteristics still blocked by missing
  evidence refs, while preserving the no-single-score authority boundary.
- **Bounded product-value evidence packet.** Target-system docs now include a
  current Haft product-value evidence packet for the semantic-spine rewrite,
  with explicit evidence refs, missing-evidence boundaries, simplify/kill
  criteria, and unmeasured gaps; the evidence ontology and term map now name
  current F0-F9 formality with legacy F0-F3 bridge/loss wording.
- **Product-value evidence refresh.** The semantic-spine product-value evidence
  packet now includes the read-only query stale-footer fallback guard as
  compact-status/noise-control evidence.
- **Equal-budget product-value comparison protocol.** The product-value
  evidence packet now defines a parity protocol for comparing Haft against an
  AI agent using ADRs plus tests, while explicitly marking the comparison as
  designed but not yet run and preserving the no-single-score policy.
- **Runtime identity and stale-footer hygiene.** Dev builds now surface VCS
  commit/source metadata in `haft version`, `scripts/build.sh --install`
  stamps commit/build date via ldflags, and read-only MCP query footers now use
  the same typed stale snapshot as `haft_query(action="status")` instead of
  raw terminal-decision stale counts.
- **Evidence/WLNK authority boundary.** `haft_decision(action="evidence")` and
  measurement responses now state that evidence and WLNK/formality display are
  not approval, not gate passage, and not global truth, and route stronger
  attempted-use reliance through EvidencePath.
- **EvidencePath claim/publication boundary.** EvidencePath authority
  boundaries now explicitly state that evidence/formality diagnostics do not
  create claim truth or publication, in addition to not creating approval,
  gate passage, or global truth.
- **MCP tools/list output-shape guard.** The MCP `tools/list` context-budget
  tests now reject rich interface output fragments such as EvidencePath
  reliance-disposition and authority-boundary values, keeping exact/audit
  examples behind explicit interface drill-downs.
- **Default status/code-context output-shape guards.** Compact status and
  default `code_context` tests now reject rich interface output fragments,
  preserving drill-down-only exact/audit examples outside normal cockpit and
  agent-context payloads.
- **Compact interface catalog output-shape guard.** The default `haft interface`
  catalog test now rejects generated-manifest and rich output-shape fragments,
  keeping top-level CLI discovery as a capability list plus JSON drill-down
  pointer.
- **Binding-denial receipt candidates.** `operator_confirmation_required`
  payloads now include explicit authorization receipt candidates: manual CLI as
  the default accepted binding path and host receipts only when a registered
  kernel verifier can confirm principal, session, action, payload hash, expiry,
  and source.
- **Read-only reconciliation selection review.** `haft decision reconcile
  selection-review SELECTION.json` now validates a proposed or
  operator-approved reconciliation selection against the same core rules as
  `apply`, reports `apply_ready`, item-level errors, and the apply command only
  when authority is already `operator_approved_reconciliation_selection`,
  without creating approval or mutating DecisionRecords.
- **Current-plan reconciliation selection validation.** Reconciliation
  selection review and apply now reject stale or fictional `reviewed_group_id`
  values and require each selected decision to belong to the reviewed current
  `DecisionReconciliationPlan` group before any mutation can run.
- **Code-context invariant authority precision.** File-level
  `haft_query(action="code_context")` and `lane="invariants"` now label
  invariants as file-level relevance candidates, cap the default candidate
  list, and point agents to symbol narrowing or `full=true` audit instead of
  presenting historical file/module fanout as local must-hold constraints.
- **Code-context high-fanout invariant summary.** `lane="invariants"` now
  summarizes very large invariant fanouts by decision/source group by default,
  avoiding hundreds of invariant sentences in normal agent context while
  preserving the complete invariant audit list behind `full=true`.
- **Compact drift-events CLI summary.** `haft drift events` now caps the
  default text event list and points to `--json` for the full audit payload,
  preserving DriftEvent grouping without dumping every per-event tail into
  normal operator context.
- **Compact MCP drift-events drill-down.** `haft_query(action="drift_events")`
  now returns a compact JSON report by default: complete summary counts, capped
  event rows, omitted counters, and a `full=true` audit command. Passing
  `full=true` preserves the complete `source_items` and compatibility report.
- **Compact MCP reconciliation drill-downs.**
  `haft_query(action="decision_reconcile")` and
  `haft_query(action="governing_set")` now return compact JSON projections by
  default, preserving summary counts, top rows, omission counters, and
  `full=true` audit commands for complete group/set payloads.
- **Governing-set answer-path parity guard.** The MCP
  `haft_query(action="governing_set")` tests now prove that emitted
  `answer_paths[].mcp_call` / `source_refs` drill-downs filter the current
  governing frontier to the exact advertised `target_ref`.
- **Pi extension carrier wording guard.** `haft carrier check` now scans the
  Pi extension runtime/tool source files in addition to package metadata,
  prompts, skills, docs, generated interface fragments, and MCP tool
  descriptions, so host package tool wording cannot imply binding authority
  outside the checked carrier set.
- **Pi query schema mirror refresh.** The Pi bundle `haft_query` and
  `haft_refresh` tool schemas now mirror the current read-only drill-down
  actions, maintenance review/drain actions, and common optional fields used by
  spec-use, drift, reconciliation, evidence-path, blocked-use, and value-space
  surfaces.
- **Pi schema drift guard.** The MCP `ToolCatalog()` tests now compare the
  kernel `haft_query` and `haft_refresh` action enums against the Pi bundle
  TypeScript schema mirror, so new kernel actions cannot silently leave the Pi
  host package stale.
- **Overseer judgment reconciliation cues.** `haft overseer judgment` and
  `haft overseer judgment --json` now include read-only reconciliation proposal
  cues from the existing high-fanout/fallback review pipeline, with proposal
  counts, kind counts, inspect-only commands, and an explicit
  `read_only_reconciliation_proposal_not_binding_authority` boundary.
- **SQL-first spec structural checks.** `haft spec check`, `haft check`, and
  `haft spec coverage` now derive structural SpecSection checks from current
  SQL SpecSection editions when present, while preserving typed carrier
  term-map entries as support data.
- **SQL-first spec read-path guard.** CLI runtime spec surfaces now have a
  regression test that rejects direct carrier parsing unless the file is the
  SQL-first compatibility helper, the explicit `spec sync` carrier import
  path, or the read-only `spec classify-change` before/after review path.
- **SQL-first WorkCommission spec snapshots.** `create_from_decision` now
  resolves `spec_section_refs` and revision hashes through current SQL
  SpecSection editions when a `project_root` is supplied, preserving carrier
  fallback for unsynced projects.
- **Assurance calculator F0-F9 formality preservation.** The legacy
  `assurance` WLNK calculator now preserves `formality_level` on the current
  FPF F0-F9 ordinal instead of folding F4-F9 into old F0-F3 buckets, and
  reports unversioned scale/bridge diagnostics for SQL evidence rows that lack
  an explicit `formality_scale_id`.
- **Host receipt verifier boundary.** Authorization receipt evaluation now has
  an explicit source-keyed host verifier registry: structurally complete host
  receipts still fail closed unless a kernel verifier is registered, and tests
  cover verifier success, denial, and malformed verifier results.
- **Baseline audit typed-legacy precision.** `haft baseline audit` now treats
  explicit `UnknownLegacyBaseline` compatibility model references as typed
  baseline model surface and classifies SQL onboarding missing-baseline test
  wording as SpecSection approval-baseline diagnostics, reducing false
  `legacy_ambiguous_baseline` noise.
- **SQL-first SpecSection baseline mutations.** Operator-invoked
  `haft_spec_section` / `haft spec onboard` approve/rebaseline/reopen paths now
  baseline current SQL SpecSection editions when present, with carrier fallback
  preserved for unsynced projects.
- **SQL-first spec health findings.** `haft check` / `haft_query(action="check")`
  now compute SpecSection drift, missing-baseline, and staleness health against
  current SQL SpecSection editions, matching the SQL-first structural
  SpecSection check path.
- **SQL-first spec onboarding reads.** Read-only `haft spec onboard` now uses
  current SQL SpecSection editions before Markdown carriers while keeping
  missing-baseline findings as the human approval gate.
- **SQL-first spec reads preserve term-map carriers.** SQL-backed SpecSection
  reads now keep typed carrier term-map entries as support data, and
  `haft_query(action="resolve_term")` resolves section refs from SQL editions
  without dropping term-map definitions.
- **SQL-first SpecificationUseRecord reads.** `haft spec use` and
  `haft_query(action="spec_use")` now read current SQL SpecSection editions
  before Markdown carriers while preserving baseline-currentness and
  GateDecision authority boundaries.
- **SQL-first SpecSection next-step reads.** `haft_spec_section(action="next_step")`
  now uses current SQL SpecSection editions before Markdown carriers, matching
  the existing CLI lifecycle projection while preserving missing-baseline
  blocking semantics.
- **Contract-generation code-context budget guard.** Default
  `haft_query(action="code_context")` regression tests now reject hidden
  contract-generation manifest fragments so generated-schema drill-downs cannot
  bloat normal code-context traces.
- **Governing-set answer-path precision.** `haft decision governing-set --json`
  now classifies exact answer paths for `spec_section`, API-contract, symbol,
  fallback, file-fallback, and unscoped targets, with explicit read-only
  drill-down hints for stronger-use review.
- **Baseline audit tail-surface classification.** `haft baseline audit` now
  classifies agent lifecycle text, decision/commission skill references,
  parity action lists, legacy-binding tests, SpecSection projection/staleness
  text, and remaining test fixture examples without hiding source-level
  `UnknownLegacyBaseline` diagnostics.
- **Baseline audit code-tail classification.** `haft baseline audit` now
  classifies binding-surface inventories, spec-review/apply/sync
  no-authority text, reconciliation metrics, comparison/value parity wording,
  overseer maintenance guardrails, symbol-drift context, and ordinary
  score/graph baseline wording while preserving explicit legacy diagnostics.
- **Baseline audit carrier/protocol wording classification.** `haft baseline
  audit` now classifies current README, AGENTS/CLAUDE guardrails, Pi
  h-verify prompts, MCP protocol docs, and v8 migration wording as existing
  carrier, lifecycle, verified-state, or DecisionRecord API surfaces instead of
  legacy ambiguous baseline debt.
- **Baseline audit artifact verification/maintenance classification.** `haft
  baseline audit` now classifies artifact verification tests and maintenance
  plan baseline wording as verified-state or autonomous-maintenance surfaces
  without hiding explicit `BaselineKindUnknownLegacy` diagnostics.
- **Baseline audit workflow-skill routing classification.** `haft baseline
  audit` now classifies `h-onboard`, `h-reason`, and `h-spec-cover` baseline
  routing text as workflow lifecycle guidance instead of legacy ambiguous debt.
- **Baseline audit drift-repair routing classification.** `haft baseline audit`
  now classifies drift-event repair, maintenance-review, reconciliation, and
  drift-route rebaseline/no-mutation wording as lifecycle or maintenance
  routing terminology instead of unresolved baseline debt.
- **Baseline audit benchmark/presentation test classification.** `haft
  baseline audit` now classifies retrieval/projection benchmark test wording
  as comparison baseline terminology and presentation-output assertions as
  presentation surface terminology.
- **Baseline audit MCP/tool API classification.** `haft baseline audit` now
  classifies MCP server schema text and haft tool baseline-output tests as
  DecisionRecord baseline API surface terminology.
- **Baseline audit overseer/MethodPack classification.** `haft baseline audit`
  now classifies overseer maintenance rebaseline wording as autonomous
  maintenance terminology and built-in MethodPack baseline evidence wording as
  a separate method-pack surface.
- **Baseline audit symbol-drift classification.** `haft baseline audit` now
  classifies `internal/codebase/symhash.go` baseline/current symbol snapshot
  wording as verified-state drift comparison terminology.
- **Baseline audit artifact drift classification.** `haft baseline audit` now
  classifies DecisionRecord drift/query/refresh baseline wording as
  verified-state snapshot terminology without hiding explicit
  `BaselineKindUnknownLegacy` debt.
- **Baseline audit SpecSection lifecycle/schema classification.** `haft
  baseline audit` now classifies remaining SpecSection lifecycle, sync,
  missing-baseline check, and schema wording under current SpecSection
  lifecycle/approval surfaces while preserving explicit unknown-legacy findings.
- **Baseline audit lockfile exclusion.** `haft baseline audit` now skips
  generated dependency lockfiles such as `package-lock.json` so package names
  like `baseline-browser-mapping` do not appear as semantic baseline debt.
- **Baseline audit project-spec carrier classification.** `haft baseline audit`
  now separates project specification carriers such as `.haft/specs/*` and
  `spec/target-system/*` from upstream source-spec references and legacy
  ambiguous baseline debt.
- **Baseline audit presentation helper classification.** `haft baseline audit`
  now classifies `noBaselineCount` and safe rebaseline display wording as
  presentation-surface terminology rather than unresolved baseline debt.
- **Baseline audit skill/guardrail surface classification.** `haft baseline
  audit` now classifies `h-spec`, `h-verify`, and agent guardrail baseline
  wording as current lifecycle/verification surfaces instead of legacy
  ambiguous baseline debt.
- **Baseline audit verification/run snapshot classification.** `haft baseline
  audit` now classifies verification-pass baseline fields and `haft run`
  baseline-phase output as verified-state snapshot terminology.
- **Baseline audit interface-contract classification.** `haft baseline audit`
  now classifies baseline wording inside the interface contract catalog as
  contract-surface terminology rather than unresolved baseline semantics.
- **Baseline audit integration-test fixture classification.** `haft baseline
  audit` now recognizes baseline helper wording in CLI integration and golden
  tests as test-fixture surface without hiding explicit unknown-legacy baseline
  diagnostics.
- **Baseline audit SpecSection drift classification.** `haft baseline audit`
  now classifies SpecSection missing-baseline and drift-finding wording under
  SpecSection approval baseline terminology.
- **Baseline audit maintenance execution classification.** `haft baseline audit`
  now classifies autonomous maintenance executor, undo, and rebaseline test
  wording as autonomous-maintenance baseline terminology.
- **Baseline audit verified-state decision vocabulary.** `haft baseline audit`
  now recognizes remaining DecisionRecord drift, symbol-baseline, CL3
  verification-pass, and baseline-test wording as verified-state snapshot
  terminology.
- **Baseline audit parity BaselineSet classification.** `haft baseline audit`
  now recognizes camel-case `BaselineSet` and parity-plan baseline wording as
  comparison/benchmark baseline terminology.
- **Baseline audit SpecState freshness classification.** `haft baseline audit`
  now classifies SpecSection state/freshness enforcement wording under
  SpecSection approval baseline terminology instead of legacy ambiguous debt.
- **Baseline audit presentation-surface classification.** `haft baseline audit`
  now classifies baseline/drift response formatting vocabulary separately from
  legacy ambiguous baseline debt.
- **Baseline audit decision API classification.** `haft baseline audit` now
  classifies `haft_decision(baseline)`, host-tool schema, and DecisionRecord
  baseline handler/test wording separately from legacy ambiguous baseline debt.
- **Baseline audit spec lifecycle CLI classification.** `haft baseline audit`
  now classifies spec approval/rebaseline CLI and SpecSection handler test
  wording as baseline lifecycle-authority terminology.
- **Baseline audit legacy-binding scope vocabulary.** `haft baseline audit` now
  classifies old-decision symbol-baseline enrichment and binding-target
  rebaseline terminology separately from legacy ambiguous baseline debt.
- **Baseline audit SpecUse currentness vocabulary.** `haft baseline audit` now
  classifies SpecificationUse baseline currentness and admission terminology
  separately from legacy ambiguous baseline debt.
- **Baseline audit autonomous-maintenance vocabulary.** `haft baseline audit`
  now classifies auto-baseline, auto-rebaseline, maintenance undo, and baseline
  snapshot/restore terminology separately from legacy ambiguous baseline debt.
- **Baseline audit retrieval benchmark vocabulary.** `haft baseline audit` now
  recognizes deterministic retrieval/golden baseline terms such as
  `baselineResults`, `baselineHits`, `topBaseline`, and baseline search errors
  as comparison/benchmark terminology.
- **Baseline audit SpecSection lifecycle handler surface.** `haft baseline
  audit` now classifies remaining `internal/cli/serve_spec_section.go`
  lifecycle helper vocabulary such as `baselineMutation` and `baselineContext`
  as lifecycle-authority surface rather than legacy ambiguous baseline debt.
- **Baseline audit decision baseline vocabulary.** `haft baseline audit` now
  recognizes decision baseline API and drift materiality terms such as
  `BaselineProfile`, `DriftNoBaseline`, `noBaselineMateriality`,
  `missingFileMateriality`, baseline operation log keys, and binding-target
  rebaseline prompts as verified-state snapshot terminology.
- **Baseline audit SpecSection store surface classification.** `haft baseline
  audit` now treats `internal/project/specflow/baseline.go` implementation
  vocabulary as SpecSection approval baseline store terminology after explicit
  unknown-legacy and typed-model checks have had a chance to classify it.
- **Baseline audit test-fixture surface classification.** `haft baseline audit`
  now classifies explicit `_test.go` helper and fixture vocabulary such as
  `newBaselineTestProject` separately from product terminology debt, while
  still scanning test files.
- **Baseline audit SpecSection approval vocabulary.** `haft baseline audit` now
  recognizes SpecSection lifecycle terms such as `spec_lifecycle_approval_baseline`,
  `PutSpecSectionApproval`, `projectBaseline`, `DeriveStateWithBaselines`, and
  baseline recorded/current/overwritten messages as SpecSection approval
  baseline terminology.
- **Baseline audit verified-state vocabulary.** `haft baseline audit` now
  recognizes decision drift/file-hash baseline vocabulary such as
  `BaselineInput`, `HasBaseline`, `DriftNoBaseline`, stored baseline hashes,
  and symbol-level baselines as verified-state snapshot terminology.
- **Baseline audit self-surface classification.** `haft baseline audit` now
  classifies the audit command implementation and its tests as a dedicated
  audit-tool surface, so classifier vocabulary does not report itself as
  legacy ambiguous terminology debt.
- **Baseline audit release-notes carrier classification.** `haft baseline audit`
  now classifies `CHANGELOG.md` baseline mentions as release-notes provenance,
  so release history remains audit-visible without dominating current legacy
  terminology diagnostics.
- **Baseline audit lifecycle-authority classification.** `haft baseline audit`
  now separates approve/rebaseline and human-gate wording from legacy ambiguous
  baseline terminology, keeping authority-boundary text visible without
  treating it as an untyped baseline object.
- **Baseline audit typed-model classification.** `haft baseline audit` now
  classifies explicit `BaselineKind`, `SectionBaseline`, `BaselineStore`, and
  baseline-shaped compatibility/projection terminology as typed baseline model
  surface rather than legacy-ambiguous debt, while keeping actual unknown legacy
  posture visible as legacy.
- **Baseline audit source-spec classification.** `haft baseline audit` now
  classifies upstream FPF source specification carriers under
  `data/FPF/` separately from current legacy-ambiguous baseline terminology
  debt, preserving source-spec audit visibility without making imported
  vocabulary look like live Haft rewrite work.
- **Baseline audit support/archive carrier classification.** `haft baseline
  audit` now classifies `.haft` MethodPack, night-run, plan, and Pi bundle
  carriers separately from current legacy-ambiguous baseline terminology debt,
  keeping those carriers visible without letting archived/support text dominate
  diagnostics.
- **Baseline audit historical governance classification.** `haft baseline audit`
  now classifies baseline mentions inside historical `.haft` governance carriers
  separately from current `legacy_ambiguous` terminology debt, and skips nested
  `node_modules` dependency noise regardless of directory depth.
- **Batchable reconciliation selection drafts.** `haft decision reconcile
  selection-draft` now supports read-only `--limit`, `--group-id`, and
  `--decision-ref` filters so old-decision scope-enrichment approval packets
  can be kept small and reviewable without becoming operator approval or
  applying reconciliation mutations.
- **Legacy file-scope drift binding fallback.** DriftEvent reports now classify
  legacy whole-file decision bindings as explicit `whole_file_fallback`
  `binding_target_missing` events that require scope enrichment, instead of
  reporting them as undifferentiated `unknown_high_risk`; material drift stays
  visible and routes to `haft decision reconcile --json`.
- **Spec carrier-change field registry.** SpecSection carrier-change
  classification now uses a single typed registry for high-risk, semantic
  scalar, relationship, and carrier-only fields. This preserves the current
  read-only classifier behavior while making future canonical fields and
  relations extend through one mechanism instead of scattered one-off checks.
- **Contract fragment posture in interface audit.** `haft interface
  contract-audit` and `haft_query(action="contract_audit")` now classify every
  interface fragment as validated, generated-target, legacy/manual, or
  unvalidated, with summary counts kept in the explicit drill-down report. This
  makes generated/validated/legacy contract posture visible without generating
  schemas or expanding default status payloads.
- **EvidencePath formality diagnostics.** `haft evidence path` and
  `haft_query(action="evidence_path")` now expose explicit
  `formality_diagnostics` for current F0-F9, legacy lossy, and unversioned
  formality readings. Stronger uses that require current F0-F9 formality now
  block unversioned evidence instead of silently treating missing scale metadata
  as current.
- **Engineering value simplify/kill criteria.** `haft value space` and
  `haft_query(action="value_space")` now include read-only
  `simplify_kill_criteria` review triggers for scope violations, missing
  parity comparisons, evidence gaps, false blocking, ceremony without measured
  value movement, and scalarized proxy-value claims. The criteria remain
  explicit review input only: they are not automatic gates, evidence, approval,
  `GateDecision`, or product-value proof.
- **Contract-generation no-default-bloat guards.** Added regressions proving
  the read-only contract generation manifest remains drill-down only and is not
  inlined into default `haft_query(status)` or MCP `tools/list` payloads.
- **Pi package metadata carrier guard.** `haft carrier check` now includes
  `packages/haft-pi/package.json` in the semio scan set while still excluding
  package-lock dependency noise, so plugin/Pi metadata cannot imply operator
  authorization or evidence authority unchecked.
- **Baseline audit legacy diagnostics.** `haft baseline audit` now emits
  explicit read-only diagnostics for legacy ambiguous baseline terminology,
  including affected file count, examples, and next-action guidance to classify
  wording without mutating baseline state.
- **Decision reconciliation preview cues.** Compact
  `haft decision reconcile` text now surfaces read-only preview cues for top
  groups, including lineage relation count, downstream dependents, downstream
  migration requirement, successor workflow posture, and claim lifecycle count,
  while keeping the full reconciliation record behind `--json`.
- **SQL-first spec read compatibility.** `haft spec review` and
  `haft spec status` now read current `spec_section_editions` from the SQL
  project graph before falling back to markdown carriers, while preserving the
  existing read-only review/lifecycle projections and failing closed when a
  present SQL edition cannot be projected back into the canonical spec set.
- **Host authorization receipt fail-closed shape.** Authority receipt
  evaluation now validates future host receipts for explicit source,
  principal, session, action, payload hash, and expiry, but still fails closed
  unless a real kernel verifier exists; model-supplied arguments remain
  non-receipts.
- **Generated MCP tool carrier guard.** `haft carrier check` now treats MCP
  `tools/list` tool descriptions and input-schema descriptions as generated
  virtual carrier surfaces, so host-facing schema text is checked for forbidden
  operator-authorization wording before materialization.
- **Decision reconciliation selection draft.** Added read-only
  `haft decision reconcile selection-draft` / `--json` to generate
  operator-reviewable `enrich_scope` candidate skeletons from reconciliation
  scope-repair groups without creating an operator-approved apply document or
  mutating decision authority.
- **Baseline terminology audit.** Added read-only `haft baseline audit`
  / `--json` to classify repository `baseline` wording across code, tests,
  docs, skills, templates, and `.haft` carriers as spec approval, pre-work
  reference, verified-state snapshot, comparison, ordinary-language, or legacy
  ambiguous usage while skipping Open-Sleigh and generated dependency noise.
- **Baseline split rewrite regression.** Added compatibility coverage proving
  pre-work reference and verified-state snapshots cannot rewrite an existing
  `SpecSectionApprovalBaseline` row for the same spec section.
- **Baseline split measurement regression.** Added coverage proving impact
  measurement records evidence without rewriting the decision drift baseline
  hashes, preserving verified-state snapshots as drift references rather than
  turning measurement into an implicit rebaseline.
- **Decision drift baseline profiles.** Drift reports and `haft check --json`
  now label stored decision file-hash baselines as
  `verified_state_snapshot` with an authority boundary, making the legacy
  `affected_files.hash` carrier explicit as a drift-detection snapshot rather
  than spec approval or a pre-work reference baseline.
- **EvidencePath current-formality guard.** `haft evidence path` and
  `haft_query(action="evidence_path")` now accept an explicit
  current-formality requirement for stronger attempted uses. When enabled,
  legacy or undeclared/lossy formality blocks bounded reliance instead of being
  treated as current F0-F9 evidence; the record remains read-only and still
  cannot create approval, gate passage, or global truth.
- **Audit projection formality scale labels.** Audit/evidence projections now
  print `F_eff` with the associated formality scale id and bridge-loss posture,
  so projection consumers no longer see a bare F-level without knowing whether
  it came from current F0-F9 or a legacy/lossy reading.
- **Assurance summary formality scale labels.** WLNK assurance summaries now
  retain the existing `F_eff` display while adding the scale id and bridge-loss
  posture for the evidence item that determines the weakest formality reading.
  Legacy F0-F3 evidence remains readable as lossy legacy formality rather than
  being silently presented as current F0-F9 authority.
- **EvidencePath formality diagnostics.** The read-only `haft evidence path`
  text summary and `query.evidence_path` discovery contract now name evidence
  formality scale, bridge, and loss posture explicitly. Legacy or undeclared
  F-levels stay diagnostic-only and still cannot create approval, gate passage,
  or global truth.
- **DriftEvent resolution target binding.** `haft drift events resolve` now
  stores the current event target identity (`changed_target_ref`, `target_kind`,
  `target_status`, and `root_cause`) in the non-binding resolution ledger.
  Applying the ledger keeps old records visible for audit but no longer lets a
  resolved/waived overlay close a different target after retargeting, rename,
  or root-cause changes.
- **DriftEvent source claim/evidence refs.** Drift reports now carry
  `claim_refs` and `evidence_refs` on `source_items` when a DecisionClaim
  explicitly names the changed governance target and evidence is bound to that
  exact claim. This is explicit-only metadata: prose is not mined, superseded or
  deprecated claims are ignored, and drift verdicts do not change.
- **YAML semantic target evaluator.** Explicit spec-section, API-contract, and
  invariant binding targets can now attach bounded evaluators from YAML-style
  carrier blocks using `id:` or `target_ref:`. Drift outside the matching YAML
  object remains audit-only; changes inside the object become semantic-target
  drift without falling back to whole-file scope.
- **Fuzzy edited-symbol retarget candidates.** When an old governed symbol file
  disappears and exact moved-symbol matching fails, drift detection now surfaces
  a low-confidence `retarget_candidate` if exactly one same-name symbol exists
  elsewhere with a changed body. Ambiguous matches stay unretargeted, and fuzzy
  candidates always require binding resolution rather than moving authority.
- **JSON semantic target evaluator.** Explicit spec-section, API-contract, and
  invariant binding targets can now attach bounded evaluators from JSON carrier
  objects using structured `id` or `target_ref` fields. Sibling JSON object
  edits remain audit-only; matching object edits become semantic-target drift.
- **Decision reconciliation lineage labels.** Reconciliation previews and
  operator-approved apply outcomes now expose explicit `lineage_relations` for
  `mergedFrom`, `supersedes`, `retiredWithSuccessor`, and
  `retiredWithoutSuccessor`, keeping authority-frontier cleanup auditable
  without treating preview text as a binding mutation.
- **Decision reconciliation downstream migration report.** Read-only
  reconciliation previews now include `downstream_migration_report` with
  before-apply review policy, dependent refs, selection impact, and an explicit
  `auto_relink=false` boundary so successor/retirement selections cannot hide
  downstream dependency work.
- **Decision reconciliation successor workflow preview.** Merge/supersede
  previews now include a read-only `consolidated_successor_workflow` contract
  that names required successor packet fields such as retained/withdrawn claims,
  changed assumptions, evidence, scope, drift watch targets, and `valid_until`
  without creating or approving a successor.
- **Decision reconciliation claim lifecycle apply.** Operator-approved
  reconciliation selections now support `claim_lifecycle_update` for explicit
  claims, allowing partial claim supersede, retire, or reopen without changing
  the parent DecisionRecord's current authority.
- **Current governing frontier provenance.** Read-only governing-set JSON now
  carries a `snapshot` envelope with source, projection, status-policy, terminal
  history policy, filter posture, and generated timestamp so audit views can
  distinguish historical decisions from current authority without relying on
  prose status summaries.
- **Current governing-set answer paths.** Governing-set JSON entries now expose
  read-only `answer_paths` for claim, spec-section, API-contract, invariant,
  symbol, fallback, and unscoped targets, giving agents exact CLI/MCP drill-downs
  without treating the path itself as evidence or a gate decision.
- **Overseer reconciliation proposals and after-action reports.** Maintenance
  runs and drain JSON now include read-only `reconciliation_proposals` for
  high-fanout/fallback groups plus an `after_action` report listing auto-closed
  items, evidence refs, remaining operator judgment, and undo commands, without
  giving overseer authority to apply reconciliation selections.
- **FPF provenance and MethodPack source posture.** FPF retrieval JSON now
  carries explicit source provenance for source kind, edition/hash,
  profile-validity, normativity, schema version, and retrieval mode while
  compact text stays compact. MethodPack definitions/cards now carry
  `source_posture` metadata and MCP pull/show responses label method cards as
  non-normative support carriers, so task-local method guidance cannot
  masquerade as authoritative FPF source material.
- **Read-only interface contract audit.** Added
  `haft interface contract-audit --json`, `haft_query(action="contract_audit")`,
  and `haft interface query.contract_audit` as a Phase F0 audit surface over the
  kernel-owned `haft interface` catalog. The report classifies contract sources,
  MCP schema posture, CLI availability, binding-sensitive authority posture,
  validation refs, and documented legacy transport exceptions without generating
  schemas or changing binding authority.
- **MCP schema validation guard.** Exposed the MCP server's compact tool catalog
  through a pure `ToolCatalog()` method and added a contract test that compares
  every MCP tool/action declared by `haftInterfaceCatalog()` against the
  `tools/list` action enums. This is the first Phase F1 validation step: it
  catches host-schema drift before generated schemas exist and does not change
  default status or binding authority.
- **MCP schema property guard.** Added a second contract test that compares
  top-level required/optional fields declared by `haftInterfaceCatalog()` with
  the advertised MCP `tools/list` schema properties. The guard caught stale
  `haft_decision` schema coverage for decision subject, scope, binding target,
  drift-watch, footprint, and claim fields; those fields are now advertised as
  compact properties while the tools/list budget test remains green.
- **Contract audit schema coverage.** `haft interface contract-audit --json`
  and `haft_query(action="contract_audit")` now include per-surface
  `schema_coverage` plus summary counts for covered surfaces, missing schema
  surfaces, and explicit compatibility exclusions. Agents can inspect schema
  drift posture from a read-only drill-down instead of inferring it from tests
  or expanding default status.
- **Contract audit nested shape coverage.** The contract audit drill-down now
  compares input-like `field_shapes` with nested MCP schema properties and
  reports `shape_coverage` per surface plus covered/missing/skipped shape
  counts. Output-only shapes are skipped explicitly; missing nested fields are
  exposed as generator targets without changing runtime behavior or binding
  authority.
- **Contract audit dynamic shape classification.** The nested-shape audit now
  treats `solution.compare` score variant IDs as an explicit dynamic-shape
  skip instead of a false missing-schema finding. Real nested generator targets
  remain visible for decision scope/claim shapes and spec-use operational gate
  shapes, while the MCP `tools/list` context-budget guard remains green.
- **Contract audit generator-target classification.** Remaining real nested
  shape gaps are now classified as explicit `generator_targets` instead of
  `missing_shape_fields`. The report surfaces generator target fields and
  summary counts for `decision.decide` scope/claim/footprint shapes and
  `query.spec_use` operational-gate shapes, keeping schema drift visible
  without bloating MCP `tools/list` or treating planned generation work as a
  failing audit.
- **Contract generation target manifest.** Added the read-only
  `haft interface contract-generation --json` /
  `haft_query(action="contract_generation")` manifest. It derives the remaining
  nested schema generator targets from the kernel interface catalog and
  contract audit, exposes a stable source digest plus field-level target list,
  and preserves the boundary that schema visibility is not operator
  authorization, evidence, or gate passage.
- **Nested MCP schema coverage for current generator targets.** The current
  `decision.decide` scope/claim/footprint shapes and `query.spec_use`
  operational-gate shape now have explicit nested MCP schema properties. The
  contract audit reports zero remaining generator target surfaces, and the
  contract generation manifest returns an empty queue while the `tools/list`
  context-budget guard remains green.
- **Generated-surface authority wording guard.** `haft carrier check` now
  flags current/support/compat carrier wording that implies prompt text,
  model-supplied MCP arguments, tool descriptions, or schema visibility create
  operator authorization. Explicit denial wording remains allowed, keeping
  README, specs, generated skills, and Pi bundle carriers lintable without
  adding noise to default status. The check also scans generated interface
  descriptions as virtual surfaces and reports them through
  `checked_generated_surfaces`, so host-facing catalog text is covered before it
  is materialized into tool/schema carriers.
- **Contract generation surface policy.** The read-only
  `haft interface contract-generation --json` manifest now carries a typed
  `surface_policy` naming the no-default-bloat rules for status, code_context,
  tools/list, compact CLI output, and future generated descriptions. The policy
  keeps generation targets as explicit drill-down data and requires carrier
  semio authority-boundary checks before host materialization.
- **Spec carrier change classifier.** Added a pure project-level
  `SpecSection` carrier change classifier for the future Markdown sync-back
  path. It distinguishes carrier-only movement, recognized semantic scalar
  updates, relationship updates, mixed updates, and unknown/high-risk edits that
  must abstain/block, without mutating SQL or treating arbitrary prose as
  authority.
- **Read-only spec carrier change review.** Added
  `haft spec classify-change --before <file> --after <file> --section <id>` as
  an explicit before/after review surface over the classifier. The command
  supports JSON/text output and stays classification-only: it does not sync SQL,
  approve sections, rebaseline drift, or promote surrounding Markdown prose to
  authority.
- **SpecSection SQL edition storage.** Added additive
  `spec_section_editions` storage for current project-local SpecSection source
  editions. The new store persists the typed section JSON, semantic hash, source
  kind, carrier path, and timestamp for future Markdown sync-back without
  reusing SpecSectionApprovalBaseline hashes or switching existing spec readers
  away from carrier parsing.
- **Typed spec carrier sync into SQL editions.** Added `haft spec sync` as an
  explicit import from `.haft/specs/*` fenced `yaml spec-section` blocks into
  `spec_section_editions`. The command refuses carriers with structural findings
  and records only parsed typed sections; it does not approve, rebaseline,
  reopen, create evidence, or promote surrounding Markdown prose to authority.
- **Reviewed spec carrier change apply.** Added `haft spec apply-change` for
  explicit before/after SpecSection carrier changes. It reuses the classifier
  and writes the after-section to `spec_section_editions` only for recognized
  semantic scalar, relationship, or mixed updates; carrier-only edits are no-op
  and unknown/high-risk changes block.
- **Spec sync/apply audit posture.** `haft spec sync --json` and
  `haft spec apply-change --json` now distinguish the SQL source edition,
  typed YAML publication projection, carrier bytes, imported semantic mutation,
  and carrier-only no-op disposition in explicit audit fields without creating
  approval, rebaseline, evidence, or gate authority.
- **SpecSection edition publication round-trip.** Added a deterministic
  SQL-edition-to-Markdown renderer for typed `yaml spec-section` publication
  projections. The renderer validates its own DB -> Markdown -> parser
  round-trip against the source semantic hash and fails closed on lossy
  projections, proving carrier bytes can rebuild an empty SQL edition store
  without creating approval, rebaseline, evidence, or gate authority.
- **Markdown fenced semantic-target drift evaluator.** Explicit
  `spec_section`, `api_contract`, and `invariant` binding targets in Markdown
  can now attach bounded evaluators from fenced semantic YAML blocks carrying
  `id`, `section_id`, or `target_ref`. Edits to sibling fenced blocks remain
  audit-only; edits inside the governed fenced section become semantic-target
  drift instead of whole-file fallback.
- **Decision reconciliation metrics packet.** Added read-only
  `haft decision reconcile metrics --json` for R9 dogfood cleanup. The packet
  combines reconciliation, current governing-set, and DriftEvent metrics so
  scope-enrichment batches can record before/after fallback scope, fanout,
  material/audit drift, and current-authority conflict counts without applying
  or authorizing any reconciliation selection.
- **Overseer reconciliation autonomy guard.** Strengthened maintenance
  reconciliation tests so overseer/drain proposals are verified as proposal-only
  outputs and cannot suggest `decision reconcile apply`, operator-approved
  selection authority, merge/supersede/retire operations, or claim lifecycle
  mutation.
- **Contract audit host-fragment posture.** The interface contract audit now
  labels every surface with `contract_source_posture` and `host_schema_posture`
  so MCP/CLI fragments are mechanically classified as kernel-catalog source,
  validated MCP mirror, validated MCP mirror with generator targets, or manual
  CLI contract. The report summarizes validated mirrors, manual contracts, and
  unvalidated host fragments while staying a read-only drill-down.
- **Spec review interface discovery.** Added `haft interface
  query.spec_review --json` as the discoverable contract for the existing
  advisory `haft_query(action="spec_review")` / `haft spec review --json`
  surface. The contract names the semantic review v2 profile, advisory-only
  authority boundary, claim/state-reading cues, and stronger-use abstain/block
  posture without adding a new MCP action.
- **DriftEvent resolution metadata ledger.** Added a non-binding DriftEvent
  resolution overlay with `resolved` and scoped `waived_until` records. The new
  `haft drift events resolve EVENT_ID --status ... --reason ...` path writes a
  local resolution ledger, and `haft drift events --resolution-ledger ...`
  overlays that metadata into the report without changing DecisionRecord
  status, lineage, baselines, evidence, gates, or carrier authority.
- **Non-markdown semantic target evaluator markers.** Explicit
  `api_contract`, `invariant`, and `spec_section` binding targets can now attach
  bounded evaluator hashes from exact `haft-target: <target_ref>` markers in
  non-markdown carriers. This keeps semantic drift scoped to the declared target
  block and preserves fail-closed `needs_binding_resolution` behavior when no
  marker, markdown heading, fenced spec-section, or explicit hash exists.

### Fixed

- **Reconciliation selection-draft target prefills.** `haft decision reconcile
  selection-draft` now reuses already-known governance targets in read-only
  `selection_template` output instead of forcing `TODO_target_kind` placeholders
  when a candidate only needs an explicit `decision_subject_ref`.
- **Baseline target authority reuse.** Re-baselining now reuses existing
  effective drift binding targets, hydrating symbol hashes and ranges from
  current source, unless the caller supplies new binding targets, hints, or
  scope instructions. Historical `affected_files` stay a footprint instead of
  forcing ambiguous binding re-inference.
- **Baseline failure atomicity.** `artifact.Baseline` now resolves binding
  targets before persisting affected-file hashes, so a failed ambiguous binding
  resolution records diagnostics without moving the prior drift baseline or
  replacing existing binding targets.
- **Symbol-aware drift noise budget.** Decision drift now records additive
  `materiality` and `trigger_kind` fields, distinguishes material governed
  symbol changes from adjacent file churn, carrier-only edits, generated/local
  runtime noise, and unknown legacy file-scope baselines, and keeps compact
  status focused on material drift while grouping audit-only fan-out into one
  trigger-path summary line. Symbol `binding_targets` now compare their own
  `body_hash` and receiver identity directly, so receiver methods such as
  `SQLiteBaselineStore.Get` and `MemoryBaselineStore.Get` no longer collapse
  through the legacy receiver-less `affected_symbols` projection.
- **Read-only DriftEvent aggregation.** Added `haft drift events` / `--json`
  and `haft_query(action="drift_events")` as a read-only projection over
  existing per-decision drift reports. The projection groups drift by
  file-level changed target, trigger, and materiality, exposes `fanout` and
  `impacted_decisions`, preserves the old `DriftReport` shape as
  `compatibility_reports`, and keeps symbol details inside source items instead
  of multiplying one file change into dozens of top-level events. Compact
  `h-status` now renders unique DriftEvents, impacted decision count, and max
  fanout while leaving full status and refresh scan compatibility reports
  available as drill-downs. DriftEvent drill-downs now also expose
  `needs_binding_resolution_events` plus `fallback_kind` / `fallback_reason`
  when an unresolved whole-file or imprecise binding must be fixed before the
  event is treated as proved material authority drift. DriftEvent JSON is now
  schema v2: symbol-level changed targets are used when material symbol evidence
  exists, file targets remain a labeled fallback, and events carry
  machine-readable `root_cause`, `root_cause_detail`, summary counts for
  semantic targets / file fallback / unknown high-risk events, and
  `resolution_status` values such as `needs_scope_enrichment`,
  `needs_rebaseline`, `needs_operator_judgment`, and `resolved`. Events now
  include read-only `suggested_next_command` hints that point to drill-down or
  review surfaces such as `haft decision reconcile --json` or
  `haft_refresh(action="review")`; the hint is not evidence, approval, or a
  mutation. Scope-manifest drift now reports `root_cause=schema_changed` while
  preserving audit-only/resolved posture for additive schema/scope cues.
- **Read-only DecisionReconciliationPlan.** Added `haft decision reconcile` /
  `--json`, `haft_query(action="decision_reconcile")`, and
  `haft interface query.decision_reconcile` as a deterministic report-only
  authority-frontier projection. It groups active/refresh_due decisions only
  when an explicit `decision_subject_ref`, bounded context, and explicit
  governance-target overlap are present; shared `affected_files` remain
  implementation-footprint hints and never become merge/supersede evidence by
  themselves. Reconciliation and current-governing-set drill-downs now expose
  `scope_enrichment_candidates`, `scope_enrichment_sets`,
  `fallback_target_sets`, and `scope_repair_hints` so old decisions with
  missing subjects, missing targets, or whole-file fallback posture point to
  an operator-approved `enrich_scope` repair path without mutating lineage.
  Each reconciliation group now also carries a read-only `preview` diff with
  current/proposed status or scope effects, required selection fields, downstream
  review cues, and an explicit mutation boundary so agents can show an
  approveable packet before any `apply` document is used. Preview records now
  include `validation_notes` that explain apply-readiness, missing successor
  or scope requirements, and judgment-only conflict cases without creating a
  selection document or authorizing mutation. Preview records also include
  read-only `downstream_impact` counts and dependent refs from links/backlinks
  so successor/retire/merge reviews can see what would need relinking or
  follow-up before apply; the report itself does not relink anything.
- **Decision reconciliation apply path.** Added
  `haft decision reconcile apply SELECTION.json` as an explicit CLI-only
  lineage mutation path for reviewed selections. Selection documents must carry
  `authority=operator_approved_reconciliation_selection`,
  `operator_approval_ref`, `reviewed_group_id`, affected decisions, operation,
  and reason; successor-based operations require an already-created successor
  DecisionRecord. The validator checks the whole batch before mutating, and MCP
  still has only the read-only `decision_reconcile` planning action in this
  slice.
- **Decision scope enrichment apply path.** The same operator-approved
  reconciliation apply document now supports `operation=enrich_scope` for
  precision enrichment of old DecisionRecords. The operation requires exactly
  one decision plus `decision_subject_ref` and governance or drift-watch
  targets, can attach claim governance refs, validates the whole batch before
  mutation, and updates only scope fields without changing status, lineage,
  evidence, baselines, or gates. Reconciliation/governing-set projections now
  read semantic governance/watch target refs even when no concrete code binding
  target is present yet.
- **Decision scope split.** DecisionRecords can now carry additive
  `implementation_footprint`, `governance_targets`, and `drift_watch_targets`
  alongside legacy `affected_files` / `binding_targets`. Drift detection uses
  watch targets first, governance targets second, legacy binding targets third,
  and treats explicit footprint-only files as provenance rather than governance
  drift authority. Binding targets now expose an explicit resolver strategy
  order, a supported-language matrix, `target_ref` for explicit semantic
  targets, and semantic target kinds (`api_contract`, `invariant`,
  `spec_section`) that do not require a file path. Receiver-qualified symbol
  targets remain distinct during normalization, and whole-file fallback carries
  an explicit low-confidence resolution source. Explicit semantic targets with
  concrete carrier/range hashes now participate in drift evaluation:
  unchanged normalized carrier text is audit-only, changed semantic target text
  is `material_semantic_target`, and semantic targets without evaluator hashes
  fail closed as `needs_binding_resolution`. Explicit semantic targets that
  point at an unambiguous markdown heading or fenced `yaml spec-section` id now
  auto-attach bounded line ranges and text hashes, so edits outside the governed
  section remain audit-only while section body changes are material semantic
  drift. When an auto-attached semantic target disappears from its carrier,
  DriftEvent now keys the event by the semantic target and reports
  `target_status=removed` with `root_cause=target_deleted` instead of treating
  the change as anonymous file fallback. Unsupported-language fallback now has
  matrix fixtures proving `.rb`, `.java`, and `.php` changes stay labeled as
  low-confidence whole-file fallback and drift to `needs_binding_resolution`,
  not material symbol drift. New DecisionRecords with `affected_files` now
  auto-enrich `binding_targets` at creation time when the resolver can safely
  pick a precise target; ambiguous multi-symbol files remain unenriched instead
  of blocking `decide` or inventing a whole-file fallback. Markdown-heading
  evaluator fixtures now cover `api_contract` and `invariant` targets as well
  as fenced spec sections, proving bounded drift classification is available
  for all explicit semantic target kinds when the carrier exposes an
  unambiguous heading. Exact moved-symbol continuity now recognizes a governed
  symbol target whose original file disappears but whose same kind/name/receiver
  and body hash appears in another source file, producing a target-level
  DriftEvent with `target_status=renamed` and `root_cause=target_renamed`.
  Edited same-identity moves now surface as
  `target_status=retarget_candidate`,
  `fallback_kind=edited_symbol_move_candidate`,
  `root_cause=retarget_candidate`, and
  `resolution_status=needs_operator_judgment` instead of silently preserving
  authority or retargeting the DecisionRecord.
  Legacy decisions without the new fields keep their existing drift behavior.
- **Read-only CurrentGoverningSet.** Added
  `haft decision governing-set` / `--json`,
  `haft_query(action="governing_set")`, and
  `haft interface query.governing_set` as a derived current-authority frontier.
  It groups active/refresh_due decisions by decision subject, bounded context,
  and effective governance/drift target; terminal decisions remain searchable
  history refs instead of live authority, and overlaps or explicit conflicts
  surface as operator-review cues without lineage, baseline, evidence, or gate
  mutations. The governing-set drill-down now supports focused read-only
  filters: CLI `--query`, `--subject-ref`, and `--target-ref`, plus MCP
  `query`, `bearer_ref`, and `source_refs`, so agents can answer "what
  currently governs this symbol/contract/spec section?" without expanding
  default status.
- **Read-only reconciliation cues in status/governor.** Status data now derives
  a compact `ReconciliationCueReport` from DriftEvents,
  DecisionReconciliationPlan, and CurrentGoverningSet. Default cockpit status
  and prompt-governor status can point to high-fanout drift events,
  reconciliation candidates, and current-authority conflicts with drill-down
  commands, while keeping lineage apply, baseline, evidence, and gate mutation
  paths separate and operator-approved.
- **Compact governor drift policy.** The prompt-governor status path now reports
  drift as unique DriftEvents with impacted decision count and max fanout
  instead of emitting one attention row per drifted DecisionRecord. Governor
  attention keeps a single drift-events drill-down line, preserving the
  evidence-debt cue without multiplying shared-file fanout into prompt noise.
- **Compact overseer status grouping.** `haft overseer status` now keeps the
  autonomous-maintenance undo disclosure visible while grouping per-decision
  drift and stale findings into compact category lines with exact drill-down
  commands. The detailed decision-level items remain available through
  `haft overseer maintain --json`, `haft overseer judgment --json`,
  `haft_refresh(action="scan", verbose=true)`, and
  `haft_refresh(action="review")`.
- **Claim-level lifecycle v1.** `DecisionClaim` now supports additive
  `lifecycle_status`, `successor_ref`, `retired_reason`, and
  `governance_target_refs` fields. Empty legacy lifecycle reads as active
  without forcing old records to persist `active`; explicit `claims` can be
  supplied through the decision input-file path, with `predictions` retained as
  a compatibility projection. Decision reconciliation items include a
  read-only claim lifecycle summary so partial claim retirement/supersession is
  visible without retiring the whole DecisionRecord.
- **Overseer drift auto-baseline safety.** Maintenance planning now uses drift
  materiality and decision health before proposing deterministic rebaseline:
  only proven non-material churn can auto-resolve, while material symbol drift
  and unknown legacy file-scope drift remain operator-review work.
- **Golden E2E module cache reuse.** The golden CLI subprocess test now
  preserves the host Go module cache before switching `HOME` to a temp project,
  avoiding false timeouts from cold `modernc.org/sqlite` downloads while still
  keeping the build cache and project root isolated.
- **BaselineKind compatibility layer.** SpecSection lifecycle and
  approve/rebaseline/reopen JSON projections now expose
  `baseline_kind=spec_section_approval_baseline`, while legacy untyped baseline
  posture has an explicit `unknown_legacy_baseline` parser representation.
  This starts the baseline split without a storage migration or new compact
  status lane.
- **Typed baseline snapshot split.** Spec lifecycle writes now go through
  `SpecSectionApprovalBaseline`, while `PreWorkReferenceSnapshot` and
  `VerifiedStateSnapshot` are separate typed objects that cannot be written to
  `spec_section_baselines` through the normal store boundary. Drill-down
  lifecycle and mutation JSON include `baseline_profile` so approval baselines,
  work-reference snapshots, verified-state evidence, and unknown legacy
  posture remain distinguishable.
- **Carrier authority manifest.** Added `haft carrier manifest` /
  `--json` as an explicit drill-down over current authority, support,
  compatibility, provenance, archive, and sidekick carriers. The manifest keeps
  `/h-reason` as a current umbrella skill, marks Pi as compatibility packaging,
  and labels standalone/TUI/desktop/Open-Sleigh surfaces as non-current so they
  do not re-enter the product model through stale carriers.
- **Carrier semio guard.** Added `haft carrier check` / `--json` as a focused
  fixed-point wording check over current/support/compat carrier text. It fails
  dead standalone/TUI/desktop runtime-surface mentions that are not explicitly
  labeled dropped, archive, provenance, support, or not-current, while keeping
  default status free of carrier-manifest noise. MCP parity is available through
  read-only `haft_query(action="carrier_manifest")` and
  `haft_query(action="carrier_check")` drill-down actions, and `haft interface`
  documents both carrier query contracts as read-only, non-status surfaces.
- **MCP binding authority boundary.** MCP dispatch now fails binding governance
  acts closed by default with a structured `operator_confirmation_required`
  response: `haft_decision(action="decide")`, WorkCommission creation actions,
  `haft_spec_section(approve|rebaseline|reopen)`, and authority-changing
  `haft_refresh(waive|reopen|supersede|deprecate)` no longer treat
  model-supplied arguments as proof of operator authorization. Manual CLI/host
  paths remain the binding path; an additive authorization receipt model now
  names `manual_cli` as the only v1-valid receipt kind and marks MCP
  receipt-backed binding unsupported until a future host verifier exists. MCP
  tool/interface descriptions now state that boundary.
- **Binding surface inventory guard.** MCP binding-gate enforcement now reads a
  classified inventory of read-only, draft, evidence-recording, binding,
  lifecycle, and execution-authority surfaces. Focused tests keep known
  authority mutations covered by `operator_confirmation_required` and scan the
  README, target/enabling specs, h-decide/h-commission skills, and tool
  descriptions for wording that would imply model-supplied prompt text is proof
  of operator authorization.
- **Formality F0-F9 preservation.** Evidence formality now preserves the
  current FPF F0-F9 ordinal directly instead of folding F4-F9 values into the
  old F0-F3 scale. Legacy F0-F3 records remain readable as lower-scale values,
  while out-of-range inputs clamp only at the scale endpoints.
- **Versioned formality scale metadata.** Evidence items now carry explicit
  `formality_scale` / `formality_bridge` metadata alongside the legacy
  `formality_level` projection. New evidence writes use
  `fpf-2026-f0-f9`; legacy unversioned F0-F3 rows are surfaced as
  `haft-legacy-f0-f3` with a loss-bearing bridge instead of being silently
  promoted. EvidencePath records expose the scale while preserving the existing
  `not_approval` / `not_gate_decision` / `not_global_truth` authority boundary.
- **OperationalGate admission posture.** `require_current_source_and_admitted_use`
  now passes only for current-source stronger-use admission; documentary-only
  reading and temporary waiver admission remain admitted/waived for their own
  contexts but block derived gate passage.
- **Operator-facing Haft artifact transparency.** Agent-facing workflow,
  status, refresh, projection, maintenance, baseline, evidence, and measure
  responses now pair governance artifact refs with human-readable titles, for
  example `**Decision title** \`dec-...\`` instead of bare IDs. Linked
  ProblemCard/SolutionPortfolio/DecisionRecord refs in status and adoption
  flows resolve to titles when available, missing projection refs render as
  explicit untitled artifacts, generated `Variant.ID` values remain visible for
  direct compare calls, and malformed direct compare payloads now fail instead
  of being silently dropped.
- **Overseer review schema.** The structured-output schema for
  `haft overseer review` now requires every declared strict location property
  and drops optional location fields that the reviewer did not require,
  preventing strict JSON schema rejection before review execution starts.
- **ProblemCard semantic identity sync coverage.** `haft sync` tests now pin a
  richer DB -> Markdown carrier -> empty DB round-trip, covering typed
  ProblemCard profile fields, semantic edition pins, source-of-truth binding,
  and exact publication recoverability instead of only the minimal signal
  envelope.
- **Drift added-file noise.** Module-scope drift detection now lists current
  scope files through `git ls-files --cached --others --exclude-standard` when
  available, preserving modified/missing checks for baselined files while
  keeping nested-gitignored build output and local governance/runtime carrier
  directories such as `open-sleigh/.haft/` out of added-file drift.
- **Maintenance evidence cooldown date basis.** The maintenance plan cooldown
  now compares today's evidence using the same local date prefix that
  `AttachEvidence` writes into `evid-YYYYMMDD-*` IDs, avoiding duplicate
  machine-check tasks around local/UTC day boundaries.

### Added

- **Symbol-precise decision binding.** Decision baselines now resolve
  code-governing `affected_files` through an additive `binding_targets` model:
  supported languages prefer symbol targets, no-symbol source falls back to a
  stable range target, explicit module/whole-file scope is typed, and
  whole-file fallback records why narrower binding failed. Drift now separates
  unresolved whole-file fallback into a compact needs-binding lane instead of
  treating it as proved material symbol drift.
- **Legacy decision symbol-binding dry-run.** `haft drift bindings` now reports
  active decisions with broad file baselines and proposes read-only symbol
  binding candidates, separating high-confidence rebaseline proposals from
  ambiguous cases that still need explicit symbol-boundary selection.
  `--apply-high-confidence` persists resolver-proven binding targets when the
  safe set is non-empty, and `--apply-selection <json>` lets an agent apply a
  reviewed selection document deterministically without changing evidence,
  approvals, file hashes, or markdown carriers. Haft's own dogfood batches now
  bind six historical decisions to explicit symbol targets, reducing unresolved
  legacy selection cases from 35 to 29.
- **OperationalGate v1 for spec use.** `SpecificationUseRecord` can now include
  an explicit read-only OperationalGate profile and derived gate decision for a
  declared use context. CLI callers pass `haft spec use --gate-file <json>`;
  MCP callers pass `operational_gate` to `haft_query(action="spec_use")`. The
  first gate rule, `require_current_source_and_admitted_use`, passes only when
  the source edition is current, the admission policy grants the declared use,
  the bearer/use context match, and the gate is not expired. Gate decisions
  remain derived readings, not spec approval, evidence creation, or work
  authority.
- **Read-only maintenance judgment packet.** `haft_refresh(action="review")`
  and `haft overseer judgment --json` now build a grouped packet for rung-3
  maintenance tasks that need judgment. The packet classifies tasks by
  recommendation, confidence, source, and category, includes exact drill-down
  calls plus suggested command candidates, and explicitly labels itself as
  not mutation, not approval, and not evidence.
- **Bounded maintenance drain for `$h-verify`.** `haft_refresh(action="drain")`
  and `haft overseer drain` now run the existing maintenance executor only
  behind an explicit invocation: rung-1 deterministic drift may rebaseline,
  rung-2 allowlisted observables may attach machine evidence and revalidate,
  and all semantic/material/judgment cases return as `needs_operator`.
  `dry_run=true` / `--dry-run` previews the same report without mutation.
- **`haft init --hermes`.** Hermes initialization now materializes a
  Hermes-adapted haft skills tree with bare `haft_*` tool references, appends
  it to `skills.external_dirs` in `~/.hermes/config.yaml` (or
  `$HERMES_HOME/config.yaml` / `--profile`), and installs the `haft` MCP server
  with absolute `HAFT_PROJECT_ROOT` plus `HAFT_EXPECTED_PROJECT_ID` in the
  user-local Hermes config. Existing Hermes config keys, other MCP servers, and
  foreign external skill directories are preserved; the legacy `quint-code`
  MCP server key is removed.
- **ProblemCard semantic-spine first slice.** New ProblemCards now carry an
  additive semantic envelope in `structured_data` with explicit profile
  provenance, semantic-edition identity, carrier binding, reference scheme,
  publication projection, and `exact`/`legacy` status. `haft_query(action=
  "related", ref="prob-...")` keeps the legacy payload shape while adding
  `semantic` plus `views.working`, `views.exact`, and `views.audit`, and
  `haft interface query.related --json` documents the compact discovery
  contract.
- **Haft's own spec spine.** The repository now tracks `.haft/specs/`
  carriers for the target system, enabling system, and term map while keeping
  runtime governance state ignored. The active sections cover Haft's
  governance boundary, layered substrate architecture, artifact production
  rules, mutation authority, host-agent policy, commission/runtime policy, and
  evidence freshness policy.
- **ChoiceResult compatibility slice.** `DecisionRecord` structured data can
  now carry an exact `choice_result` with `next_move` values
  (`choose_now`, `reject_current_set`, `probe_again`, `reroute`), while
  `ComparisonResult.selected_ref` stays a legacy advisory recommendation.
  `legacy_recommendation_ref` is the preferred advisory alias for comparison
  output; compare never creates a bound human choice or execution authority.
- **Read-only spec semantic review.** `haft spec review` and the MCP
  drill-down `haft_query(action="spec_review")` now run an advisory semantic
  review over active `SpecSection` records. Findings return FPF-oriented hints
  for missing bearers, frame mismatches, unsupported strong claims, and
  description/authority confusion; the review is explicitly non-authoritative
  and does not create evidence, approvals, rebaselines, `GateDecision`, or
  `SpecUseAdmission`.
- **FPF retrieval provenance metadata.** FPF section retrieval now carries
  explicit profile/index provenance (profile id, source kind, source edition,
  source hash, profile validity, normativity, index schema version, and
  retrieval mode) through the internal retrieval path. `haft fpf search --json`
  exposes those fields structurally, while explain/exact text views name
  route/related carriers as non-normative projections over FPF sections.
  Default search output stays compact.
- **Spec ClaimRegister v1.** SpecSection carriers may now declare explicit
  `claims:` entries with stable IDs, L/A/D/E class labels, claim-scoped support,
  evidence, validity, and governing-pattern refs. `haft spec review --json`
  exposes the read-only claim register and advisory findings for mixed,
  unclassified, or unsupported declared claims; no prose is extracted into
  canonical claims.
- **ProblemCard PublicationUnit v1.** Exact ProblemCard semantic envelopes now
  carry source-edition pins, source/publication/carrier hashes, and explicit
  omission/loss/recoverability fields. `haft_query(action="related")` exact and
  audit views distinguish source episteme, publication projection, and carrier
  bytes; markdown sync rejects future semantic/publication-unit schema versions
  instead of importing them as exact.
- **Spec SystemReferenceFrame v1.** SpecSection parsing now derives a typed
  `system_frame` object from explicit `system_frame`, existing `spec`, or the
  carrier kind compatibility fallback. `haft spec review --json` reports that
  frame and frame diagnostics now compare declared target/enabling frame against
  the carrier frame instead of inferring authority from TS/ES or kind prefixes.
  The parser and review profile now also support typed `carrier` and `sidekick`
  frames, with `publication_system` and `external_system` normalized to those
  canonical frame kinds.
- **ProblemCard C.22.2 profile fields.** Problem framing now records
  cue/thin/deep profile level, source posture, why-now, scope, acceptance
  probe, freshness disposition, computed P2W readiness, and blockers. Wish,
  ticket, and chosen-method sources cannot become P2W-ready without an explicit
  boundary, and `query.related` working views expose the profile/readiness
  labels without changing compact defaults.
- **TransformationRecord v1 compatibility payload.** `DecisionRecord`
  structured data may now carry an explicit `transformation_record` describing
  the transformed entity, initial state, post state, relation, context/window,
  and outward method/work/evidence/publication refs. The field is validated and
  discoverable through `haft_decision` and `haft interface decision.decide
  --json`, but no record is synthesized from legacy prose, post-conditions,
  method runs, WorkCommissions, evidence, or publication units, and refs do not
  prove occurrence, approval, evidence truth, or publication.
- **ChoiceRecord C.11 compatibility fields.** The existing `choice_result`
  payload now carries explicit C.11 fields for subject, option set, comparison
  basis, choice rule, next move, reversibility, and reopen condition. Fresh
  h-decide DecisionRecords populate option/basis/rule fields from explicit DRR
  inputs and derive `reopen_condition` from rollback triggers when no explicit
  `choice_result` is supplied. Explicit `choice_result` carriers persist
  `reversibility` and `reopen_condition` through CLI/MCP paths; legacy or
  minimal carriers continue to parse as compatibility projections and no
  ComparisonResult `selected_ref` is promoted into a bound choice.
- **StateReadings v1 for spec semantic review.** `haft spec review --json`
  now emits per-section `state_reading` objects that name the reading profile,
  bearer, frame, use, reading, and reopen condition. The compact text summary
  names this qualified-reading policy instead of implying global
  ready/pass/current authority.
- **Spec semantic review v2 profile.** `haft spec review` now exposes a
  read-only `spec_semantic_review_v2` profile that names which semantic-spine
  inputs are used, boundary-preserved, or explicitly abstained. The review uses
  ClaimRegister, SystemReferenceFrame, and StateReadings signals, preserves
  PublicationUnit and TransformationRecord authority boundaries, and blocks
  high-risk licensing/legal/compliance/privacy/security sections from stronger
  use until explicit L/A/D/E claims and support refs exist. Findings now carry
  an explicit `category` such as `claim_posture`, `publication_boundary`,
  `frame`, or `unknown_abstain`, so agents can route review input without
  treating the finding as evidence, approval, GateDecision, or
  SpecUseAdmission.
- **SpecificationUseRecord v1.** `haft spec use SECTION_ID` and
  `haft_query(action="spec_use")` now build a read-only spec-use admission
  record for a declared use context and policy. The payload separates source
  edition, baseline currentness, admission policy, waiver expiry, and
  `gate_decision=not_applicable_no_operational_gate` so a current baseline
  cannot masquerade as admission and a spec-use admission cannot masquerade as
  an OperationalGate pass.
- **EvidencePath/RelianceDisposition v1.** `haft evidence path ARTIFACT_REF
  EVIDENCE_REF` and `haft_query(action="evidence_path")` now build a read-only
  reliance record for one existing evidence item and declared attempted use.
  The payload binds claim refs, producer/method/work trace refs, currentness
  windows, and authority boundaries; evidence presence cannot create approval,
  `GateDecision`, or global truth.
- **EngineeringChangeCase v1 projection.** `haft change case DECISION_REF` and
  `haft_query(action="change_case")` now derive a read-only case aggregate over
  one DecisionRecord's ProblemCard refs, TransformationRecord payload,
  ChoiceResult, evidence item refs, and optional EvidencePath records. The case
  is explicitly a projection, not a new FPF root kind, proof, approval,
  `GateDecision`, work occurrence, or global truth.
- **Qualified correspondence graph v1.** `haft correspondence graph
  DECISION_REF` and `haft_query(action="correspondence_graph")` now derive a
  read-only expected-vs-observed graph from DecisionRecord claims,
  TransformationRecord, affected files, and claim-bound evidence. Every edge is
  qualified by origin/source refs and `path_status=graph_path_not_proof`, so a
  graph path cannot masquerade as evidence, proof, approval, `GateDecision`, or
  global truth.
- **Semantic drift route v1.** `haft drift route DRIFT_KIND` and
  `haft_query(action="drift_route")` now map the semantic-spine drift taxonomy
  to read-only candidate repair actions and language-state moves. Evidence,
  publication, and description-layer drift do not route directly to code repair,
  unknown drift kinds fail closed, and the route never mutates code, carriers,
  evidence, decisions, baselines, or gates.
- **Blocked-use attention item v1.** `haft attention blocked BEARER_REF` and
  `haft_query(action="blocked_use")` now build a read-only object-first
  attention item with exact source-return refs, blocked-use context, admissible
  next-action invitations, and authority boundaries. The item is not a WorkPlan,
  evidence, approval, `GateDecision`, or global truth, and it is available only
  through explicit drill-down surfaces.
- **Haft engineering-value characteristic space v1.** `haft value space
  BEARER_REF` and `haft_query(action="value_space")` now build a read-only
  `HaftEngineeringValueECS` projection with no single Haft/FPF score. Every
  characteristic names bearer, method, window, denominator, evidence refs,
  protected trade-offs, and reopen condition; healthy reopening is not counted
  as simple failure, and missing evidence blocks value claims instead of
  fabricating movement.
- **Repository-move audit and repair in `haft doctor`.** `haft doctor
  --moved-from <old-root>` now reports stale project-root carriers after a
  checkout move, and `--repair` performs an explicit exact-match repair for
  supported carriers. The repair path covers Codex user project trust entries,
  Claude user project state exact JSON string literals, project-local MCP
  configs, and OpenCode config; default `doctor` remains read-only and
  `--repair` is rejected unless the old root is supplied.
- **Project-id guarded binding diagnostics.** MCP host configs produced by
  `haft init` now include `HAFT_EXPECTED_PROJECT_ID` alongside portable
  `HAFT_PROJECT_ROOT` values where the project identity is available, and
  `haft serve` resolves binding through the shared ProjectBinding path before
  exposing mutation-capable tools. `haft doctor` now reports the resolved root,
  project id, expected id, DB path/state, and artifact count so moved checkouts
  can distinguish "wrong cwd" from "wrong project identity".
- **`@haft/pi`: a Pi-native behavior package (first carrier slice).** New
  `packages/haft-pi` ships a Pi extension that owns a small NDJSON MCP bridge
  to `haft serve` and registers the full typed tool suite natively
  (`haft_query`, `haft_problem`, `haft_solution`, `haft_decision`,
  `haft_note`, `haft_refresh`, `haft_method`, `haft_commission`,
  `haft_spec_section`). The package carries `/h-*` prompt templates
  (including manual-gate `/h-decide` and `/h-commission`), Agent Skills for
  the MethodPack loop (`h-method`) and the FPF knowledge layer
  (`fpf-development`, `fpf-semiotics`), a `before_agent_start` prompt
  governor fed by the kernel's governor projection, and cockpit surfaces
  (session widget with overseer/decision counts and the open method run,
  footer status per tool call). The bridge handles concurrent starts, spawn
  failures with retry, and reports tools missing from older `haft` binaries
  with an explicit upgrade message. Out of scope for this slice:
  npm publish metadata, custom TUI renderers, active-tool lanes, and
  provider-payload interception.
- **`haft init --pi`.** The `@haft/pi` package is embedded into the haft
  binary; `haft init --pi` materializes it under `.haft/pi/haft-pi` and
  idempotently registers the local-path entry in `.pi/settings.json`
  (project-local, loads after Pi project trust). The entry is written as
  `../.haft/pi/haft-pi` because pi resolves project-scope local package
  paths relative to the `.pi` directory itself; a broken root-relative
  entry from earlier builds is migrated in place. `--pi` counts as a host
  selection, so it does not drag in the default Claude config. No npm step:
  the extension's only runtime import (`typebox`) is a Pi-bundled core
  package resolved via `peerDependencies`.
- **Kernel governor projection: `haft_query(action="status",
  view="governor")`.** A compact, prompt-budgeted status slice for host-side
  prompt governors: overseer signal counts,
  pending/unassessed/refresh-due/drift decision counts, top attention items,
  active problems, and open method runs — capped lists, no coverage section,
  no navigation strip. Replaces client-side clipping of the full dashboard
  in host integrations.
- **Opt-in overseer post-commit review loop.** New `internal/overseer`
  subsystem (packet/run/finding lifecycle, queue, maintenance classifier,
  risk heuristics) with `haft init --overseer` and `haft overseer` commands:
  a soft post-commit hook builds review packets, a local daemon runs reviews
  through the project's host agent, and overseer signals surface read-only
  in `haft_query(action="status")`. `haft init --no-claude-md` is renamed to
  `--no-file-instructions` (the old flag remains as a deprecated alias).

- **MethodPack V1: task-local SWE method gates.** Added the built-in
  `swe-core` method catalog with seven compact methods:
  `verification-before-completion`, `systematic-debugging-before-fix`,
  `behavior-first-testing`, `refactor-only-under-tests`,
  `domain-port-before-adapter`, `functional-core-imperative-shell`, and
  `make-illegal-states-unrepresentable`. New `MethodRun` artifacts use the
  `mpull-*` prefix and persist under `.haft/method-runs`. The new
  `haft_method` MCP tool supports `pull`, `close`, `show`, `detail`, and
  `status`; `pull` creates compact task-local cards and `close` validates hard
  gate evidence or explicit waivers before completion. `haft init`
  materializes `.haft/methods/swe-core` by default, `haft interface` exposes
  compact method contracts, generated instructions route agents through the
  pull/close loop, and MCP initialize instructions now advertise the MethodPack
  habit trigger before code work.
- **Compact Haft interface discovery and input-file artifact creation.**
  `haft interface <capability> --json` now exposes small machine-readable
  contracts for agent workflows, and `haft artifact create <capability>
  --input-file <input.json> --json` can create/update ProblemCards,
  SolutionPortfolios, DecisionRecords, and Notes via the same artifact core as
  MCP. Supported capabilities: `problem.frame`, `solution.explore`,
  `solution.compare`, `decision.decide`, and `note.record`.
- **Read-only overseer status command.** `haft overseer status` now renders
  the latest overseer signals and autonomous-maintenance ledger without
  re-running the maintenance loop; `--json` exposes the same snapshot for
  tooling.

### Changed

- **Markdown carriers round-trip structured artifact data through `haft sync`.**
  Artifact markdown projections now include a hidden `structured_data` carrier
  block when structured JSON exists. The shared artifact parser extracts that
  block back into SQLite on `haft sync`; legacy ProblemCard markdown without an
  envelope imports as `legacy` with audit warnings instead of being promoted to
  exact v3 semantics.
- **Default `h-status` is now an operator cockpit.** `haft_query(action="status")`
  renders capped high-signal lanes for operator attention, active work,
  decision-health counts, and a one-line coverage cue; detailed status remains
  available through `full=true`, full module coverage through
  `haft_query(action="coverage")`, drift/stale detail through
  `haft_refresh(action="scan", verbose=true)`, and maintenance work orders
  through `haft_refresh(action="plan")`. MCP initialize instructions, the
  bundled `h-status` skill, the interface catalog, Pi carriers, and installed
  instruction templates now spell out that compact omission is not evidence of
  absence.
- **MCP defaults now stay compact by default.** `tools/list` trims
  non-load-bearing schema descriptions and points agents at `haft interface`
  for full contracts; `haft_refresh(action="scan")`, `haft_query(status)`,
  and `haft_query(code_context)` now return capped summaries unless callers
  explicitly request `verbose=true` or `full=true`.
- **Bundled haft skills now teach the compact CLI path.** The frame, explore,
  compare, decide, note, reason, and status skills route agents through
  `haft interface` and input-file artifact creation instead of inlining long
  schemas; MCP write tools remain the compatible fallback.
- **MCP initialize instructions surface autonomous maintenance explicitly.**
  Agents are now told to relay autonomous-maintenance actions, undo commands,
  and typed maintenance work orders instead of silently absorbing overseer
  changes.
- **Bundled FPF corpus refreshed.** The vendored `data/FPF` pointer and baked
  `internal/cli/fpf.db` were regenerated for the upstream ontic/transformation
  vocabulary update.

### Fixed

- `haft init` instruction carriers restore the peer-engineering communication
  style from the original `CLAUDE.md`: dry technical humor is allowed when it
  helps, and agents are told to sound like pairing engineers rather than
  executive-presenting bots. Fixes [#92](https://github.com/m0n0x41d/haft/issues/92).
- Shared `haft-embed` daemons no longer inherit the first client project's
  working directory; the launcher now starts them from the private socket
  directory and resolves configured cache paths to absolute paths before launch.
- Shared `haft-embed` daemons no longer retain multi-gigabyte allocator arenas
  indefinitely after cold corpus warms: hybrid recall batches corpus embedding
  misses, the Rust sidecar disables the ORT CPU memory arena by default
  (`--cpu-arena` restores the old allocator mode), and socket daemon idle
  timeout now tracks active embedding requests rather than open idle clients.

## [8.1.0] — 2026-06-06 — shared embedding runtime

Architectural pivot recorded in
`.haft/decisions/dec-20260525-v8-architecture-pivot-from-standalone-agent-to-g-bbe45cb7.md`.
Standalone interactive agent (`haft agent`), TUI (Bun/OpenTUI/SolidJS package
from the prior 8.0.0 release), and desktop wrappers (Tauri / Wails) **dropped**.
Haft becomes a governance substrate: kernel + CLI + MCP server + 15 skills,
plugged into Claude Code / Codex / OpenCode / Cursor over their native skill
+ slash-command surfaces. See [MIGRATION-v8.md](MIGRATION-v8.md).

### Removed

- **Artifact-based FPF "semantic search" prototype** (`haft fpf semantic-search`
  / `haft fpf semantic-index` and the `.json.gz` semantic artifact). Superseded
  by the baked-vector FPF hybrid (see Added); the dead command surface and the
  `BuildSemanticArtifact` / `SearchSpecSemantically` path are gone. The live
  `haft fpf search` / `section` / `info` commands are unchanged.
- `haft agent` standalone interactive agent — all of `internal/agentcore`,
  `internal/agentdriver`, `internal/agentproto`, `internal/agentserver`,
  `internal/agentstore`
- v8 TUI package (`tui/` — Bun + OpenTUI + SolidJS bundle, ~46k LOC dropped)
- Wails-era desktop frontend (`desktop/frontend/` — React app for the
  prior Wails wrapper). `desktop-tauri/` and `tui-react/` trees are
  dead-code but still git-tracked pending operator decision on full
  removal (no Go code launches either of them in v8).
- Orphan packages post v7-agent removal: `internal/agentloop`
  (coordinator, overseer, spawn — ~5400 LOC), `internal/protocol`
  (bus, commands, events), `internal/session` (sqlite store +
  migrations + tests), `internal/setup`, plus CLI helpers:
  `login.go`, `models.go`, `setup.go`, `session_mode.go`,
  `message_projection.go`, `files.go`, `term_echo_*.go`,
  `internal/tools/ask_user.go`
- v7 helper commands: `haft login`, `haft models`, `haft setup`
- `/h-reason` umbrella skill — replaced by the 15-skill catalog
- Prior `[8.0.0]` architecture (TS+Bun+OpenTUI+SolidJS standalone TUI,
  gradual deprecation in 8.0 + removal in 8.1 plan) superseded by the
  May 25 pivot DRR per FPF reasoner critique (BLP violation confirmed)

### Added

- **Shared Rust embedding sidecar daemon.** Local embeddings now use one
  per-user/per-model `haft-embed` process instead of one model process per
  `haft serve` instance. The Go embedding adapter first connects to a
  user-owned Unix socket daemon keyed by binary/model/cache/dim; the first
  client starts it under a lockfile, later Claude Code / Codex / MCP sessions
  reuse the same loaded ONNX model. The socket directory is private to the
  current user, sockets are `0600`, startup races are serialized, and the daemon
  exits after an idle timeout so memory is released when agents stop using it.
  The old stdio child-process adapter remains the fallback path.
- **Code-graph retrieval — codegraph-parity lexical heuristics.** `haft_query`
  symbol seeds and the `search` action now tolerate typos (bounded edit
  distance), split compound identifiers (`getUserName` → get/user/name), stem
  query terms, and accept field qualifiers (`kind:`/`lang:`/`path:`/`name:`) —
  closing the grep-to-find-a-seed fallback. Deterministic, no embeddings.
- **Graph-proximity recall in `related`.** A "Related by graph proximity"
  section ranks symbols / decisions / notes by distance in the fused
  code+reasoning graph via deterministic Personalized PageRank (no embeddings,
  no second runtime). Held-out link-prediction on the real graph measured
  recall@10 ≈ 1.8× a name-lexical baseline.
- **"Tested by" coverage lane in `related`.** For a file, surfaces which test
  functions exercise each callable symbol (structural coverage via call
  edges — "exercised by", not "verified") and flags exported symbols no test
  reaches; the proximity section stays production-only.
- **Multi-language structural code edges.** The code graph grew beyond Go
  call/dispatch to structural relationship edges across three languages: Go
  `implements` / `embeds`, Python `extends` (class inheritance), and
  TypeScript/JavaScript `extends` / `implements` (from explicit heritage
  clauses). Unresolved or external targets are dropped, never invented; each
  edge surfaces in callers/callees/impact with its kind label. (Resolution is
  directory-scoped for now — cross-module imports are a follow-up.)
- **Fact memory in `haft_note`.** `haft_note` is now a fact carrier: record
  atomic `observations` (rationale optional) and anchor a fact to
  decisions/problems/notes via typed `anchors`, persisted as real graph edges
  that surface in `related` / backlinks. A dead anchor (missing target) is
  rejected, never silently kept. Anchors now also accept **code symbols**
  (`Name` or `Name@file` to disambiguate): a fact attaches to the exact symbol
  and surfaces at it in `code_context` / `node` — the same fusion payoff
  artifact anchors give. Symbol resolution lives in the CLI shell (the artifact
  core never imports `codebase`); a dead or ambiguous symbol anchor rejects the
  whole note, the same no-dead-edge invariant in both directions
  (`dec-20260604-26be1e4b`).
- **`haft init` installs/updates project `CLAUDE.md` haft section** — new
  step in the init flow. Writes a haft-managed section delimited by
  `<!-- haft:start -->` / `<!-- haft:end -->` HTML-comment markers.
  Idempotent: re-running `haft init` replaces content inside the markers
  and preserves any operator-authored content outside. Opt-out via
  `haft init --no-claude-md`. Template embedded into the binary from
  `internal/cli/claude_md_template.md`; the same content is mirrored
  between haft markers in repo-root `CLAUDE.md`, with drift caught by
  `TestClaudeMDTemplateInSyncWithRepoRoot`.
- **CLAUDE.md showcase template** carries the new "Description ≠ Work"
  core rule, a self-check pattern before long responses, friction-tradeoff
  explainer (why kernel-persistence is worth the in-the-moment cost),
  canonical FPF flow diagram, skill catalog with mode classification,
  Quick Decision Framework for small reversible choices, Communication
  Style calibration table, Thinking Principles, Critical Reminders, and
  FPF Glossary. End-users get this on `haft init`; haft maintainers see
  the same content between markers in repo-root CLAUDE.md.
- **15-skill v8 catalog** installed by `haft init`:
  `h-reason` (umbrella), `h-frame`, `h-diagnose`, `h-explore`, `h-compare`,
  `h-decide`, `h-verify`, `h-status`, `h-onboard`, `h-spec-cover`, `h-note`,
  `h-commission`, `h-abduct`, `h-boundary-unpack`, `h-semio-review`.
  Auto-triggering skills fire on operator context. `h-decide` and
  `h-commission` are manual-only (`disable-model-invocation: true`)
  per Transformer Mandate.
- **`haft check routing`** — CI-friendly golden-prompt routing reliability
  check. 40 cases pairing operator-style prompts with expected skills;
  enforces 70% pass threshold from pivot DRR prediction.
- **Kernel MCP hard gates** — `haft_decision(action="decide")` validates
  required DRR fields server-side and returns structured errors with
  FPF spec references (CMP-02, DEC-08, X-WLNK, CMP-04, DEC-05) plus
  how-to-proceed sections. Tactical mode supports explicit `_skips` +
  `_skip_reason` field bypass.
- **`h-diagnose` parallel hypothesis testing** — spawns one Agent
  subagent per hypothesis to prevent the LLM's natural anchoring bias.
  Forces 3+ rivals per FPF CC-B.5.2-2.
- **`h-compare` dim-wise parallel scoring** — spawns one Agent subagent
  per comparison dimension scoring all variants. Parity plan and
  selection policy declared BEFORE scoring (Anti-Goodhart).
- **MIGRATION-v8.md** — v7→v8 migration guide with upgrade checklist,
  behavioral-change reference, and rollback procedure.
- **`Warnings []string` on `ToolResult`** — Slice B warning detectors
  for h-explore (diversity check), h-compare (parity hints), h-decide
  (DRR completeness hints) preserved from pre-pivot work.
- **Inline FPF-discipline guards (kernel, advisory — never block)** — three
  deterministic checks adopted from FPF `16cd313`, surfaced as soft warnings
  so the agent self-corrects while the human stays final authority
  (Transformer Mandate):
  - **Umbrella-word frame guard** (FPF E.10 wording-use precision) —
    `haft_problem(action="frame")` scans title/signal/acceptance against a
    curated EN+RU trigger registry (`quality`, `robust`, `scalable`, `clean`,
    `ready`, `secure`, …) and names, per word, the precise recovery and the
    overread to block. `internal/artifact/umbrella.go`.
  - **Content-vs-reputation decide guard** (FPF E.9.DA:4.4b) —
    `haft_decision(action="decide")` flags rationale leaning on popularity /
    adoption / "industry standard" / "best practice" and asks for the content
    reason that makes the option right for *this* problem.
    `internal/artifact/reputation.go`.
  - **Non-discriminating dimension warning** (FPF A.19.ECS) — `compare`
    flags a TARGET dimension on which every variant scores identically (dead
    weight, or a hidden parity condition mislabeled as a target); role-aware,
    so constraints (all-pass is correct) and observations are skipped.
    `internal/artifact/solution.go`.
- **`/h-spec-cover` impact-ranked coverage (V1)** — uncovered-file output
  ranks modules by impact instead of a flat list (`dec-20260527-e4b86938`).
- **Code drift surfaced in `/h-status` (H1)** — `haft_query(action="status")`
  reports decisions whose affected files changed since baseline, so drift is
  visible without a manual `/h-verify`.
- **Auto-baseline disposition floor — safety classifier** (`dec-20260606-9b4a4c52`).
  Pure deterministic classifier (`internal/artifact/autobaseline.go`) that maps
  each drift report to one disposition by keying off the existing symbol-level
  `SymbolVerdict`: provably-additive drift → auto-resolve-silent; a
  modified/removed governed symbol (or deleted file) → stage-for-confirm;
  anything unprovable (no baseline / unanalyzable) → surface-for-review.
  Conservative by construction — a governed-symbol change can **never** be
  silently auto-baselined, regression-pinned against the seeded harness-drain
  removed-func case. This release ships the tested safety core only; acting on
  the silent bucket (re-baseline + snapshot history for reversibility), the
  staged confirm-digest surface, and the callee-closure monitoring extension are
  deferred slices — the closure step waits on cross-package code-graph edges
  (`note-20260606-90688835`).
- **Context graph — code intelligence fused with the reasoning graph**
  (`dec-20260603-5825abc6`, plan `note-20260603-7d6632f1`). A code-graph
  (symbols + call/dispatch edges) built on the existing tree-sitter substrate,
  where every tool interleaves code structure with the decisions, problems,
  variants, notes, and invariants governing it — the fusion no pure code-graph
  can do. Functional core / imperative shell throughout; an unresolved
  call/dispatch is an **absent** edge, never a wrong one ("partial coverage
  worse than none"). New `haft_query` actions (cross-host over MCP):
  - **`code_context`** — the full reasoning context for a file or a symbol
    within it. Line-aware fusion join (`SearchByAffectedSymbolAt` +
    `Target.Line`) disambiguates overloaded same-name symbols by body line-range,
    so a decision attaches to the right overload instead of the union of all of
    them; granularity is reported honestly (`line-precise` vs
    `file+name (overload not disambiguated)`). A module-governed symbol shows
    `module governed by dec-Y`, never blank. `internal/contextgraph`.
  - **`callees` / `callers` / `impact`** — bounded directed traversal over the
    code-edge layer (depth ≤10, default 2, result ≤20), each reached symbol
    annotated with BOTH symbol-level (line-aware) and module-level governing
    decisions; ambiguous seed names list candidates rather than silently
    picking one. `internal/codeintel` (pure BFS over an `EdgeSource` port).
  - **`node`** — a symbol's detail page: byte-exact body **re-read + re-hashed**
    from disk before slicing (never stale source on an actively-edited file),
    its immediate caller/callee trail, and ALL same-name overloads each with
    their own per-overload governance + member outline for container types.
  - **`explore`** (PRIMARY) — the single-call answer: deepest connected call
    chain (≤7 hops, ≤1 dispatch bridge), each on-chain symbol fused with its
    governing decisions/invariants, plus blast radius (callers + covering
    decisions) and verbatim freshness-checked seed source — enough to answer
    "how does this flow and why is it shaped this way" at 0–1 Read. A chain
    that hits a dynamic-dispatch boundary it cannot resolve says so rather than
    implying completeness.
  - **Language-pluggable seam (not Go-locked)** — `EdgeResolver` / `SymbolView`
    ports + a per-language adapter registry; Go is the first adapter
    (`internal/codebase` call-site, cross-file, and interface→impl dispatch
    resolution). Other languages add an adapter; node extraction already spans
    7 languages. Interface dispatch resolves through `:=`-inferred receivers,
    not only declared ones (package-scoped type-fact inference), with
    conservative drop on any ambiguity so a shadowed or cross-package binding
    never produces a wrong edge. Concrete cross-package method calls resolve to
    the receiver type's method via package-scoped type facts; a symbol with no
    recorded callers says so honestly rather than returning a bare empty result.
- **Context-graph self-refresh (staleness detection + auto-rebuild)** — the code
  index detects when the source tree changed since its last build (fingerprint
  compare) and rebuilds automatically: lazily on a `haft_query` that finds the
  index stale, and on MCP server startup. Traversal / `explore` results are never
  served from a stale index waiting on a manual rebuild.
- **MCP server-instructions doctrine + mandatory session-start status** — the
  MCP `initialize` `instructions` field now always carries, in order, (1) a
  mandatory first-action rule — run `haft_query(action="status")` at session
  start, because a governed project's working memory lives in the graph, not
  only the code — and (2) a consult-before-editing doctrine for the fused code
  graph (tool-by-intent guidance, honest-coverage markers, anti-patterns).
  Delivered through the one always-on instruction channel; no CLAUDE.md
  duplication. `composeServerInstructions` (`internal/cli`), fires
  unconditionally.
- **Fuzzy-tolerant seeds + multi-seed `explore` (no new tool, no embeddings)** —
  folded into the EXISTING `haft_query` actions, determinism preserved. A seed
  name with no exact match falls back to deterministic substring ranking
  (exact > prefix > shorter): exactly one fuzzy match is used and **labeled**
  fuzzy; more than one returns candidates rather than silently picking. `explore`
  additionally accepts a bag of 2+ names in the same `symbol` argument and finds
  the connecting static path between adjacent seeds (bounded BFS, both
  directions); a disconnected pair is reported as "no static path", never
  bridged. (`SearchSymbols`, `resolveSeed`, `ExploreBag`.)
- **Curation gate for agent-drafted rationale** (`dec-20260603-732219b6`) —
  `/h-explore` and `/h-decide` (skill bodies + the `haft_solution` /
  `haft_decision` tool descriptions) bucket the agent's drafted
  weakest-links / risks / strengths by the agent's own confidence and surface the
  **doubtful ones first**, so the operator scrutinizes the load-bearing-but-
  uncertain arguments instead of rubber-stamping a flat wall. Honesty invariant:
  a low-confidence point is never down-ranked to look tidy. No kernel/schema
  change — the gate lives in guidance, not validation.
- **Ceremony auto-scaler — risk-floor + ask-when-blind**
  (`dec-20260604-…0a6edafd`). New `haft_query(action="ceremony", files=[…])`: a
  deterministic floor recommends a ceremony mode (tactical / standard / deep)
  proportioned to a change's **risk**, detected from path/content patterns +
  recorded governance facts — **never from call-graph fan-out** (a "leaf" is
  never assumed safe; a one-line auth change is high-risk). Hard invariant: a
  detected high-risk change (irreversible / security / privacy / public-API /
  data / financial-authz / destructive content / low-reversibility governance)
  is **never** routed tactical; when the floor cannot classify a touched file it
  escalates and asks rather than defaulting low. Recommendation only — the
  operator binds the mode (Transformer Mandate). Pure core in `internal/ceremony`
  (functional core / imperative shell), wired into `/h-frame`, `/h-explore`,
  `/h-compare`.
- **Hybrid semantic recall over FTS5 + PPR (optional EmbeddingGemma
  sidecar).** `haft_query(action="search")` can now fuse keyword (FTS5) and
  semantic (embedding cosine) recall via Reciprocal Rank Fusion (k=60, 0.15
  cosine floor) over the decisions+notes corpus — the graph stays primary and
  the layer **augments, never replaces** FTS5+PPR, degrading silently to
  keyword+graph recall when the embedder is absent. Embeddings come from an
  optional out-of-process Rust sidecar (`embed-sidecar/` — `haft-embed`,
  EmbeddingGemma 768-dim via fastembed-rs, newline-JSON stdio); haft's own Go
  build gains no new cgo. Hexagonal port-adapter (`internal/embedding` —
  `local` | `openai` | `none`), brute-force in-memory cosine index (no vector
  DB), corpus vectors cached in `artifact_embeddings` (migration v29) keyed by
  model contract + content hash. `install.sh` delivers the sidecar to
  `~/.haft/runtimes/` (OPTIONAL — a missing Rust toolchain warns and
  continues); `config.embedding` controls provider/model/dim,
  `embedding.provider=none` disables. Implements `dec-20260605-fe77b358` (Rust
  fastembed-rs sidecar) and closes the spike gate `dec-20260604-3aaad199`. A
  live R@k eval over the real `.haft/decisions` corpus (16 paraphrased queries)
  measured hybrid beating FTS5-alone by +25% R@10 (0.75 → 1.00), MRR +0.319 —
  well above the decision's 10% threshold. The sidecar is **self-healing** — a
  mid-session fault (crash / timeout) respawns the process once and retries, so a
  fault costs one query, not FTS for the rest of the session. The corpus index
  warms in the **background** (search returns FTS5+PPR immediately while cold and
  during re-warms — never blocking on a first-run model download), and a
  created/updated decision **invalidates** it so it becomes semantically
  searchable the same session.
- **Cross-project semantic recall at the index boundary.** The cross-project
  decision recall surfaced on `/h-frame` (the `Cross-Project History` section)
  was FTS5-only and paraphrase-blind — a frame whose problem reworded a prior
  cross-project decision missed it. It now reuses the embedding layer at the
  boundary. `IndexStore.Search` gains an **OR-fallback** (AND-first,
  OR-on-shortfall) that recovers paraphrased recall even without embeddings; a
  `CrossHybrid` (`internal/project`) fuses **AND-precise** FTS with embedding
  cosine via RRF over the cross-project corpus, **degrades byte-identically** to
  `IndexStore.Search` when the sidecar is absent, and guarantees hybrid recall
  **≥** FTS (additive invariant, via a tail recall-floor top-up). Corpus vectors
  are cached in `global_embeddings` on the index DB (additive; drop = recompute).
  A live R@k eval on the real cross-project corpus measured R@10 0.00 (old
  AND-only) → 0.75 (OR-fallback) → 1.00 (hybrid), MRR → 1.00. Implements
  `dec-20260605-8096a563`; the shipped per-project recall is untouched and
  `internal/project` gains no `recall` import (no cycle).
- **Semantic recall over the FPF spec itself (baked into `fpf.db`).** FPF spec
  retrieval is now hybrid-by-default: a reworded "how do I think about X"
  question recovers the right pattern by meaning, not just keyword. Per-section
  vectors are **baked into the embedded `fpf.db` at index time** (committed,
  deterministic — end users never embed the ~5700-section spec), keyed by
  `section_id` + model contract (`fpf_embeddings`, spec schema v3). The runtime
  fuses section-level FTS5 with **two SEPARATE cosine arms — pattern cards and
  spec prose — plus a card-priority merge**: a single pooled index let the
  ~5600 prose sections drown the 66 thinking-pattern cards (measured card R@10
  collapse 0.88 → 0.24), so cards and prose are ranked in their own
  populations. On paraphrased queries this recovers both — pattern-card R@10
  **0.24 → 0.88** (MRR 0.728) and spec-prose R@10 **0.00 → 0.47** over
  deterministic FTS. Degrades byte-identically to the deterministic tier+FTS
  path when the sidecar or baked vectors are absent. CI only **verifies** the
  committed index is fresh + vector-baked (`indexer -verify`); the heavy CPU
  bake runs on the maintainer's machine, never on a runner.
- **Decomposed-Brier calibration on decision predictions.** A decision
  prediction can now carry an optional `probability` in `[0,1]` (sampled as
  2–3 noisy votes, never one authoritative number); verified outcomes feed a
  Murphy (1973) Brier decomposition (reliability − resolution + uncertainty),
  so `/h-verify` reports whether the project's forecasts are over- or
  under-confident. Pure deterministic core (`internal/reff/calibration.go`),
  additive storage (probability rides the existing `structured_data` JSON — no
  migration), scored only at decide/verify time and honest about cold-start
  below 15 forecasts. Implements `dec-20260603-c3c7fa88`.
- **Cross-module `call` edges for Python and TypeScript.** Python and TS were
  structural-only (`extends`/`implements` from in-file heritage); they now also
  emit `call` edges resolved across modules — Python `from M import N`,
  module-qualified `m.foo()`, relative imports, and class construction; TS named
  + namespace imports with base-path + extension/index resolution. Closes the
  "directory-scoped for now" follow-up flagged on the multi-language edges entry
  above. Honest coverage held end to end: every unresolved, ambiguous, external,
  or instance-method (`obj.method()`) call is a **dropped** edge, never a guessed
  one (`dec-20260603-5825abc6`, extend-in-place).
- **Dynamic-dispatch callback edges + TS module resolution.** A new heuristic
  `callback` edge wires a named function passed to a callback **sink**
  (`emitter.on("x", onX)`, `signal.connect(handler)`, `addEventListener`,
  `subscribe`, …) from the enclosing function to the handler, closing the
  "callback-only function shows zero callers" hole; a function passed as plain
  data is never wired (exactly-one-or-drop). Intra-file EventEmitter dispatch is
  paired by event name (`.on`/`.once`/`.addListener` ↔
  `.emit`/`.fire`/`.dispatchEvent`) so "what runs when this emits" exists in the
  graph. TS module resolution learned `tsconfig`/`jsconfig` `paths` + `baseUrl`
  (JSONC-tolerant) and npm workspaces, with the per-project cache invalidated on
  a config-file mtime change. Plus polish: each Py/TS file is tree-sitter-parsed
  once per pass (was 4×), and generated files (`*.pb.go`, `*_pb2.py`, `*.gen.go`,
  …) are down-ranked behind hand-written code on a name collision.
- **Trust-decay signal on governing decisions in `code_context` (V2).** Each
  decision in the "Decisions governing this code" block now surfaces how far it
  can still be trusted — its non-active status (refresh-due / superseded) plus
  how many of its predictions remain unverified (`· N/M predictions
  unverified`) — so a decision whose claims were never checked no longer reads
  as fully authoritative right where the agent reads the *why*. Pure over the
  stored claims, no extra DB fetch; full evidence-based R_eff remains a
  follow-up.
- **Graph-grounded `h-diagnose` and `h-frame` (V3).** `h-diagnose` gained a
  Step 2.5 that pulls `code_context` once for the failing symbol and injects its
  invariants + trust-decay into each (read-only) hypothesis subagent, making a
  **stale governing decision** a first-class root-cause suspect that
  reasoning-on-code-alone systematically misses. `h-frame` gained an optional
  `seed_file` that runs the shipped PPR over the fused graph and appends the
  nearest governing artifacts to its keyword recall. Both live in the shell
  (best-effort, non-regressing). A 12-agent design workflow refuted adding the
  same hook to `h-explore` (it would anchor the diversity that is explore's job).

### Changed

- **Embedding startup is lazy and shared.** `haft serve` no longer prewarms the
  FPF, per-project, or cross-project embedding layers just because an MCP client
  connected. The model is loaded only when a semantic query actually needs it,
  and multiple live `haft serve` processes share that one Rust sidecar when the
  shared socket path is available. On any shared-daemon setup fault, recall
  still degrades through the existing optional-embedding contract rather than
  hard-failing.
- **Embedding sidecar defaults to int8-quantized EmbeddingGemma.** The local
  `haft-embed` sidecar now defaults to `embeddinggemma-300m-q`: measured
  retrieval parity with the fp32 model (FPF card R@10 0.88, MRR ~0.73) at a
  **~304 MB first-use model download instead of ~1.1 GB**. Applies to every
  embedding consumer (FPF spec, per-project and cross-project recall); the baked
  FPF vectors carry the same model id so the runtime contract matches. Selectable
  via `config.embedding.model` (`embeddinggemma-300m` for fp32, `-q4` for 4-bit).
- **`/h-fpf` renamed to `/h-reason` and expanded into a full FPF
  reasoning umbrella.** The v8 pivot had split the old `/h-reason` into
  15 specialized skills + a narrow `/h-fpf` fallback, betting that
  description-based auto-trigger would route reliably. Session-data
  analysis showed that bet was partly wrong: in practice operators
  invoke skills via explicit slash commands (`/h-status`, `/h-verify`,
  `/h-explore`) more than auto-trigger ever fires from natural language,
  and the umbrella entry point was missed by users with v6/v7 muscle
  memory. The new `/h-reason` carries the full reasoning palette in one
  place — framing, exploration, comparison, verification, notes, plus
  the slideument patterns that don't have dedicated skills (Goldilocks
  problem selection, NQD discipline, Bitter-Lesson Preference,
  Scaling-Law Lens, stepping stones, Anti-Goodhart indicator roles).
  Description is broad enough to also fire as fallback on ambiguous
  "let's think this through" / "structured approach" / "apply FPF here"
  signals. Specialized skills remain — they auto-trigger on sharper
  signals and carry deeper procedures; `/h-reason` covers their
  compressed inline versions and delegates to them for thoroughness.
  Old `/h-fpf` skill directory is now in `deprecatedSkillDirs` and gets
  cleaned up on `haft init`.
- **Skill descriptions rewritten per Anthropic SOTA best practices** —
  third-person, pushy ("Make sure to use this skill whenever..."), verbatim
  user trigger phrases instead of FPF pattern IDs. Counters Claude's
  documented tendency to undertrigger skills (per Anthropic's
  skill-creator playbook). Particular focus on `h-frame`, which now
  catches the high-value moment when an operator proposes a refactor /
  rewrite / migration without first naming the problem or acceptance
  criteria. Subroutines (`h-abduct`, `h-boundary-unpack`,
  `h-semio-review`) marked `INTERNAL SUBROUTINE` in description so the
  model knows not to surface them directly to operators.
- **`h-onboard` procedure refactored — agent drafts, operator reviews.**
  Previous procedure pushed operators to author the three spec carriers
  (`target-system.md`, `enabling-system.md`, `term-map.md`) themselves,
  with the rationale "they need to feel it." This misinterpreted
  Transformer Mandate (which only governs binding choices, not
  descriptive observation) and defeated the purpose of having an AI
  agent. The new procedure: agent reads README, project-config files
  (`package.json` / `pyproject.toml` / `Cargo.toml` / `go.mod`), source
  entry points, CI files, and drafts initial spec carriers directly to
  disk via the `Write` tool. Each file starts with a `DRAFT` HTML
  comment so the operator sees it as a starting point, not authoritative.
  Operator reviews and edits where the agent inferred wrong. The
  "autonomy default" question was also removed — that belongs at
  `/h-commission`, not onboarding.
- **Repo-root `CLAUDE.md` restructured as single source of truth** —
  maintainer-only prelude (haft v8 architecture notes, preserved across
  `haft init`) followed by haft markers wrapping the same showcase
  template that ships to end users. The bracketed section is the
  canonical good-engineering config; the prelude is haft-specific
  context for AI agents working on haft itself.
- CLAUDE.md gained a top-level "v8 Architecture (governance substrate)"
  section describing three surfaces (skills, CLI, MCP) sharing one
  artifact graph, FPF placement (skills = MethodDescription, kernel =
  enforcement), and Transformer Mandate placement on h-decide /
  h-commission.
- README header reframed to "FPF governance substrate". Skill catalog
  table added with mode classification (auto / manual / subroutine).
  Cookbook section added with common workflow walkthroughs.
- Artifact-graph hygiene: superseded 4 prior decisions conflicting with
  the pivot (dec-20260513-v8-architecture-retroactive, v8-sunset-retroactive,
  v8-attribution-retroactive, dec-20260424-desktop-smart-add-rpc).
- `task install` simplified — drops legacy React+Ink TUI build,
  drops v8 OpenTUI bun-install step, drops desktop install hint. Now
  installs only the Go binary + Open-Sleigh runtime. `task install`
  was broken after the pivot (referenced deleted `tui/package.json`);
  this fixes it. Vars `TUI_DIR`, `TUI_BUNDLE`, `TUI_INSTALL_DIR`,
  `TUI_V8_DIR`, `TUI_V8_BUNDLE` removed. Tasks `tui`, `tui-v8-install`,
  `tui-v8-build`, `tui-v8-test`, `tui-v8-typecheck` removed.
  `repomix` include/ignore globs purged of `tui/`, `tui-react/`,
  `desktop/`, `desktop-tauri/`. Desktop vars kept and `desktop:*`
  tasks reachable pending operator decision on full tree removal.
- `task lint` no longer typechecks the deleted legacy TUI TypeScript;
  runs `go vet` only.
- **Maintenance forcing function in entry-point skill bodies (V4)** —
  h-status / h-reason / etc. now act on a surfaced refresh reminder (run
  `haft_refresh(action="scan")` before answering, re-baseline drift on
  in-session files) instead of only printing it.
- **V5 audit — the `description ≠ work` self-check pattern dropped from
  h-frame, h-compare, h-explore** skill bodies, where it nudged the model to
  narrate intent instead of acting.
- **Re-grounding rule added; `h-reason` description trimmed for Codex
  compatibility** — operator-facing text must pair every artifact ID
  (`V1`, `dec-…`, `prob-…`) with its human-readable title (FPF A.7 Strict
  Distinction).
- **Mandatory plain-words final step in `h-explore` / `h-compare`** — both
  skills now end with a short plain-language section (zero artifact IDs, zero
  undefined FPF jargon) the operator can act on from that section alone, so a
  comparison or option set is never delivered as an ID-and-jargon wall the
  operator cannot read.
- FPF spec refreshed to `16cd313 wording-use ontological precision
  restoration` (was `04dd733`, via `562813f`). Picked up upstream: the
  quality-improvement campaign (A.19.ECS Evaluation CharacteristicSpace
  construction, E.8.ECSPF publication form, E.21 pattern-quality, E.9.DA DRR
  decision-adequacy, E.2.DA pillar-adequacy, E.22 improvement-oriented
  quality-read framing, E.23 quality-improvement loop) and the wording-use
  ontological precision restoration (E.10.ARCH architecture, C.16.P, C.16.Q
  [moved from A.6.Q; `evaluativeAscription` → `qualityTermAscription`],
  C.30.P). Embedded SQLite index (`internal/cli/fpf.db`) rebuilt — 5607
  chunks (5541 spec + 66 patterns).
- **`CheckDrift` per-scope tree walk memoized** — `/h-status` (the
  session-mandatory first action) was re-running `filepath.WalkDir` once per
  decision per drift scope; many decisions share the whole-repo `"."` scope, so
  the same ~24k-file tree walk ran ~8× per status (~3.6s dominated by the
  redundant walks, not git subprocesses). The walk is now memoized by
  normalized scope within a single `CheckDrift` pass — identical files per
  scope, computed once instead of N times. Behavior unchanged; no cache-coherence
  surface (the memo lives only for the one pass). (`internal/artifact/decision.go`.)
- Embedded FPF spec index regenerated (`data/FPF` + `internal/cli/fpf.db`).

### Fixed

- **Embedding bake no longer OOMs on the full spec corpus.** The FPF vector
  bake batched 256 sections per sidecar round-trip; transformer attention is
  O(batch · seq²), so a batch of long sections built a multi-GB activation
  tensor and the sidecar ballooned past 6 GB and stalled. The batch is bounded
  small (peak activation back in the low hundreds of MB), with negligible
  throughput cost.
- **Incompatible/old `haft-embed` degrades to FTS instead of hanging.** When the
  resolved sidecar binary could not serve the shared daemon (e.g. an older
  `haft-embed` that rejects `--serve-socket`), the stdio fallback then blocked
  forever on a handshake the old protocol never sends. The shared path now flags
  an unusable binary (`errSharedSidecarUnusable`) and recall degrades straight to
  FTS5+PPR; the stdio handshake is additionally bounded by
  `HAFT_EMBED_STARTUP_TIMEOUT_SECS` (generous default for a cold model download,
  `0` = unbounded) so no resolved binary can hang the caller. Honors the
  "recall never hard-fails on the optional layer" invariant. (`internal/embedding`.)
- **`task install` / `task build` use the committed FPF index, never rebake.**
  They depended on `fpf-refresh`, which re-pulled the FPF submodule and rebaked
  `internal/cli/fpf.db` through a live `haft-embed` — failing (or, before the
  hang fix, stalling) whenever the local sidecar was missing or stale. The
  committed `fpf.db` (go:embed'd into the binary, the same artifact CI verifies)
  is now built as-is; rebaking is the explicit `task fpf-refresh` maintainer step
  after an FPF spec bump.
- **Terminal-status artifacts no longer resurface as expiry debt** — the
  `haft_refresh` stale-collection path now excludes `deprecated` / `superseded`
  artifacts, matching the active-decision scan's status filter. An archived-but-
  expired decision stops showing per-item expiry debt that it no longer counts
  toward the active budget. (`internal/artifact/refresh.go`.)
- **`haft_refresh(action="scan")` summary-by-default** — drift output is
  summarized (counts + top-5 modified paths per decision) instead of dumping
  full per-file diffs that could exceed the context budget on large repos;
  `verbose=true` restores the full dump.
- **Nav-hint feedback loop** — dropped dead `/h-char` and `/h-refresh` nav
  hints that pointed at removed/renamed commands.
- **Import-aware heritage resolution — no shadowed-base wrong edge** (Py + TS).
  `pythonHeritageEdges` / `tsHeritageEdges` resolved a base class/interface
  directory-locally and ignored the import surface, so an unimported same-named
  class in a sibling module could shadow an imported base and emit a **wrong**
  `extends`/`implements` edge to the decoy — violating the no-wrong-edge
  invariant. Heritage now resolves with import awareness (an explicit import
  wins, resolved cross-module; else a same-file type; a name that is both — a
  shadowed redefinition — or neither is dropped), the same exactly-one-or-drop
  discipline the call resolver already used. Also removed a dead `MethodSig.Arity`
  field whose doc-comment claimed a precision mechanism that never ran.
- **Module-level invariants no longer mislabeled "must hold here" on a
  symbol.** A symbol-targeted `code_context` / `explore` was asserting
  module-level invariants (e.g. a roadmap decision's phase gates) as invariants
  binding a symbol they do not govern. The file's invariants are now
  partitioned: only those whose source decision governs the symbol directly (via
  `affected_symbols`) stay under "must hold here"; the rest move to a "Module
  context (may not bind this symbol)" section. File-level views (no symbol) are
  unchanged — every file invariant binds the file. (`internal/contextgraph`.)

## [8.0.0] — 2026-05-14

Flagship release. New v8 agent TUI on Bun + @opentui/core + SolidJS, talking to the haft `agentserver` over HTTP+SSE on `127.0.0.1`. The v7-era React+Ink TUI is renamed to `tui-react/`, kept reachable behind `haft agent --legacy-tui` for the v8.0 cycle, and conditionally retired in v8.1 once interactive usage drops below 5%.

### Added

- **v8 backend backfill** — `agentcore` gains defensive-copy accessors (`Turns()`, `Parts(turnID)`, `PermissionsList()`, `SubAgentsList()`, `ModelChoice()`) so the TUI cannot mutate underlying maps/slices. `agentdriver` learns the `spawn_subagent` tool route + `SubAgentRunner` interface with the single-level invariant (nested spawns return a synthetic `isError` tool_use_completed instead of collapsing the turn). `agentserver` exposes `GET /auth/status` with a snake_case `AuthStatusPayload` + an injectable `AuthStatusProvider`. All M1+M2 invariants from v7.1.0 preserved.
- **v8 TUI package (`tui/`)** — Bun 1.3.13 + `@opentui/core` 0.1.105 + `@opentui/solid` 0.1.105 + `solid-js` 1.9.10. Layered structure:
  - `sdk/core/` — surface-agnostic transport (SSE reader with reconnect + exponential backoff 250ms → 8s cap, RPC poster with 5xx retry + 4xx no-retry, injected decoder + error mapper).
  - `sdk/agent/` — typed wire mirror of `agentproto`: 20 `AgentEvent` variants, branded `SessionID` / `TurnID` / `PartID` / `PermissionID` / `SubAgentID`, `agentErrorMapper` translating HTTP status + body markers (turn_already_running, turn_mismatch, turn_not_running, permission_unknown, unsupported_command, session_not_found) into typed `RPCError`s.
  - `sdk/harness/` — RESERVED namespace for v8.1+ harness panels. Not implemented; the empty slot is the load-bearing extensibility seam.
  - `components/` — `border` / `spinner` / `key-hint` / `logo` primitives.
  - `routes/agent/session/` — `part-text`, `part-reasoning` (collapsible), `part-tool-use`, `permission-prompt`, `subagent`, `turn` view (For-loop dispatch by part kind), top-level `SessionView` with title + model header + turn stream + subagent footer + active permission prompt + key-hint footer.
  - `routes/harness/` — RESERVED for v8.1+.
  - `themes/` — `dark` (default), `light`, `high-contrast`, all implementing a closed 17-token shape. `setTheme` / `cycleTheme` / `currentTheme` reactive store.
  - `router.ts` — route registry with reactive `activeRoute()` accessor; ships with one route registered.
  - `store.ts` — Solid `createStore` + `apply(AgentEvent)` reducer materializing every wire variant (session lifecycle, turn lifecycle, text / reasoning / tool_use part completion, subagent spawn/complete, permission requested/resolved, model switched) into the SessionView shape.
  - `app.tsx` + `main.tsx` — SolidJS root, OpenTUI `render(App)`, reads `HAFT_AGENT_PORT` from env, AbortController-driven teardown on SIGINT / SIGTERM.
  - `build.ts` — Bun build with `createSolidTransformPlugin()` producing single-file ESM at `dist/haft-tui.js` (~1.4MB).
- **`haft agent --v8` opt-in cobra route** — spawns `agentserver` on `127.0.0.1:0`, captures the chosen port, spawns Bun against the TUI bundle with `HAFT_AGENT_PORT` in env, races SIGINT/SIGTERM/Bun-exit/server-fatal with a 2s grace window before SIGKILL fallback and a 5s graceful `agentserver` Shutdown.
- **`haft v8 serve` smoke command** — runs the v8 backend standalone (StoreDispatcher only, no LLM) so the wire protocol can be exercised via `curl` before the v8 TUI is wired. Prints one JSON startup line `{port, addr, store_root, driver}`, handles SIGINT, end-to-end smoke test green.
- **Bundle install pipeline** — `task install` copies `tui/dist/haft-tui.js` to `~/.haft/tui-v8/current/haft-tui.js`. `.goreleaser` archives ship both TUI bundles. `findV8TUIEntry` resolves installed → `tui/dist` → `tui/src/main.tsx` dev mode.

### Changed

- **`tui/` renamed to `tui-react/`** — the v7-era React + Ink + JSON-RPC TUI moves aside to free the `tui/` path for v8. Pure rename; git history preserved via rename detection. `internal/cli/tui_spawn.go`, `Taskfile.yaml`, `.goreleaser.yaml` updated to point at the new path.
- **`haft agent` default route remains legacy** in the v8.0 cycle. The v8 stack is opt-in via `--v8`; the default will flip to v8 in a follow-up release once the provider-integration slice adapts `internal/provider` to `agentdriver.Provider`. `--legacy-tui` is reserved as the post-flip escape hatch.

### Deprecated

- **`tui-react/` (legacy React+Ink TUI)** — deprecation banner prints once per process lifetime to stderr on every `haft agent` invocation. Conditionally removed in v8.1 when telemetry / release-thread feedback confirms <5% interactive usage. Non-interactive `haft board` text dump survives indefinitely.

### Fixed

- (No fixes in this release — the v8 work is purely additive on top of the v7.1.0 foundation.)

## [7.1.0] — 2026-05-13

Maintenance release on top of 7.0.0. Adds an explicit CLI completion path for externally-run WorkCommissions, lays the foundation of the v8 agent stack alongside (not replacing) the v7 production paths, fixes a silent-misroute defect on `haft_decision(measure|baseline|apply)` that corrupted the artifact graph under typical LLM-client usage, and refreshes the embedded FPF corpus.

### Added

- **`haft commission complete-external <wc-id>` CLI** ([#78](https://github.com/m0n0x41d/haft/issues/78), [#79](https://github.com/m0n0x41d/haft/pull/79)) — operator path to close a WorkCommission lifecycle after an external runner (anything outside Haft Harness) has produced local runtime evidence. The command auto-records `start_after_preflight` if the WC is still `preflighting`, then records the terminal `complete_or_block` lifecycle event. Accepts `--verdict completed|pass|failed|blocked` and inline (`--payload '{...}'`) or file-based (`--payload-file evidence.json` / `--payload @evidence.json`) evidence payloads. Refuses queued/terminal/non-running states explicitly rather than abusing `cancel`; does NOT apply, merge, or publish workspace diffs (those remain on `haft harness apply`). Before this, downstream `depends_on` WorkCommissions stayed blocked unless the operator had access to hidden MCP/tool-only lifecycle calls. Contributed by @karabelaselias.
- **v8 agent stack — foundation M1 (`internal/agentcore`, `internal/agentproto`, `internal/agentstore`)** — Layers G2/P/G3 of the new agent runtime planned in `.context/v8_plan.md` §2. `agentcore` defines pure algebraic Session/Turn/Part/Permission/SubAgentLink/ModelChoice types with sealed sum variants, opaque typed IDs (SessionID, TurnID, PartID, PermissionID), and transitions that always return new Session values (no field mutation). Mutating an existing Turn/Part, recording a Part without a Turn, completing a terminal Turn, attaching a SubAgent without naming its parent Turn, or resolving a never-requested Permission are inexpressible by construction. `agentproto` is the wire format shared with the upcoming TS TUI: 18 AgentEvent variants (session.\*, turn.\*, part.\*.delta, part.tool_use.\*, permission.\*, subagent.\*, model.switched, auth.expired) and 9 RPCCommand verbs. Tagged-envelope JSON with `kind` discriminator; `timeStamp` wrapper pins RFC3339Nano UTC; unknown kinds rejected with typed errors so forward-compat surface failures stay at the boundary. `agentstore` is an append-only per-session JSONL journal at `<store_root>/<id>/events.jsonl` with `meta.json` index. Pure Apply/Replay reconstruct a Session byte-for-byte; streaming deltas are wire-only via `IsJournalEvent` predicate so the journal stays compact. M1 acceptance bar — 1000 mixed events through both pure replay and disk round-trip producing `reflect.DeepEqual` Sessions — is asserted by `TestStore_Replay1000ViaDisk`. Coexists with legacy `internal/agent` / `internal/agentloop`; no v7 production code path is touched.
- **v8 agent stack — foundation M2 (`internal/agentserver`, `internal/agentdriver`)** — Layers G4/G5: HTTP+SSE transport and a real turn driver. `agentserver` exposes RPC verbs as POST endpoints (`/session`, `/session/:id/turn|cancel|rename|model`, `/permission/:id`), reads as GET (`/session`, `/session/:id`, `/healthz`), and fans every published AgentEvent to subscribers through a single `/event/global` SSE channel that filters by `session_id` on the client side. Server binds `127.0.0.1:0` and reports the chosen port back for env-var handoff to the TUI process. Pluggable `Dispatcher` interface keeps transport and engine independent; `StoreDispatcher` (test/no-LLM path) and `DriverDispatcher` (real engine) both implement it. `agentdriver`'s `Driver.Drive(ctx, Session, userText)` opens a turn, streams the Provider's events, dispatches tools through `ToolDispatcher`, synchronously gates permission-required tools through a `PermissionGate` (per-driver sync primitive — driver Opens, blocks on chan, HTTP handler Resolves on operator POST; ctx cancellation cleans up pending entries with no leaked channels), and finishes with `turn.completed` or `turn.failed`. Pure orchestration: no global state, no implicit clock, no implicit ID source — `IDGen` and `Now` injected for deterministic tests. `Provider`, `ToolDispatcher`, `EventSink` interfaces decouple from real LLM clients; production wiring for `internal/provider` and `internal/tools` lands in a later slice. `CombinedSink` wraps Store + Hub: state-mutating events go through `agentstore.Append` (journaled) AND publish to Hub (broadcast); streaming deltas are broadcast-only via the `IsJournalEvent` gate. M2 acceptance bar from `.context/v8_plan.md` — `curl /event/global` shows live stream during `haft agent` — is asserted by integration tests covering the full path (POST /session → SSE session.created → POST /turn → SSE turn.started/text deltas/tool_use/turn.completed) plus journal replay producing the same state via `Store.Load`. Deliberately deferred to next slice: materialized assistant TextPart/ReasoningPart events, in-driver SubAgent runner, GET /auth/status, real provider/tool adapters. Legacy `internal/agent` / `internal/agentloop` paths stay alive unchanged in parallel.
- **v8 agent driver code-review hardening pass** — multiple `fix(review)` follow-ups landed against the M1/M2 foundation before release. Concurrency: `store.Load` serialized against concurrent `Append` writes, per-session Append serialized with safe Hub cancel, `model.set` serialized against `turn.submit` Load. HTTP surface: permission errors mapped to 400/404 instead of generic 500, `ErrTurnAlreadyRunning` mapped to 409 Conflict, cancel-aware turn completion with stale-cancel HTTP status differentiation, wire-safe `SessionPayload` serialization on resume. Wire protocol: tool args sent as raw JSON instead of base64, `ModelChoice` and `agentstore.SessionMeta` tagged for snake_case JSON so REST clients populating `credential_key` and `GET /session` responses match the rest of `agentproto` (the Go-field-name leak previously broke case-insensitive matching across `_` boundaries). Turn lifecycle: deltas flushed on provider error with `model.set` rejected mid-turn, replayed running turns rejected synchronously, turn-failed on tool/flush errors with assistant text/reasoning journaled, journal append validation with deduped streamed part IDs, `turn.started` ordered after `StartTurn` with closed-stream-plus-canceled-ctx treated as canceled. Permission gate: validation tightened with cancel matched to turn ID and shared hub, canceled permissions now journaled (previously lost on terminal turn) and `failTurn` publish errors surfaced instead of swallowed. Codex P1/P2 driver findings addressed in the initial review batch, plus two follow-up P2 fixes landed before tag: `agentstore.Store.Append` now treats `writeMeta` as best-effort post-commit — previously a `meta.json` write failure surfaced as `publish failed` to the driver for an event already durable on disk, making the driver skip `turn.failed` for a turn the journal would replay as `Running` and leaving `HasLiveTurn` permanently blocked (the journal is the authoritative record; `meta.json` is a denormalised cache for `List`); and `agentdriver`/`agentcore` now normalise empty `[]byte` tool args to `nil` in `NewToolUsePart` so `json.RawMessage` encodes as JSON `null` instead of producing invalid JSON that failed `PartToolUseStartedEvent` encoding and aborted the turn before the tool ran. Net effect: the v8 foundation merges with wire-protocol invariants verified under concurrent and adversarial conditions, not just happy-path tests.
- **C.28 CausalUse-CAL typed binding on `EvidenceItem` and `DecisionPrediction`** — embeds the new FPF `ee40821` controlled vocabulary structurally, not only as corpus text. `EvidenceItem.causal_support_basis` accepts one of `{observational|interventional|realized_counterfactual|identified_estimate|simulation_only}` on the wire; long FPF names (`simulationOnlyCounterfactualOutputBasis`, etc.) are also accepted and canonicalized on read in `internal/artifact/causal.go`. `DecisionClaim.realizability` and `DecisionPrediction.realizability` accept `{realizable|nonrealizable|unknown}` and round-trip alongside `verify_after`. Per CC-B3.9, `internal/reff.ScoreEvidenceWithCausalBasis` caps R at 0.5 when the basis is simulation-only OR the linked claim's realizability verdict is `nonrealizable`, regardless of verdict/type/CL — undeclared causal-ladder climbs no longer silently raise R. `unknown` realizability does NOT cap (bounded use may still be admissible per C.28). Expired evidence remains at 0.1; the cap floors at 0.5, it does not raise weak scores. The MCP and CLI transports both expose the new fields: `haft_decision(action="evidence")` accepts `causal_support_basis`, and `predictions[]` on `decide` accepts `realizability`. `AttachEvidence` rejects unknown basis values at the artifact boundary (typed invariant); legacy evidence and predictions without the new fields continue to round-trip and score identically to pre-7.1.0 behavior. Soft warning surfaces on both transports when evidence content reads like a causal-use claim (`causal`, `intervention`, `counterfactual`, `uplift`, `treatment effect`) but no basis is declared — warning, not reject, so legacy ingest stays unbroken. `reff.Evidence` gained `CausalSupportBasis` and a forward-compatibility `ClaimRealizability` field; per-claim realizability resolution from `ClaimRefs` is wired through `WLNK` aggregation while assurance-engine plumbing of per-claim realizability via `ComputeClaimAssurance` is marked `TODO(post-7.1)` and degrades gracefully (empty realizability = legacy behavior). 17 new tests pin the contract: 6 in `internal/artifact/causal_test.go` (alias canonicalization, normalization round-trip, omitempty), 5 in `internal/reff/reff_test.go` (cap fires on simulation_only and nonrealizable, no-op on unknown, parity with `ScoreTypedEvidence` for legacy inputs, expired evidence not raised), and 1 in `internal/fpf/server_test.go` asserting the MCP-advertised schema exposes the new fields with the C.28 description.
- **A.15.4 Work-Relevant Source Restoration footer on `/h-view brief|rationale|audit|compare|engineer|manager` projections** — every projection rendered by `internal/present.ProjectionResponse` now ends with an explicit `## Carrier — Not Source of Truth (A.15.4)` section listing the underlying artifact refs (problems, portfolios, decisions) that produced the projection, plus the `haft_query(action="get", ref="<id>")` recovery path and the on-disk `.haft/{decisions|problems|solutions|evidence}/<id>.md` source locations. The skill prompt already cites `X-SOURCE-RESTORATION` (cross-cutting) and names projection outputs as a sweep target; this lands the operational hook at the carrier itself so an LLM consuming a projection sees the recovery instruction inline rather than relying on out-of-band protocol awareness. Empty projections still render the section with an informational fallback line so the carrier semantics stay visible even when the graph is empty. The renderer lives in `internal/present` (flow layer) and stays Core-clean: `TestPureCoreDoesNotDependOnSurfaceOrFlow` is unaffected. 6 golden tests in `internal/present/projection_carrier_test.go` pin the footer across all five non-default views, plus the no-sources fallback and the recovery-path citation.

### Fixed

- **`haft_decision(measure|baseline|apply)` silently misrouted on `artifact_ref`** ([#77](https://github.com/m0n0x41d/haft/issues/77)) — when an LLM client passed `artifact_ref` (the universal target key in `haft_refresh`, and the only documented ref on `haft_decision(evidence)`), the explicit ID was silently dropped and the call resolved to whichever DecisionRecord came back first from `store.ListByKind(KindDecisionRecord, 1)` — most commonly the most-recently-touched one. Baselines snapshotted the wrong files; measurements landed against the wrong claim chain; `haft_refresh action=scan` still flagged the intended target as `no baseline`. The reporter saw the stale-scan count drop from 42 to 27 in one round after manually switching their integration to `decision_ref`. Both serve-mode (`internal/cli/serve.go`) and tools-mode (`internal/tools/haft.go`) handlers had the same defect. Fix: `measure` and `baseline` now accept either `decision_ref` or `artifact_ref` and refuse to proceed without one — the silent `ListByKind(...,1)` fallback is gone for these two actions because corrupting authoritative state is worse than refusing to act. `apply` accepts both keys and keeps the auto-detect fallback since it is a read-only "generate brief" path with no persistent side effect. The FPF-guardrail `bindDecisionRef` helper on the tools path also honors the alias so guarded flows are not bypassed. Schema descriptions for `decision_ref` and `artifact_ref` updated to list every action that accepts each key, so future LLM clients see the right map at registration time. Three regression tests pin the bug shape: two DecisionRecords exist, the caller names the older one via `artifact_ref`, and the test fails if the implementation reaches for the newer one; a third test guards the new "no ref provided" guidance path.
- **`install.sh` picked the wrong archive on CLI installs** — install script could grab the desktop tarball over the CLI archive depending on release asset ordering; now selects the CLI archive deterministically.
- **Artifact store `writeMeta` race on fixed `meta.json.tmp` filename** — the artifact `Store` is documented as concurrent-safe, but `writeMeta` wrote to a fixed `meta.json.tmp` path before renaming, so two concurrent writers raced on the same temp file and could clobber each other's payload mid-write. Switch to `os.CreateTemp` so each writer gets a unique temp file before the rename. Discovered during v8 agent driver review while auditing the M2 hardening pass — same Store backs both legacy artifact paths and the new agent layers, so the fix lands across the surface.
- **CI `-race` flakiness on v8 `agentdriver` integration tests** — `TestIntegration_PermissionRoundTrip`, `TestIntegration_TurnCancel`, and `TestDispatcher_ModelSet_RejectsDuringRunningTurn` raced their `TempDir` cleanup against the dispatcher goroutine: `srv.Shutdown` waits for HTTP handlers but `handleTurnSubmit` fires a detached `DriveTurn` goroutine that keeps writing journal events after the HTTP request returns, surfacing on Linux runners as `WARNING: DATA RACE` reports between `agentstore.Journal.Append` (writer goroutine) and `agentstore.Journal.Close` (test cleanup), plus `TempDir RemoveAll cleanup: directory not empty` failures. Two test-side fixes (no production code path touched): `bootIntegrationServer.cleanup` now drains every session listed by `store.List("")` via a `TurnCancel + poll` helper that loops until the dispatcher returns `ErrTurnNotRunning` (the dispatcher clears its `running` map after `DriveTurn` returns, so that error is the synchronization point guaranteeing the goroutine has fully unwound); `TestDispatcher_ModelSet_RejectsDuringRunningTurn` uses its own dispatcher outside the boot helper and got an explicit `defer drainRunningTurn` call. `fakeTools.calls` also gained a `sync.Mutex` and a `Calls()` snapshot accessor since it was read by the test goroutine while the dispatcher goroutine appended.

### Chore

- **FPF corpus refresh to `ee40821c`** — submodule `data/FPF` bumped from `b18acde` through seven upstream commits to `ee40821` ("formatting for GitHub", 2026-05-12). Substantive content additions, not housekeeping: a new architectural region for causal evidence (C.27 Causal-use calculus + C.28 `CounterfactualSamplingRealizabilityProfile` with controlled `CausalEvidenceSupportBasis` vocabulary: observational / interventional / realized-counterfactual / identified-estimate / simulation-only) and a new A.15 cluster member A.15.4 "Work-Relevant Source Restoration" governing the recover-source-before-reliance step when an encountered item (dashboard, generated explanation, credential view, projection output, copied approval, schema/API wording, composed source chain) is about to support a work or reliance claim by appearance. Two terminology cleanups (A.6.P boundary norm square + counterfactuality) refine wording that the embedded index surfaces automatically via search; the skill prompt cites pattern IDs only, so no skill-side rewording is needed for those. Index regenerated via `task fpf-refresh`: indexed_chunks 4972 → 5062 (+90; 4996 spec + 66 patterns), fpf_commit matches submodule HEAD. Index and submodule pointer move together — the release workflow rebuilds the index from the locked submodule SHA on tag, so a drift between these two files would surface as either stale search results in dev or a mismatch on next release build.
- **`h-reason` skill prompt minor update for new FPF concepts** — two changes:
  - New cross-cutting pattern `X-SOURCE-RESTORATION` (A.15.4) added to `internal/fpf/patterns/cross-cutting.md` and surfaced in the skill floor's Cross-cutting block. The detection rule "Object ≠ Description ≠ Carrier" was already in the skill; the operational rule "before reliance, recover the project source that makes the action admissible" was not. Practical sweep targets named explicitly include dashboards, generated explanations, credential views, projection outputs (`/h-view brief|rationale|audit`), copied approvals, provenance labels, schema/API text, and composed source chains. Anti-pattern: skipping restoration because the carrier "looks authoritative" — the failure A.15.4 names.
  - `DEC-06 Predictions` extended with one sentence: predictions are causal claims; check realizability (can you sample the target distribution under physical / ethical / operational constraints?) before committing them as acceptance gates. Pointer to C.27 / C.28 added so the agent can pull the full calculus on demand without bloating the L1 floor.
- **Drop `darwin-amd64` (Intel Mac) from the CLI release matrix** — no longer built.

## [7.0.0] — 2026-04-29

v7 promotes specs to authoritative artifacts. The product is no longer "decision governance plus task execution"; it is **project harnessability**. A repository becomes harnessable only after it carries a parseable ProjectSpecificationSet (TargetSystemSpec + EnablingSystemSpec + TermMap), and Decisions / WorkCommissions / RuntimeRuns / Evidence flow downstream as consequences of that spec. The product surface model is also clearer: one Haft Core (semantic authority) under two production surfaces — MCP Plugin (embedded host-agent surface for Claude Code and Codex) and CLI Harness (operator/runtime surface). Desktop remains an alpha track and is not part of the v7 production envelope. Surfaces dispatch typed actions; they do not invent semantics.

This is a major release. v6 artifacts (decisions, problems, notes, evidence, commissions) carry forward without migration; the new spec carriers and the SpecOnboardingMethod typed flow are additive. Re-run `haft init` in existing projects to pick up the updated MCP commands and placeholder spec carriers.

### Added

- **Multi-commission decision scope-leak guard** — second-and-later `haft_commission(action="create_from_decision")` calls against the same `decision_ref` now require an explicit `slice_description` parameter naming which slice of the parent DecisionRecord this commission specifically implements. Without it the call returns the typed error `multi_commission_requires_slice_description`. Background: a v7 dogfood batch (decision dec-20260429-v7-spec-drift-surfacing-990b1d96, 4 commissions on the same decision with three of them sharing a lockset on `internal/cli/serve.go`) revealed the scope-leak anti-pattern — each codex inherits the full decision body, sees its `allowed_paths` include the hot file, and independently implements every slice whose scope intersects its writeable surface. Lockset serialization does not help: each session starts from base HEAD without awareness of the other slices' work. The new guard rejects the second commission unless it carries explicit per-slice scope text. Single-commission decisions (the common case) are unaffected. The guard pairs with two additional surfaces: (1) `haft harness result` now prints a `slice:` line when the commission carries `slice_description`; (2) `haft harness run --drain` prints `WARN:` lines at startup when two or more selected commissions share both `decision_ref` and any `lockset` entry, OR share `decision_ref` while any of them lacks `slice_description`. Background note: `.context/multi-commission-anti-pattern-retrospective.md`.
- **Harness batch drain workflow with per-commission delivery_policy** (`dec-20260428-harness-drain-v3-16bf21f3`) — opt-in `haft harness run --drain --concurrency N` operator path drains runnable WorkCommissions while preserving claim-time lockset conflicts and AutonomyEnvelope creation/preflight/execute gates. Per-commission `delivery_policy` is the apply-authority gate (V3 invariant: NO envelope evaluation at apply): a terminal commission with verdict=pass and `delivery_policy=workspace_patch_auto_on_pass` is auto-applied to the project checkout as a discrete revertable git commit directly from the drain loop, emitting `auto_apply_succeeded: commission=... files=N` on operator stdout. `workspace_patch_manual` (default) keeps the diff in the workspace clone awaiting `haft harness apply`. An EXPLICITLY blocked AutonomyEnvelope decision still keeps the manual path even on `auto_on_pass` policy because that represents a concrete operator decision rather than a missing snapshot. Stale leases older than the configured age cap (default 24h, override via `OPEN_SLEIGH_STALE_LEASE_MAX_AGE_S`) are skipped at intake with typed `lease_too_old` reason and surfaced in `haft harness status`. Four landed slices: `f28a6615` drain flag + Open-Sleigh keep-alive, `48593198` per-commission delivery decision payload, `61874c5c` stale-lease cap at intake (canonical layer) plus surgical fix of slice 1's accidental orchestrator-side stale check that broke retry-timer tests, `d13db50c` + `1301abac` auto-apply trigger in `watchHarnessDrainUntilIdle` plus V3 invariant repair (envelope_missing no longer blocks auto-apply). End-to-end validated on real codex over 4 dogfood rounds; auto-apply currently wired into drain mode only — single-commission `haft harness run` (no `--drain`) still requires manual `haft harness apply` even on `auto_on_pass`. Detailed guide: [`docs/7.0/harness-batch`](https://haft.dev/docs/7.0/harness-batch). Treat as Beta on production-code commissions.
- **`task_context` on the four remaining persistent artifact kinds** ([#66](https://github.com/m0n0x41d/haft/issues/66) follow-up to v6.2 DecisionRecord work in `dec-20260424-8b141266`) — `haft_problem(action="frame")`, `haft_solution(action="explore")`, `haft_note(action="record")`, and `haft_refresh(action="waive|reopen|supersede|deprecate")` now accept an optional `task_context` string that flows into the artifact ID and filename as a sanitized slug, e.g. `prob-20260428-task-12-a1b2c3d4.md`. Empty/missing `task_context` keeps the current `<prefix>-YYYYMMDD-<8hex>` shape, so existing callers are unaffected. Sanitization stays in the canonical `sanitizeIDSlug` helper — no per-kind divergence. `ReopenDecision` and `CreateRefreshReport` got `*WithTaskContext` variants while preserving the original signatures as backwards-compat wrappers. Closes the surface-uniformity gap reported in issue #66 (DecisionRecord-only support shipped in v6.2.x).
- **Haft v7 specification onboarding spine** — `haft init` now creates parseable placeholder carriers for `.haft/specs/target-system.md`, `.haft/specs/enabling-system.md`, and `.haft/specs/term-map.md` without inventing active product claims. New `haft spec check` runs deterministic L0/L1/L1.5 checks over fenced `yaml spec-section` blocks, term-map entries, optional section fields, and obvious carrier/object authority confusion.
- **Derived spec coverage CLI** — `haft spec coverage` and `haft spec coverage --json` derive per-section states (`uncovered`, `reasoned`, `commissioned`, `implemented`, `verified`, `stale`) from artifact links, WorkCommissions, affected files, and evidence. Output includes `why` and `next_action` rather than a single coverage percentage.
- **Project readiness state `needs_onboard`** — Go core, Desktop Rust shell, and TypeScript UI now distinguish `ready`, `needs_init`, `needs_onboard`, and `missing`. Desktop blocks generic task spawning until a project is ready, while initialized projects with draft or incomplete spec carriers surface onboarding as the primary action.
- **Desktop onboarding slice (alpha)** — Settings now exposes typed onboarding actions for initialized projects, including `Open Target Spec`, `Open Enabling Spec`, `Open Term Map`, `Run Spec Check`, and `Refresh Readiness`, with spec-check findings grouped by carrier row. Desktop surfaces remain alpha and are not part of the v7 production envelope; the canonical v7 onboarding surfaces are CLI (`haft spec onboard`) and MCP plugin (`haft_spec_section`).
- **Harness readiness guard with `--force-skip-specs` escape** — `haft harness run` blocks broad execution for `needs_onboard` projects by default. Operators may pass `--force-skip-specs "<reason>"` for explicit out-of-spec tactical work; the reason is recorded on selected WorkCommissions through `spec_readiness_override`. The flag is audit-only — it does NOT relax scope guards, lockset enforcement, or AutonomyEnvelope.
- **`/h-commission` operator command and lifecycle actions** — plugin-mode users now have an explicit entrypoint for the DecisionRecord → WorkCommission authorization step. `/h-commission` creates/reuses WorkCommissions without starting execution and can inspect, cancel, or requeue existing commissions with explicit transition constraints. Codex installs it as an explicit-only `$h-commission` skill; starting Open-Sleigh remains a CLI/Desktop runtime boundary via `haft harness run`.
- **Packaged Open-Sleigh runtime install path** — `task install`, release archives, and `install.sh` now treat Open-Sleigh as a first-class Haft runtime under `~/.haft/runtimes/open-sleigh/current`. `haft harness run` can launch either a repo-local source runtime through `mix` or an installed release runtime through `bin/open_sleigh`, so release users do not need a local `open-sleigh/` checkout for harness runs.
- **SpecPlan draft proposal CLI** — `haft spec plan` groups uncovered or stale spec sections by document kind, spec kind, dependency signature, and affected area, then emits review-only DecisionRecord drafts. The command is explicitly read-only by default; `--accept <proposal-id>` is the one executable proposal action that creates one DecisionRecord with section refs, while `merge`, `split`, and `discard` are typed non-executable actions that report their command gap rather than silently degrading.
- **Desktop harness page detail (alpha)** — the Harness page surfaces structured workspace, runtime, evidence, operator-next, and filtered tail facts for a selected WorkCommission instead of forcing operators to inspect raw JSON or logs. Alpha surface; the canonical operator path for v7 is CLI (`haft harness run` / `haft harness status` / `haft harness apply`).
- **Golden v7 E2E proof** — `internal/cli` now has a deterministic smoke that drives a temp project through `go run ./cmd/haft spec check --json`, spec coverage, spec plan, DecisionRecord, WorkCommission preparation, mock RuntimeRun lifecycle, evidence attachment, and verified SpecCoverage edges.
- **ImplementationPlan hybrid core** — new `internal/implementationplan` package models plan id, revision, decision refs, dependency edges, and locksets as pure types. It rejects cycles and impossible dependencies, and is the substrate for harness plan parsing and commission scheduling. Tests cover DAG parse, cycle rejection, dependency satisfaction, and overlapping-lockset conflict.
- **AutonomyEnvelope minimal core** — new `internal/autonomyenvelope` package introduces the first explicit envelope types: allowed repos/paths/actions/modules, concurrency and commission budgets, forbidden one-way-door actions, failure strategy, and expiry/revocation. Commission preflight reads an envelope snapshot when present, blocks out-of-envelope actions deterministically, and cannot skip freshness, scope, or evidence gates. Default behavior remains conservative when no envelope exists.
- **WorkCommission projection intent and projection debt** — new `internal/workcommission/projection.go` models `local_only`, `external_optional`, and `external_required` projection policies. `external_required` becomes an explicit `completed_with_projection_debt` block state when execution evidence passes but external publish is missing or failed; `local_only` is unaffected. Invented status, owner, date, severity, completion, scope, and promise claims are rejected at validation.
- **Spec refs propagated through DecisionRecord and WorkCommission** — spec-linked decisions now carry stable spec refs, and WorkCommissions copy the relevant spec refs and revision/snapshot facts into their commission snapshot. Commissions with missing spec refs are blocked unless an explicit tactical override is recorded; SpecCoverage derives edges from refs instead of fuzzy title matching.
- **Commission snapshot freshness gates** — preflight now blocks stale or drifted commissions before execution starts on decision revision hash, problem revision hash, base SHA, scope hash, and spec refs/revisions where available. A targeted stale-block canary covers the path; hard freshness mismatch is terminal/blocking, not advisory.
- **SpecOnboardingMethod typed core** — new `internal/project/specflow` package models the v7 onboarding flow as pure typed Go values. `Phase`, `Check`, and `WorkflowIntent` form the contract surfaces consume; the canonical phase registry covers the target-system spine (environment/role/boundary) and the enabling-system spine (architecture/work-methods/effect-boundaries/agent-policy/commission-policy/runtime-policy/evidence-policy) in one ordered chain. Each phase composes Checks (`RequireField`, `RequireStatementType`, `RequireClaimLayer`, `RequireValidUntil`, `RequireTermDefined`, `RequireGuardLocation`, `RequireBoundaryPerspectives`) so SoTA fields cannot be omitted. FPF citations stay in agent reasoning and in Core-emitted `context_for_agent` strings; carriers never carry pattern IDs.
- **`haft spec onboard` CLI subcommand** — operator-facing entry point that returns the next typed `WorkflowIntent` for the current project as plain text or `--json`. Flags `--approve`, `--rebaseline`, `--reopen` mutate the SpecSectionBaseline store; `--reason` and `--approved-by` are recorded in the audit trail.
- **`haft_spec_section` MCP tool** — embedded host-agent surface for the same typed onboarding loop, with actions `next_step`, `approve`, `rebaseline`, `reopen`. Returns the same `WorkflowIntent` / baseline-result JSON the CLI emits so plugin-mode and CLI surfaces share one shape.
- **SpecSectionBaseline + drift detection** — SQLite migration v28 adds `spec_section_baselines(project_id, section_id, hash, captured_at, approved_by)`. The `approve` transition snapshots a SHA-256 baseline of the active SpecSection; subsequent edits to the carrier surface as `spec_section_drifted` findings in `haft spec check`. Triage actions: `rebaseline` (intentional evolution; reason required), `reopen` (drop baseline; section returns to onboarding loop). Mirrors the existing decision baseline + drift pattern at the spec level.
- **Spec section staleness detection** — `haft spec check` and the new `haft_query(action="check")` action emit `spec_section_stale` findings for active SpecSections whose `valid_until` is past today. Refresh discipline now lives at the claim level, not only at evidence.
- **`haft_query(action="check")` MCP action** — plugin-mode parity for the CLI `haft check` command. Returns the unified, CI-actionable enforcement report covering decision drift + evidence decay + unassessed decisions + coverage gaps + spec drift + spec stale + spec structural in one structured response. JSON parity with `haft check --json` is enforced by a contract test.
- **`/h-onboard`, `/h-status`, `/h-verify` rewired around the new tools** — `/h-onboard` now drives onboarding through `haft_spec_section(action="next_step")` with mandatory FPF retrieval per phase via `haft_query(action="fpf", ...)` and forbids FPF citations inside `.haft/specs/*` YAML carriers. `/h-status` distinguishes overview (`status`) from CI-actionable enforcement (`check`). `/h-verify` discovery routes through `haft_query(action="check")` first; legacy `haft_refresh(action="scan")` stays for drill-down. Regression tests assert load-bearing clauses across all three prompts.
- **Readiness nudge on MCP reasoning tools** — `haft_problem(frame)`, `haft_solution(explore)`, `haft_decision(decide)`, and `haft_note` now append a soft warning to text results when the project is `needs_onboard`. Warns the operator that decisions made now cannot link to spec refs and downstream WorkCommissions / harness runs will block until specs are in place. Skipped on machine-JSON responses, on tools that already enforce readiness (haft_commission, haft_spec_section, haft_refresh, haft_query), and on `ready` / `needs_init` / `missing` projects.
- **`haft_query(action="resolve_term")` for investigation-first discipline** — new MCP action grounds an umbrella term in the project's bounded context in one round-trip: returns `term_map_entries`, `spec_section_refs` (sections whose `terms` field references the term), `artifact_mentions` (FTS-indexed past Decisions / Problems / Notes), and a typed `resolution` (`resolved` / `ambiguous` / `absent`) with a structured `next_action` hint. `/h-frame`, `/h-decide`, and `/h-note` slash prompts now require the host agent to sweep the bounded context via `resolve_term` BEFORE bouncing back to the operator on vague signals — and even then to ask one concrete question naming the candidates the resolver returned, not "what do you mean?".
- **Experimental OpenCode (sst/opencode) host support** ([#68](https://github.com/m0n0x41d/haft/issues/68)) — `haft init --opencode` writes `opencode.json` at the project root with the `mcp.haft` block (`type: local`, `command: ["<binary>", "serve"]`, `environment.HAFT_PROJECT_ROOT`, `enabled: true`), preserves any existing top-level keys (theme, username, formatter config, other MCP servers), and removes the legacy `quint-code` MCP entry if present. Commands install to `~/.config/opencode/commands/` (or `.opencode/commands/` with `--local`); the h-reason skill installs to `~/.config/opencode/skills/h-reason/` (or `.opencode/skills/h-reason/` with `--local`). Pass-through transformer — same SKILL.md and command markdown as Claude Code. OpenCode is tracked alongside Cursor and Gemini CLI as an experimental/legacy host; production v7 plugin support is still narrowed to Claude Code and Codex.

### Changed

- **v7 production surface model documented as MCP Plugin + CLI Harness over one Haft Core** — specs and README now state that MCP is the embedded Claude Code/Codex agent surface and CLI Harness is the operator/runtime surface. Plugin commands and CLI subcommands must compile to typed artifact transitions rather than free prompts. Desktop is tracked as an alpha track outside the v7 production envelope.
- **v7 host-agent support narrowed** — Claude Code and Codex are the supported embedded host-agent surfaces. Cursor, Gemini CLI, JetBrains Air, and generic MCP clients are retained only as experimental or legacy protocol carriers.
- **MCP prompt guidance updated for spec-first work** — `/h-onboard`, `/h-status`, and `/h-commission` now describe target/enabling specs, term maps, readiness, commission recovery, and the plugin/runtime boundary; regression tests cover the load-bearing prompt clauses.
- **Harness operator output made compact and actionable** — `haft harness run`, `status`, `watch`, `tail`, and `result` now prefer one human line per meaningful runtime state, terminal next-action hints, evidence summaries, workspace/diff facts, and raw JSON only behind explicit JSON/debug output.
- **Long-lived desktop conversation behavior tightened** — provider/control envelopes are audit-only instead of visible chat messages, and follow-up text on terminal/checkpointed/blocked tasks routes to continuation instead of writing to a dead PTY.
- **Desktop task status normalization shared across desktop surfaces (alpha)** — raw task status values compile through typed `TaskRunState` helpers before input capability, dashboard attention, Jobs columns, status dots, chat streaming, and implementation ladder state are derived. Alpha; production surfaces are CLI and MCP.
- **Spec coverage now models runtime carriers** — `haft spec coverage` derives WorkCommission → RuntimeRun → evidence edges from stored runtime events, promotes implemented/verified states from real runtime and evidence signals, and only reports RuntimeRun gaps for malformed carriers instead of emitting a synthetic global unsupported-edge gap.
- **WorkCommission lifecycle semantics centralized across surfaces** — Go core, Desktop view models, CLI harness selectors, Desktop RPC, spec coverage, and Open-Sleigh now share the same lifecycle meanings. `failed` is recoverable and requires operator action, completion states satisfy dependencies, and only completed/projection-debt/cancelled/expired states are terminal.
- **ProjectSpecificationSet typed core** — `internal/project` now treats the canonical spec model (`ProjectSpecificationSet`, `SpecDocument`, `SpecSection`, `TermMapEntry`) as pure typed objects with explicit draft/active/deprecated/superseded/stale/malformed states, instead of loose maps and ad-hoc strings. Markdown remains the carrier and fenced YAML the canonical parse object.
- **Mutating Open-Sleigh tools enforce commission scope before mutation** — adapter and tool runtime paths now require the WorkCommission scope guard up front, so write/edit calls outside `allowed_paths` and writes to `forbidden_paths` are rejected before any file changes. Terminal diff validation is retained as defense-in-depth, not the first hard guard.
- **SpecSection vocabulary aligned via single source** — `project.SpecSectionValidStatementTypes`, `project.SpecSectionValidClaimLayers`, and `project.SpecSectionValidGuardLocations` are now the canonical sets consumed by both the parser-level speccheck and the SpecOnboarding method's Check vocabulary. Eliminates silent drift where `rule`/`promise`/`gate` were valid in one validator but rejected by the other.
- **`haft check` rolls up spec health into the unified exit code** — CI-facing `haft check` now reports stale / drifted / unassessed / coverage-gap findings PLUS the spec health rollup (drift + stale + structural). Single non-zero exit when any kind of debt exists; existing decision/evidence checks contribute the same way they always did.
- **`/h-commission` slash command updated for v7.x batch surface** — slash command now documents `delivery_policy` (manual default vs auto_on_pass opt-in), drain mode flags, V3 apply invariants (envelope at create/preflight/execute only; per-commission discrete revertable apply; no remote ops from drain), Open-Sleigh stale-lease cap with `OPEN_SLEIGH_STALE_LEASE_MAX_AGE_S` env override, and the `--drain cannot be combined with --detach` constraint. Agents reading this command now get the V3 mental model rather than the V6.x prepare-only flow.
- **README batch harness section + qc-landing detailed guide** — README has a new "Batch Harness (Beta — v7.x)" section between "What Makes It Different" and "Desktop App" with three-step quickstart and Beta disclaimer. Detailed guide lives at [`docs/7.0/harness-batch`](https://haft.dev/docs/7.0/harness-batch) covering workflow diagram, three real use cases (overnight docs/refactor batch, mixed manual+auto, single-commission baseline), invariants and guard surfaces, known rough edges (Elixir deps cache in workspace clones, `commission list` selector default, drain-only auto-apply scope), and three common troubleshooting scenarios. v7.0 is now the default version on the docs nav dropdown; v6.2 demoted to a regular link.

### Fixed

- **Desktop harness IPC action shape** — Tauri commission actions now translate camelCase UI arguments into the snake_case CLI RPC payloads expected by the Go handlers, including the new `harness_tail` command.
- **Multi-turn desktop continuation cleanup** — third and later follow-ups now preserve durable conversation turns while stripping continuation control prompts and audit-only provider envelopes from both Rust seed blocks and TypeScript transcript rendering.
- **WorkCommission lifecycle action ordering** — lifecycle record/start/run/complete actions now reject out-of-order updates instead of appending impossible runtime events to queued or preflight-only commission state.
- **Project readiness false-ready projection** — Desktop readiness now combines reported status with project existence, `.haft`, and spec-carrier facts so missing or onboarding projects cannot render as runnable from an optimistic carrier status.
- **WorkCommission attention recovery hints** — queued, ready, preflighting, and running commissions that need operator attention now keep `requeue` available when recoverable; expired commissions remain limited to inspect/cancel lifecycle actions.
- **Harness workspace apply scope enforcement** — `haft harness apply`, auto-delivery, CLI result output, and Desktop harness RPC now require explicit `scope.allowed_paths`, honor `scope.forbidden_paths`, and surface typed disabled-apply reasons for forbidden, unknown, or out-of-scope workspace diffs. `affected_files` and `lockset` remain scope facts but no longer authorize mutation by themselves.
- **Open-Sleigh legacy canary compatibility** — tracker-first legacy tickets no longer fail the new commission-mutation guard, while real WorkCommission runs still require scoped material changes. The mock adapter now emits deterministic measure evidence so legacy canaries exercise the evidence path instead of timing out.
- **`haft_commission` MCP schema rejected by host LLM API** — top-level `allOf` block of conditional `if/then` requirements (commission_id required for show/requeue/cancel; reason required for requeue/cancel) passed Go-side schema construction but the host LLM API rejects top-level `allOf`/`oneOf`/`anyOf` and took the entire haft MCP server offline (`/mcp` reported `1 MCP server failed`; `/h-verify` and `/h-onboard` returned HTTP 400). Per-action conditional requirements were already enforced at the handler boundary in `internal/cli/serve_commission.go`; the schema only declares `action` as universally required now. Two regression tests prevent the same shape from creeping back into any tool: one specific to `haft_commission`, one iterating every advertised tool's `inputSchema`.
- **`haft_solution` compare variant identifier discoverability and silent set override** ([#71](https://github.com/m0n0x41d/haft/issues/71)) — the explore → compare round trip silently lost data in three independent ways:
  - **Generated variant ids never surfaced.** `materializeVariantIDs` produced `V1`/`V2`/... when callers omitted `id`, but the explore response only showed them in body prose. ChatGPT/Codex agents skipped the prose and sent free-form titles to compare, which then errored as "outside the declared compare set" with no list of correct ids. The explore response now appends a deterministic `Variants:` index (`V1 — <title>` rows) and a usage hint listing every payload field that must use those exact ids. Comparison error messages also append `; expected one of: ["V1", "V2", ...]` so the agent can self-correct without re-running explore.
  - **`parseJSONArg` swallowed JSON shape errors.** Callers used `_, _ = parseJSONArg(...)` for `dominated_variants`, `pareto_tradeoffs`, and `incomparable`. A malformed value (wrong shape, not `[]any`) produced an empty payload, and validation reported "missing variant" errors that pointed nowhere. `parseJSONArg` now returns `(present, error)` and all three call sites propagate.
  - **Caller-supplied `non_dominated_set` silently overridden when computed Pareto carried no dominance signal.** When every dimension scored with text outside the canonical ordinal vocabulary (e.g., `"medium-high"`, `"good"`) `compareDimensionValues` returned `dimensionComparisonUnresolved` for every pair and the conservative computed front collapsed to the entire compare set. The Compare path then overwrote the human's manual ranking with this noise. The new honesty fallback detects "zero comparable pairs across all dimensions" via `paretoFrontResult.comparablePairCount`, retains the caller's `non_dominated_set` as authority for explanation coverage, keeps `ComputedParetoFront` conservative (full set) for transparency, and emits a typed warning naming the indecisive dimensions. When ANY pair is comparable, the computed front continues to override the caller's set as before.

### Chore

- **Repo-root-safe frontend unit tests** — the desktop frontend package now exposes `npm --prefix desktop/frontend test` for Node contract tests over readiness, IPC argument shape, transcript filtering, and view data.
- **Dogfood spec readiness state recorded** — `spec/enabling-system/DOGFOOD_SPEC_READINESS.md` documents the current Haft-on-Haft readiness as `needs_onboard` (placeholder `.haft/specs/*` carriers, `.haft/` ignored at repo root), so dashboards do not show fake active spec authority for this repo.
- **FPF corpus refreshed to upstream `b18acde`** — `data/FPF` submodule bumped to bring in upstream "terminology cleanup in E.8, E.9, E.19" and prior "controlled semantic coarsening" / "quantum-like cluster" commits. Embedded `internal/cli/fpf.db` regenerated against the new spec: 4818 → 4961 indexed sections (+143). Search results from `haft fpf search` and `haft_query(action="fpf", ...)` now reflect the cleaned-up E.9 (Design-Rationale Record Method), E.8, and E.19 terminology. Slash commands and `h-reason` SKILL.md needed no edits — none cite E.8/E.9/E.19 sections directly, and FTS-driven retrieval picks up the new terminology automatically. One curated golden case (`internal/fpf/testdata/search_golden_queries.json` → "decision record lookup accepts current E.9 conformance hit") had its `expected_pattern_ids` and `top_n` refreshed because FTS ranking shifted post-cleanup; documented in `bb64ec8f`.
- **Release pipeline regenerates the embedded FPF index from the locked submodule SHA before `go build`** — `.github/workflows/release.yml` now invokes `cmd/indexer` against `data/FPF/FPF-Spec.md` ahead of binary compilation, so a developer who tags a release without first running `task fpf-refresh` no longer ships a stale corpus snapshot. Defense in depth: the committed `internal/cli/fpf.db` should already match, but the release artifact is now authoritatively built from the submodule pointer.
- **`darwin-amd64` (Intel Mac) CLI build dropped from the release matrix** — GitHub-hosted `macos-13` runners (the last Intel Mac image) are deprecated and scheduling priority has been lowered such that build jobs queue 30+ minutes during peak hours. Apple Silicon is ~95%+ of the developer Mac fleet at this point, so the cost of holding the entire release pipeline on a long-tail Intel build is no longer justified. Intel Mac users on v7.x can either stay on v6.2.1 or request a v7.0.x patch with a hand-built `darwin-amd64` artifact uploaded via `gh release upload`. Desktop `darwin-amd64` shipping continues unchanged because it cross-compiles cleanly on `macos-latest` via the Tauri Rust target.

## [6.2.1] — 2026-04-22

### Fixed

- **`haft init --codex` no longer installs deprecated Codex prompts** — Codex initialization now migrates the bundled `h-*` entrypoints into Codex skills under `.agents/skills`, removes stale Haft prompts from `~/.codex/prompts` when Air is not also requested, and writes `agents/openai.yaml` so only `$h-reason` allows implicit invocation. The phase/action skills (`$h-frame`, `$h-char`, `$h-explore`, `$h-compare`, `$h-decide`, `$h-view`, `$h-verify`, `$h-status`, `$h-search`, `$h-note`, `$h-onboard`, `$h-problems`) are explicit-only. Air keeps its existing prompt bootstrap path.

### Chore

- **FPF corpus/index refresh** — bumped the embedded FPF corpus snapshot and regenerated `internal/cli/fpf.db`.

## [6.2.0] — 2026-04-20

### Fixed

- **MCP `haft_solution` and `haft_problem` schemas missing `parity_plan`** ([#62](https://github.com/m0n0x41d/haft/issues/62)) — deep-mode comparison validates a structured parity plan (`baseline_set`, `window`, `budget`, `missing_data_policy` per FPF G.9:4.2) but the advertised MCP tool schema in `internal/fpf/server.go` did not expose any parameter that accepted those four keys. Deep mode was unreachable from MCP clients (Claude Code, Cursor, Gemini CLI, Codex). The standalone schema and dispatcher already handled `parity_plan` correctly — only the MCP-advertised schema was missing it. Added structured `parity_plan` object parameter to both `haft_solution(action="compare")` and `haft_problem(action="characterize")`. Two regression tests assert it stays exposed.
- **Artifact ID collisions across branches** ([#63](https://github.com/m0n0x41d/haft/issues/63)) — `GenerateID` previously rendered a per-day sequential counter (`dec-20260418-001`), so two branches creating decisions on the same day produced identical filenames in `.haft/` and the merge had to be resolved by hand. Switched to a 32-bit `crypto/rand` hex suffix (`dec-20260418-a3f7c198`). ~4.3B values per kind per day; birthday-paradox collision probability stays below 10⁻⁶ for the first few thousand IDs. The `seq` parameter is preserved for call-site compatibility but no longer rendered into the ID. Three regression tests cover the new format and uniqueness guarantees.
- **Tauri IPC shape mismatch on every mutation** — the desktop backend's `rpc_mutation!` macro declared every Tauri command as `(project_root: String, payload: Value)`, but the frontend sends flat command-specific fields (`{ path }`, `{ decision_id, reason }`, `{ input }`, …) and never passed `projectRoot` explicitly. Every user-facing UI action (switch project, dashboard load, create decision, implement, etc.) failed at Tauri's IPC argument validation before reaching the CLI subprocess. Rewrote the macros to accept each command's real field shape and made `projectRoot` optional (subprocess inherits `HAFT_PROJECT_ROOT` from the Tauri host environment). Four Tauri command registrations renamed to match the frontend (`characterize_problem`, `adopt_problem_candidate`, `dismiss_problem_candidate`, `assess_comparison_readiness`) plus new `add_project_smart`.
- **Cross-project decision recall keyed on user-supplied title** — the global decision index was storing `selected_title` as `decision_id`, so two decisions with the same chosen option in one project would collide and overwrite each other in `~/.haft/index.db`. Now uses the canonical artifact ID returned by `artifact.Decide()`. Requires plumbing `dispatchTool` to return the created artifact ref alongside the response string.
- **FPF hint map drift risk** — previous implementation hardcoded pattern IDs per phase in a Go map that could silently diverge from pattern files. Hints now generate from embedded markdown; renaming or removing a pattern ID is detected at build time via smoke test.
- **Baseline directory crash** — `hashFile()` now detects directories and returns skip error instead of attempting to read directory as file. `affected_files` containing directory paths (e.g. `src/infra/auth/`) are skipped gracefully instead of failing the entire baseline operation.
- **Test alignment** — baseline and verification tests updated to match graceful skip behavior.

### Changed

- **Desktop frontend migrated from Wails v2 to Tauri v2** — Rust shell + React 19 + TypeScript; faster launch, smaller binary, native per-platform packaging. `haft desktop` launcher finds the installed app or falls back to `desktop-tauri/target/release/bundle/` in dev.
- **`parity_plan` JSON Schema unified** — both transports now share a single `artifact.ParityPlanJSONSchema` helper instead of two parallel maps in `tools/haft.go` and `fpf/server.go`.
- **Pattern attribution cleaned up** — patterns sourced from Levenchuk material (slideument + semiotics slideument) relabeled from generic "Haft operational pattern" to specific source references (FRAME-06/07, CHR-02/06/07/08, CMP-07, EXP-07, VER-09, X-TERM-QUALITY, X-GLOSSARY, X-BITTER-LESSON).
- **FRAME-07 Goldilocks** — fabricated "10-20% beyond current capability" replaced with zone-of-proximal-development framing per slideument slide 7.
- **FRAME-08 unified with X-STATEMENT-TYPE** — question 3 of the Reading Checklist delegates to the X-STATEMENT-TYPE taxonomy (rule/promise/explanation/gate/evidence) instead of duplicating a parallel list.
- **CHR-11 source clarity** — explicit note distinguishing the slideument slide 35 didactic 5-step compression from the canonical FPF-Spec A.6.P:4 four-layer structure (Stable lens → Kind-explicit relation tokens → Slot-explicit qualified relation records → Change-class lexicon → Lexical guardrails). Each didactic step carries a canonical A.6.P:4.x reference.
- **h-reason SKILL.md trimmed** — 400 → 359 lines. Removed Concept Index (duplicated routes matchers), merged RAG search reference into FPF spec lookup, compacted Feature Maturity table into a status-keyed list. Micro-patterns preserved as direct-response floor.
- **Removed "Mandatory FPF retrieval (MUST execute before reasoning)" section** from h-reason SKILL.md — contradicted the interaction-mode protocol and doubled the auto-hint cost.
- **Hint query keywords dynamized** — per-phase example retrieval keywords now derived from the first N matchers of the corresponding `phase-*` route in `fpf-routes.json` instead of a hardcoded Go map. Matcher rename propagates to hint automatically.
- **`NextSequence` deprecated** — `GenerateID` no longer needs sequence lookup since the switch to crypto/rand suffixes. All five artifact creators (Decide, FrameProblem, ExploreSolution, CreateNote, CreateRefreshReport) now skip the wasted DB query and pass 0. The function itself stays in the `Store` and `ArtifactStore` interface for one release; planned for removal in 6.3.
- **Single `haft run` pipeline** — removed `--steps` and `--plan` as separate modes. One pipeline: Plan → Execute → Review → Baseline. `--auto` controls whether to pause.

### Added

- **Dashboard + Implement + Adopt flows (desktop)** — one proved operator loop from decision to PR. The Dashboard replaces the separate Problems / Decisions pages: active decisions with Implement button, governance findings with Adopt / Waive / Reopen buttons, and recent activity in one surface. Clicking Implement creates a worktree, spawns an agent with invariants + rationale + `.haft/workflow.md` + knowledge-graph invariants injected, verifies on completion, baselines affected files on success (CL3 evidence recorded), and generates a PR body from decision rationale. Adopt on drift / stale findings creates an agent thread with full context; agent never auto-resolves — presents options, user chooses. Resolutions recorded as RefreshReport. Implement guards: G1 blocks (multiple active decisions), G2/G4 warn (missing parity plan, subjective dimensions), no-invariants warns. Irreversible actions (Implement, Create PR, Reopen, Supersede, Deprecate) require confirmation dialogs showing affected artifacts. Governance scanner auto-refreshes on a timer.
- **`haft run` — full implementation pipeline from CLI** — reads a DecisionRecord, plans tasks via an agent, executes each with build verification, runs final invariant review, baselines on success. One command: `haft run dec-001`. Two modes: interactive (pauses between tasks) and `--auto` (full pipeline). `-c` for extra context files, `-p` for extra instructions. Task plan persisted as `.haft/plans/{ref}.md`, human-editable before execution. Per-task `go build` verification; on failure, a fix agent is spawned automatically. Final review runs invariants + build + tests; on failure, a fix agent is spawned and review re-runs.
- **Haft Design System (desktop Tauri frontend)** — lifted the design-system kit from the `haft-design-system` bundle into production: eight typed primitives (`Eyebrow`, `Button`, `Badge`, `Card`, `Input`, `StatCard`, `MonoId`, `Pill`) under `desktop/frontend/src/components/primitives/` consuming the existing Tailwind `@theme` tokens; shell components (`RailBtn`, `SidebarTask`, `StatusDot`) extracted from `App.tsx` into `components/shell/`; `ComparisonTable` component (border-first Pareto-front grid with accent highlighting and recommendation banner) replaces the legacy inline `<table>`. Dashboard, Decisions (with new `DecayWindow` progress bar computed from `created_at` + `valid_until`), Jobs, and Portfolios pages migrated to primitives.
- **`governance_mode` field on DecisionRecord** — declares whether `affected_files` are governed at the file level (`exact`) or widened to module-level scope (`module`, default — preserves pre-6.2.x behavior). Exact mode skips the silent directory inflation in baseline / drift detection. Honors FPF X-SCOPE: every claim has explicit where + under what + when.
- **FPF semiotic patterns** — 7 new patterns distilled from Levenchuk's semiotics slideument: FRAME-08 Reading Checklist (6 pre-reasoning questions), FRAME-09 Strict Distinction Quad (Role/Capability/Method/Work), CHR-10 Boundary Norm Square (L/A/D/E), CHR-11 Relational Precision Restoration Pipeline (A.6.P), CHR-12 Umbrella-word Family (quality / action / service / sameness / wholeness specializations), X-STATEMENT-TYPE (classify every load-bearing sentence), X-FANOUT-AUDIT (sweep all carriers on concept rename).
- **Compiled FPF pattern index** — 65 pattern chunks indexed alongside 4625 FPF spec chunks. Phase-keyed routes (frame / characterize / explore / compare / decide / verify / cross-cutting) in `fpf-routes.json`. 7 pattern files under `internal/fpf/patterns/`.
- **Auto-injected FPF hints in reasoning tool responses** — `haft_problem`, `haft_solution`, `haft_decision` responses include compact pattern ID citations for the current phase with retrieval guidance. Hints derive from embedded pattern files at runtime via `//go:embed` — renaming a pattern heading propagates automatically.
- **Core pattern markers** — `**Core:** true | <phase>` frontmatter in pattern markdown selectively surfaces top patterns per phase in auto-injected hints. Supports cross-phase citation (e.g. CHR-01 core in both frame and characterize).
- **FPF Micro-Patterns baseline in h-reason SKILL.md** — compressed always-in-context versions of core patterns for direct-response mode where no tool is called and hints don't inject.
- **`Valid-until` self-application on FPF pattern files** — each pattern file under `internal/fpf/patterns/` now declares a `**Valid-until:** YYYY-MM-DD` review date. New `TestPatternFilesNotPastValidUntil` fails when any pattern file is past its date, forcing periodic review of attribution, Core markers, and content currency. Initial dates set to 2026-10-18 (six-month review cadence).
- **`internal/embedding` package** — designated home for provider-bound embedding implementations. Hosts the OpenAI semantic embedder (`embedding.NewOpenAI`) extracted from `internal/fpf/semantic_embedder.go`. The fpf package now keeps only the abstract `SemanticEmbedder` interface and `SemanticEmbedderDescriptor` type — no provider, openai-go, or agent imports.
- **Transport-parity golden test** (`internal/cli/serve_parity_test.go`) — documents the action enum drift between standalone (`internal/tools/haft.go`) and MCP (`internal/cli/serve.go` switch dispatch) for each tool. Documented drift today (haft_problem.adopt, haft_decision.apply, haft_refresh.drift/reconcile, haft_query.board/list/coverage) is captured in the test's `knownTransportDrift` map; new drift fails the test. Detection layer for the unified-contract refactor planned for 6.3.
- **Layered architecture boundary tests** (`internal/artifact/core_boundary_test.go`) — replaced the desktop-only denylist with `TestPureCoreDoesNotDependOnSurfaceOrFlow` (asserts pure-Core packages — including `internal/fpf` — never import flow/surface/provider/external) and `TestEmbeddingPackageIsFlowLayerOnly` (asserts no Core package imports `internal/embedding`).
- **Cross-project recall regression tests** — verifies `haft_decision(action="decide")` returns the canonical artifact ID; two decisions with the same `selected_title` in one project now produce distinct global-index entries.
- **`haft init` smoke tests** (`internal/cli/init_smoke_test.go`) — assert MCP config production for Claude Code + Cursor stays correct: bare `haft` command (no absolute binary path), legacy `quint-code` key migrated out, idempotent re-runs, `HAFT_PROJECT_ROOT` env var plumbed through.

### Chore

- **Dead code sweep** — removed unused `loadSemanticRoutes` helper, unused `getBinaryPath` in `internal/cli/init.go`, unused `normalizeDecisionPredictions` in `internal/artifact/claim_status.go`, unused `testSteppingStoneVariant` test helper, and unused ANSI color constants `aBlue`/`aMagenta` in `internal/cli/run_ui.go`.
- **Modernize lint hints applied**: `strings.SplitSeq` in `patterns.go`, `min()` builtin replacing manual length cap, tagged switch over `Status` in `present/format.go`.
- **`task install` cleans stale GOBIN binary** — `rm -f $GOBIN/haft` runs first so PATH resolution picks up the freshly-built `BIN_DIR/haft` instead of an older `go install`-produced binary.
- **FPF spec submodule** bumped from 08e8e6f to 585938a.
- **Desktop Cargo.lock** — `haft-desktop` adds `dirs` dependency.

## [6.1.0] — 2026-04-14

### Added

- **`haft check` CLI command** — CI-friendly governance verification. Runs stale scan, drift scan, unassessed decisions, coverage gaps. Exit 0 = clean, exit 1 = findings. `--json` flag for structured output.
- **Full governance state in `/h-verify`** — scan now surfaces pending problems (backlog/in-progress count), addressed problems without linked decisions, and invariant violations from knowledge graph. Single entry point for "what needs attention."
- **`.haft/workflow.md` support** — hybrid markdown+YAML project policy file. Parsed at serve/agent startup. Intent + Defaults injected into agent prompts. `haft init` creates commented example.
- **Problem typing on ProblemCard** — `problem_type` field: optimization, diagnosis, search, synthesis. Accepted on frame, stored in DB, shown in `/h-status` and `/h-problems`.
- **Derived decision health model** — replaces single "phase" with two independent axes: Maturity (Unassessed / Pending / Shipped) and Freshness (Healthy / Stale / AT RISK). Freshness evaluated only for Shipped decisions. Never stored — computed at query time.
- **Claim-scoped evidence supersession** — new measurement supersedes only previous measurements for the same `(claim_ref, observable)`, not all measurements on the decision. Prevents unrelated evidence from being retired.
- **Claim-scoped R_eff** — `R_eff(decision) = min(R_eff(claim_i))` where each claim's R_eff is computed from its own evidence. More precise than decision-level aggregation.
- **F_eff / G_eff decomposition** — Formality (F0–F3) and Groundedness (CL-derived) exposed as view concerns alongside R_eff for evidence diagnosis.
- **Deep onboard for legacy projects** — `/h-onboard` now runs module coverage analysis and deep scans blind modules: reads code, identifies responsibilities, invariants, implicit decisions, risks. Supports parallel subagent execution when available.

### Changed

- **"No evidence = Unassessed"** — decisions without evidence are shown separately from healthy decisions, not treated as fresh. UI surfaces coverage gaps.
- **Verdict vocabulary normalized** — measurement result aliases (`accepted`/`partial`/`failed`) mapped to canonical evidence verdicts (`supports`/`weakens`/`refutes`) at storage boundary.
- **CL0 + supports = inadmissible** — evidence from opposed context with verdict `supports` is rejected at ingest, not merely penalized.
- **G1 enforced: one active decision per problem** — `Decide()` rejects if another active DecisionRecord exists for the same problem_ref.
- **G2: parity plan warnings** — `haft_solution(action="compare")` in standard/deep mode warns if parity plan is empty or unstructured.
- **G4: subjective dimension warnings** — compare warns on dimensions like "maintainable", "simple", "scalable" — asks to decompose into measurables or tag as observation-only.
- **Core boundary enforced** — integration tests verify Core packages (`internal/artifact`, `graph`, `fpf`, `reff`, `codebase`) have zero `desktop/` imports.

### Fixed

- **Desktop: oversized task output tails bounded** — prevents UI freeze on large agent outputs.
- **Knowledge graph integration tests** — FindDecisionsForFile, FindInvariantsForFile, ComputeImpactSet tested on seeded DB with real project data.

## [6.0.0] — 2026-04-13

### Breaking Changes

- **Product renamed from quint-code to Haft** — binary, MCP tools (`quint_*` → `haft_*`), slash commands (`/q-*` → `/h-*`), skill names, and docs all use `haft` naming. Existing MCP configs, skill references, and slash commands from v5.x will not work without updating.
- **Decision data model replaced** — claim-aware decision kernel with structured claims, predictions, and claim-bound evidence replaces markdown-only reconstruction. Existing decision artifacts require migration.
- **Reasoning model changed** — 5-mode activity model (Understand / Explore / Choose / Execute / Verify) replaces the artifact-centric 6-step protocol. Skill instructions, prompts, and agent behavior follow the new model.
- **`/h-verify` replaces `/h-refresh`** — `/h-refresh` is deprecated and auto-cleaned on install. Use `/h-verify` for discovery (scan + drift + pending verify_after) and triage.

Note: older changelog entries keep historical `quint-code`, `quint_*`, and `/q-*` names where they describe behavior, commands, or releases from that era.

### Added

- **Desktop app (pre-alpha)** — Wails v2 desktop application with dashboard, problem board, decision detail with evidence F/G/R decomposition, portfolio comparison with Pareto front visualization, task spawning (Claude Code / Codex), agent chat view, terminal panel (Cmd+\`), multi-project management, and search (Cmd+K). Dark theme following the design system. Pre-alpha: not recommended for production use.
- **Standalone Haft runtime** — local-first `haft agent` / TUI flow with persisted sessions, checkpointed vs autonomous execution, permission and question dialogs, model/session pickers, compaction, spawned subagents, and a typed JSON-RPC protocol between UI and runtime.
- **Knowledge graph** — `internal/graph` package providing unified query interface over existing artifact, module, and dependency tables. Queries: FindDecisionsForFile, FindInvariantsForFile, FindModuleForFile, TransitiveDependents, ComputeImpactSet. All cycle-safe with depth limiting. 17 tests.
- **Invariant injection into agent prompts** — when implementing a decision, agents receive invariants from ALL decisions governing the affected files, not just the current decision's own invariants. Invariants tagged with source decision ID.
- **Invariant verification** — automated checking of "no dependency from X to Y" and "no circular dependencies" patterns against the live module dependency graph. Returns holds/violated/unknown per invariant.
- **Governance invariant alerts** — governance scanner now runs invariant verification on decisions with drift findings, creating problem candidates for violations.
- **Probe-or-commit readiness gate** — AssessComparisonReadiness evaluates portfolio comparison quality: variant count, dimension coverage, score fill rate, constraint presence, parity plan. Returns commit/probe/widen/reroute with specific recommendations. Shown in desktop Portfolios page.
- **Evidence F/G/R decomposition** — decision detail page shows per-evidence formality level (F0-F3), congruence level (CL0-CL3), verdict badges, freshness indicators, and coverage gaps (claims without evidence).
- **Auto-run toggle for agent tasks** — per-task toggle between checkpointed (agent pauses) and auto-run (agent proceeds without intervention) modes. Persisted across app restart.
- **`haft sync` for team workflow** — syncs `.haft/` markdown files into local SQLite database after `git pull`. Enables team collaboration where `.haft/*.md` in git is the shared source of truth and each engineer has their own local database.
- **Probe-or-commit behavioral gate** — Choose mode now includes a readiness checklist before comparison: dimension coverage, variant diversity, and whether a specific next investigation could change the ranking. Returns commit / probe / widen / reroute.
- **Language precision triggers** — Understand and Choose modes catch ambiguous terms (service, process, quality, component) and subjective comparison dimensions (maintainable, simple, scalable) before they corrupt downstream reasoning.
- **`verify_after` field on claims** — `DecisionClaim` and `PredictionInput` now accept `verify_after` (RFC3339 or YYYY-MM-DD). Claims with past verify_after dates that remain unverified are surfaced by `haft_refresh(scan)` as `pending_verification` stale items with observable and threshold details. MCP schema updated.
- **Constraint-aware Pareto computation** — `computeParetoFront()` now eliminates variants that are strictly worst on all comparable peers for any constraint dimension before dominance computation. Constraint violations are reported as warnings. Variants with missing constraint data are preserved conservatively.
- **Standalone agent refresh tool parity** — `HaftRefreshTool` now exposes all 6 actions (scan, drift, waive, reopen, supersede, deprecate) matching the MCP server schema. Previously only scan/drift were available to the standalone agent.
- **Explicit reroute map** — legitimate upstream transitions documented: Choose → Understand (comparison reveals bad framing), Explore → Understand (wrong problem type), Execute → Choose, Verify → any earlier mode.
- **Claim-aware decision kernel** — decisions now persist canonical structured claims, predictions, claim-bound evidence, live measurement status, and deterministic Pareto/coverage state instead of relying on markdown-only reconstruction.
- **Deterministic projections** — projection views now render the same artifact graph for different audiences, including engineer, manager, audit, compare, delegated-agent brief, and change-rationale handoff surfaces.
- **Route-aware FPF retrieval** — indexed section summaries, route expansion, explain/full controls, golden-query evaluation, tree drill-down, and experimental semantic retrieval over the embedded FPF corpus.
- **Broader codebase awareness** — C/C++ module and include detection, symbol hashing, richer module/dependency scanning, and module-governance reporting in status/coverage flows.
- **Expanded client integrations** — `haft init` now installs MCP/command surfaces for Claude Code, Cursor, Gemini CLI, Codex CLI/App, and Air while keeping the same local binary/runtime.
- **`haft_problem(action="close")`** — marks a ProblemCard as `addressed`. Previously required manual frontmatter editing. Exposed in MCP schema for both plugin and standalone modes. ([#43](https://github.com/m0n0x41d/quint-code/issues/43))
- **Auto-baseline after `decide`** — when `affected_files` are provided, file hashes are snapshotted immediately after the decision is recorded. No more manual `haft_decision(action="baseline")` calls. ([#43](https://github.com/m0n0x41d/quint-code/issues/43))

### Changed

- **Core architecture refactored into explicit layers** — artifact build/store logic, presentation formatting, protocol transport, agent runtime, and TUI shell now live as clearer functional boundaries with purer `Build*`/formatting paths and thinner orchestration shells.
- **Agent execution moved beyond slash-command steering** — the repo now supports both MCP/plugin workflows and a standalone agent/TUI loop, with persisted execution mode aliases and compatibility bridges for older symbiotic/collaborative terminology.
- **Provider/model support expanded** — the registry and CLI now support multi-provider model discovery/switching with GPT-5.4-class defaults/fallbacks instead of the older 5.3-era baseline.
- **FPF search quality improved materially** — deterministic route lookup, better weighting/sanitization, explicit section summaries, and MCP-accessible spec search replaced the older narrower retrieval path.
- **`haft init --codex` TOML generation fixed** — idempotent section replacement instead of append, prevents duplicate key errors on repeated init.

### Fixed

- **`haft serve` / plugin mode now matches the core claim model** — served MCP schema and handlers understand predictions, strict decision/measurement arrays, claim refs/scope, and projection views instead of lagging behind the direct runtime.
- **Slash-command guidance no longer points users at stale `/q-*` actions** — note validation, nav strips, MCP presentation text, and h-reason docs now consistently steer users through the `h-*` surface, with `/h-view` as the advanced projection entry point.
- **Large pasted prompts no longer explode the TUI** — oversized pasted text is collapsed to `[N rows inserted]` placeholders in the input/queue/transcript UI, while the raw content is preserved and expanded only at submit time.
- **Queued follow-up messages preserve real prompt state** — multiline text, attachments, and hidden collapsed-paste payloads now survive queueing, replay, and draft restore paths without truncation or accidental `trim()` damage.
- **Decision/evidence integrity issues tightened** — malformed compare/measure payloads now fail loudly, Pareto fronts are computed deterministically, and claim/evidence bindings keep canonical scope instead of silently degrading.
- **Governance shutdown no longer panics on double-close** — `sync.Once` prevents channel double-close during fast project switching.
- **SwitchProject validates new project before teardown** — pre-checks DB accessibility, preventing zombie state if the target project is broken.
- **Task auto_run field restored from database** — was persisted but silently lost on restart.
- **WAL mode + busy_timeout on all SQLite connections** — prevents SQLITE_BUSY during concurrent governance scanner and UI queries.
- **Null safety across all Go→JSON view projections** — nil slices now serialize as `[]` not `null`, preventing frontend TypeError crashes on 30+ array fields.
- **Task runner race conditions fixed** — state copied under mutex before use outside lock in shutdown, cancel, and finalize paths.
- **Atomic file writes for config and registry** — temp file + rename prevents corruption from concurrent access.
- **Task timeout enforcement** — agent processes killed after configurable timeout (default 300 min), preventing zombie processes.
- **Artifact Create uses single transaction** — artifact insert and link inserts wrapped in one transaction, preventing partial state on link failure.
- **tableHasColumn PRAGMA cached** — eliminated 2 PRAGMA queries per evidence operation.
- **Large agent output truncated** — outputs over 500 lines show last 200 with "Show full output" button, preventing WebView freezing.
- **Search race condition fixed** — stale results from earlier queries no longer briefly flash.

## [5.3.1] — 2026-03-25

### Fixed

- **NavStrip no longer triggers agent auto-execution** — "Next:" label replaced with "Available:" + explicit guard line ("do not auto-execute"). Slash commands (`/q-explore`, `/q-decide`) replace tool call syntax (`quint_solution(action="explore", ...)`), so agents read them as user actions, not callable functions.
- **NavStrip is mode-aware** — available actions now reflect the current depth mode. Tactical shows `/q-explore | /q-decide` (short cycle). Standard without characterization shows `/q-char | /q-explore` — making `/q-char` visible as the gateway to the full cycle. Standard with characterization shows only `/q-explore`. EXPLORING in tactical shows `/q-decide | /q-compare (upgrade)` instead of always suggesting compare.
- **`quint_solution(action="compare")` rejected valid dimensions** — compare handler used raw type assertions instead of `parseStringArrayFromArgs` helper. When MCP clients serialized `dimensions` or `non_dominated_set` as JSON strings (common without schema loaded), the assertion silently failed, producing a misleading "at least one comparison dimension is required" error. Same fix applied to `scores` (new `parseNestedStringMapFromArgs` helper) and measure handler arrays (`criteria_met`, `criteria_not_met`, `measurements`).
- **"No baseline" scan confused with "not implemented"** — `CheckDrift` now checks git history for affected files when no baseline exists. Distinguishes "files changed since decision (likely implemented, needs baseline+measure)" from "files unchanged (not yet implemented)". Prevents agents from misreporting implemented decisions as not started.

### Added

- **NavStrip interpretation in q-reason skill** — new section documenting that "Available:" is a menu for the user, not instructions for the agent. Clarifies that tactical mode has fewer steps but the same human consent gates, and only Path 3 (explicit delegation) overrides the guard.
- **Proactive check for "no baseline" in q-reason skill** — instructs agents to not assume "no baseline" means "not implemented" and to check git history before reporting status.

## [5.3.0] — 2026-03-24

### Added

- **Interactive terminal dashboard (`quint-code board`)** — Bubbletea v2 TUI with four tabs: Overview (health, activity, depth distribution, coverage, contexts, evidence), Problems (backlog with drill-in), Decisions (list with R_eff/drift, drill-in with glamour markdown), Modules (coverage tree). Live refresh every 3s. Connected tab borders, alternating row colors, adaptive dark/light theme, dynamic help bar. Exit code 1 with `--check` flag for CI/hooks.
- **Decision mode computed from artifact chain** — `inferModeFromChain` derives mode from linked problems (characterization) and portfolios (comparison). Agent-declared mode can only escalate, not downgrade. Fixes misclassification where full-cycle decisions were recorded as tactical.
- **FTS5 search keyword enrichment** — `search_keywords` column on artifacts, indexed by FTS5. Agent generates synonyms and related terms at write time. Accepted on `quint_note` and `quint_decision`. Migration 15 rebuilds FTS5 index.
- **C/C++ header-only module detection** — `-I` include directories from `compile_commands.json` are registered as modules (FileCount=0), so dependency edges to `include/` directories are no longer dropped by `ScanDependencies`.

### Fixed

- **`/q-refresh scan` now rescans modules** — module structure updates alongside drift and stale checks, keeping dependency graph fresh without requiring a separate `coverage` action.
- **C/C++ symlink-safe include resolution** — `resolveInclude` canonicalizes both `projectRoot` and `-I` paths with `EvalSymlinks` before computing relative paths. Fixes silent edge loss on macOS symlinked checkouts.
- **Notes excluded from drift detection** — notes are observations, not implementations. ScanStale no longer flags notes with affected_files as "no baseline."
- **Module scanner excludes `.claude` and `.context` directories** — Claude Code worktrees and reference repos no longer inflate module count.
- **q-reason skill context-aware entry** — skill no longer always falls through into full FPF cycle. Three paths: think-and-respond (no artifacts), prepare-and-wait (human drives), full autonomous cycle (agent drives). Default is prepare-and-wait.

## [5.2.0] — 2026-03-23

### Added

- **C/C++ module detection** — `compile_commands.json` as primary source (searches project root, `build/`, `cmake-build-*/`). Falls back to directory-based heuristic with `Makefile`/`CMakeLists.txt`/`meson.build` markers. Graceful fallback if `compile_commands.json` paths don't resolve.
- **C/C++ import parsing** — extracts `#include "..."` edges (skips `<...>` system includes). Resolves include paths using `-I` flags from `compile_commands.json`. Falls back to relative and project-root resolution.
- **C/C++ extensions** — `.c`, `.h`, `.cpp`, `.cc`, `.cxx`, `.hpp`, `.hxx` registered in language registry.

### Fixed

- **`quint_solution(action="explore")` rejected valid variants** — MCP clients that serialize the `variants` array as a JSON string (instead of a parsed array) caused silent parsing failure, resulting in a misleading "genuinely distinct options" error with 0 variants. Same fix applied to all array fields across note/problem/decision handlers. ([#33](https://github.com/m0n0x41d/quint-code/issues/33))
- **Status always rescans modules** — `quint_query(action="status")` now runs a fresh module scan instead of showing stale cached data. Previously required `action="coverage"` to trigger rescan.
- **Symlink-safe path resolution** — C/C++ module detection uses `filepath.EvalSymlinks` on project root and source paths for reliable matching on macOS and symlinked project directories.

## [5.1.0] — 2026-03-20

### Added — Computed Features

- **R_eff computation** — effective reliability = min(evidence_scores) with CL penalties (CL3=0, CL2=0.1, CL1=0.4, CL0=0.9). Expired evidence scores 0.1. Computed on every access.
- **Evidence decay → stale detection** — decisions with R_eff < 0.5 auto-surface in stale scan. R_eff < 0.3 = "AT RISK" label.
- **Graduated epistemic debt** — stale items sorted by severity (days overdue), debt magnitude displayed.
- **Diversity check** — Jaccard similarity on variant titles+descriptions. Warns at >50% word overlap.
- **Archive recall** — FTS5 search at frame/explore time surfaces related active artifacts as "Related History".
- **Characterization cross-check** — compare warns on dimension mismatch, asymmetric scoring, parity rules.
- **Parity checklist** — auto-generated per-dimension parity questions from characterization.
- **Goldilocks signals** — readiness score (section completeness) + complexity counts (constraints, targets, dimensions) in problem selection.
- **Problem lifecycle** — three-way split: Backlog (no work) → In Progress (has portfolio) → Addressed (has decision).
- **Proactive evidence prompts** — after frame/explore in standard+ mode, tool prompts agent to collect and attach evidence.
- **Periodic refresh prompt** — if >5 days since last scan, any tool response reminds agent to run refresh.
- **Lemniscate feedback** — failed/partial measurement suggests reopen with concrete command.

### Added — Codebase Awareness

- **File drift detection (Level A)** — `baseline` action snapshots SHA-256 hashes of affected files. `/q-refresh` detects drift (modified, file missing, no baseline). Self-correcting: unbaselined decisions surface in `/q-status`.
- **Module detection (Level B)** — detects modules/packages across Go (`go/parser`), JS/TS (`package.json` + `index.ts` barrel files), Python (`__init__.py`), Rust (`Cargo.toml` + `mod.rs`). Interface-based architecture — one implementation per language behind `ModuleDetector`/`ImportParser` interfaces.
- **Dependency graph (Level C)** — parses imports to build module dependency graph. Go uses `go/parser` stdlib (100% accuracy), JS/TS/Python/Rust use regex. Impact propagation: when module A drifts, drift report shows dependent modules and their decisions.
- **Decision coverage report** — `quint_query(action="coverage")` shows governed/partial/blind modules. R_eff-aware: only `DecisionRecord` artifacts count as governance, `partial` status for modules where all decisions have degraded evidence (R_eff < 0.5).
- **`.quintignore` support** — project-specific exclusions for module scanning. Also respects `.gitignore` (local + global) via `go-gitignore` library.
- **Module coverage in `/q-status`** — coverage section appended to status dashboard when modules are scanned.
- **Module-aware onboarding** — `/q-onboard` now includes module coverage analysis step, prioritizes blind modules.

### Added — Unified Storage & Cross-Project Recall

- **Unified storage** — database moved from `.quint/quint.db` (in-repo) to `~/.quint-code/projects/{id}/quint.db` (home dir). Markdown projections remain in `.quint/` for code review. No binary files in git.
- **Project identity** — `.quint/project.yaml` with immutable generated ID (`qnt_` + 8 hex). Created on `init`, committed to git, same for all developers.
- **Cross-project decision recall** — `~/.quint-code/index.db` stores decision summaries across all projects. When framing a new problem (`/q-frame`), related decisions from other projects surface with CL2/CL1 tags.
- **CL matching** — same primary language = CL2 (similar context), different language = CL1 (different context). Auto-detected from codebase modules.
- **Serve guard** — if old `.quint/quint.db` exists but project not migrated, MCP blocks all tool calls with migration instructions.
- **`QUINT_SERVER_ORIGIN`** — new env var in MCP config. `local` (default) for solo dev. URL value accepted for future remote server mode (not implemented yet).
- **Context facts** — `context_facts` table auto-populated on startup with project fingerprint (languages, module count, decision count, domains).

### Added — Decision Quality & Integrity

- **Adversarial verification gate** — `/q-decide` runs a verification check before recording. Tactical: one-line counter-argument. Standard/deep: 5 probes (deductive consequences, strongest counter-argument, self-evidence check, tail failure scenarios, WLNK challenge). Grounded in FPF A.12 + Verbalized Sampling research.
- **Evidence supersession** — when `Measure()` records a new measurement, previous measurements on the same artifact are marked `verdict='superseded'` and excluded from R_eff. Prevents old partial measurements from dragging R_eff down permanently.
- **Inductive measurement gate** — `Measure()` warns if no baseline exists for the decision's affected files. Measurements without baseline record at CL1 (self-evidence), not CL3. R_eff honestly reflects verification quality.
- **R_eff shared package** — `internal/reff/` extracts `ScoreEvidence`, `VerdictToScore`, `CLPenalty` as shared pure functions. Single source of truth for both `artifact` and `codebase` packages.
- **Note-decision dedup** — containment-based overlap check at write time. If >70% of a note's title words appear in an active decision title, the note is rejected. 50-70% = warning. Also checks note-vs-note duplicates.
- **Reconcile action** — `quint_refresh(action="reconcile")` batch-scans all active notes against all active decisions for overlaps. One Go-side pass, no per-note agent calls.
- **Shipped vs Pending** — `/q-status` splits decisions into "Shipped" (has measurement) and "Pending Implementation" (no measurement).
- **Post-implementation ritual** — SKILL.md teaches agent to baseline + verify + measure after implementing a decision.

### Added — Developer Experience

- **Structured logging** — middleware in `serve.go` auto-logs every MCP tool call entry/exit with tool name, action, duration_ms, status. Domain logging for artifact create/baseline/drift and codebase scan operations.
- **Codex skill support** — `quint-code init --codex` installs `/q-reason` skill to `~/.agents/skills/q-reason/SKILL.md`.
- **Pre-commit hook** — `.githooks/pre-commit` mirrors CI pipeline exactly: `go mod tidy`, `golangci-lint`, `go test -race`, `go build`.

### Added — Product Features

- **FPF E.9 Decision Records** — four-component structure: Problem Frame, Decision/Contract, Rationale, Consequences. Decide response shows full DRR inline.
- **Indicator roles** — characterization dimensions tagged as constraint (hard limit), target (optimize), or observation (Anti-Goodhart).
- **Per-dimension measurement freshness** — valid_until on individual comparison dimensions. Compare warns on expired measurements.
- **Note auto-lifecycle** — notes auto-expire at 90 days. Detectable by scan. Waive/deprecate/supersede supported.
- **Generalized refresh** — waive/supersede/deprecate work on ALL artifact types (notes, problems, decisions, portfolios), not just decisions.
- **Multi-problem decisions** — `problem_refs` array parameter: one decision can address multiple problems.
- **Audit trail** — every tool call logged to audit_log table (fire-and-forget).
- **SoTA survey prompt** — explore in standard/deep mode reminds to check existing solutions before deciding.
- **Status caps** — dashboard sections capped (decisions=5, stale=5, problems=5, addressed=3) with overflow indicator.
- **List action** — `quint_query(action="list", kind="DecisionRecord")` for full artifact listing without caps.
- **Evidence display in problems** — /q-problems shows evidence count and verdict summary per problem.

### Fixed

- **CL=0 silent upgrade** — CL=0 (opposed context) no longer defaulted to CL=3. Uses -1 sentinel for "not provided".
- **NextSequence race condition** — uses MAX(id) instead of COUNT to avoid TOCTOU duplicate IDs.
- **Swallowed errors** — store.Update and store.AddLink errors in refresh operations now logged via logger.Warn.
- **FTS5 special characters** — comprehensive stripping of +, -, :, ~, single quote alongside existing chars.
- **MCP server stability** — panic recovery in request handler, 1MB stdin buffer (was 64KB), lifecycle logging (start/stop/EOF), stdout write error handling.
- **MCP init config** — uses QUINT_PROJECT_ROOT env instead of cwd. Removed stale nested .mcp.json.
- **Codex/Air project config** — `init --codex` / `init --air` now write MCP settings to project-local `.codex/config.toml` instead of shared `~/.codex/config.toml`.
- **writeFileQuiet** — uses logger.Warn instead of fmt.Fprintf(stderr).
- **MCP JSON string arrays** — `parseStringArray` now handles arrays sent as JSON strings by MCP clients (e.g., `"[\"a\"]"` instead of `["a"]`).
- **Coverage governance honesty** — only `DecisionRecord` artifacts count as governance. Notes no longer inflate coverage percentage.
- **Root module coverage** — root modules (Path: "") now match all files in the project, not just root-level files. Fixes JS/TS and Rust single-package coverage.
- **Measurement CL scoring** — measurements without baseline record at CL1 (0.4 penalty), not CL3. Prevents unverified measurements from inflating R_eff.
- **Coverage R_eff consistency** — unknown verdict in coverage computation now scores 0.5 (weakening), matching artifact package. Was incorrectly 0.0.
- **Status notes filter** — `/q-status` recent notes section filters out deprecated/superseded notes.
- **Evidence supersession in R_eff** — `ComputeWLNKSummary` excludes superseded evidence items from R_eff calculation.
- **FTS5 sanitization** — cross-project recall query sanitizer now strips periods, commas, semicolons, dashes, and other punctuation that caused FTS5 syntax errors.

### Changed

- **Apply deprecated** — decide response includes full DRR body. Apply action returns body directly (backward compat). `/q-apply` slash command removed.
- **Refresh UX** — tool description, schema, and slash command updated: "manage artifact lifecycle" not "detect stale decisions". `artifact_ref` parameter added (alongside `decision_ref` for compat).
- **Nav strip** — shows tactical decide option after frame. No apply prescription after decide.
- **Storage location** — database moved from `.quint/quint.db` to `~/.quint-code/projects/{id}/quint.db`. Requires re-running `quint-code init` to migrate.
- **Coverage always rescans** — `quint_query(action="coverage")` always runs fresh module scan instead of caching for 7 days.

### Removed

- `/q-apply` slash command
- Apply prescription from nav strip and decide response

## [5.0.0] — 2026-03-16

### Breaking Changes

Complete product redesign. v5 is not backward-compatible with v4.

- All v4 MCP tools removed
- All v4 slash commands removed
- Hypothesis/phase-based model replaced with problem/solution/decision model
- Phase FSM, role system, L0/L1/L2 user-facing layers removed
- .quint/ directory structure changed

### Added

- 6 MCP tools: quint_note, quint_problem, quint_solution, quint_decision, quint_refresh, quint_query
- 11 slash commands
- /q-reason skill with diagnostic framing protocol
- Artifact system with dual-write storage (DB primary, files secondary)
- Navigation strip in every tool response
- Note validation (rationale, conflicts, scope)
- Decision modes (note, tactical, standard, deep)
- Impact measurement and evidence attachment
- Versioned characterization
- All-artifact stale scan with lineage on reopen
- FPF spec search (4243 sections embedded)
- goreleaser for cross-platform builds

### Architecture Decisions

ADR-1 through ADR-19 documented in `.context/v5-architecture-decisions.md`
